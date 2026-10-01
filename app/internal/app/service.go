// Package app wires UNCLI together: store, profiles, the Claude adapter and
// installer, the local runtime and the session manager. It knows nothing
// about Wails; the bridge exposes it to the UI.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"uncli/config"
	"uncli/internal/adapter/claude"
	"uncli/internal/core"
	"uncli/internal/profile"
	"uncli/internal/runtime/local"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Settings keys.
const (
	SettingCLIVersion = "cli.claude.version" // empty = the pinned version
	SettingCLIPath    = "cli.claude.path"    // a binary to use instead of a managed one (development)
)

type Paths struct {
	Config  string // db and user YAML
	Cache   string // downloaded CLIs
	Scratch string // chat session folders
}

// DefaultPaths uses the OS's user config and cache directories.
// UNCLI_DATA_DIR moves the database, user YAML and scratch folders
// elsewhere (development, tests); downloaded CLIs stay in the shared cache.
func DefaultPaths() (Paths, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, err
	}
	base := filepath.Join(cfg, "uncli")
	if d := os.Getenv("UNCLI_DATA_DIR"); d != "" {
		base = d
	}
	return Paths{Config: base, Cache: filepath.Join(cache, "uncli"), Scratch: filepath.Join(base, "scratch")}, nil
}

type Service struct {
	Paths     Paths
	Store     *store.Store
	Profiles  *profile.Set
	Adapter   *claude.Adapter
	Installer *claude.Installer
	Sessions  *session.Manager

	emit Emitter

	authMu   sync.Mutex
	auth     *core.AuthInfo
	authTime time.Time
}

func New(paths Paths, emit Emitter) (*Service, error) {
	for _, d := range []string{paths.Config, paths.Cache, paths.Scratch} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := store.Open(filepath.Join(paths.Config, "uncli.db"))
	if err != nil {
		return nil, err
	}
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	set, err := profile.Load(defaults, paths.Config)
	if err != nil {
		db.Close()
		return nil, err
	}
	inst := claude.NewInstaller(filepath.Join(paths.Cache, "cli", "claude"))
	s := &Service{Paths: paths, Store: db, Profiles: set, Installer: inst, Adapter: claude.New(inst), emit: emit}
	s.Sessions, err = session.NewManager(session.Deps{
		Store: db, Adapter: s.Adapter, Runtime: local.New(), Profiles: set,
		Binary: s.binary, Sink: newCoalescer(emit, 50*time.Millisecond), ScratchDir: paths.Scratch,
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Service) Close() {
	s.Sessions.Close()
	s.Store.Close()
}

// cliVersion is the version the user runs: the pinned one unless they
// chose another at their own risk.
func (s *Service) cliVersion() string {
	if v, _ := s.Store.Setting(SettingCLIVersion); v != "" {
		return v
	}
	return s.Installer.Pinned()
}

func (s *Service) binary(ctx context.Context) (string, string, error) {
	if p := s.overridePath(); p != "" {
		return p, "custom", nil
	}
	v := s.cliVersion()
	if p, ok := s.Installer.Path(v); ok {
		return p, v, nil
	}
	return "", "", session.ErrNoCLI
}

func (s *Service) overridePath() string {
	if p := os.Getenv("UNCLI_CLAUDE_BIN"); p != "" {
		return p
	}
	p, _ := s.Store.Setting(SettingCLIPath)
	return p
}

type CLIStatus struct {
	Installed    bool   `json:"installed"`
	Version      string `json:"version"`
	Pinned       string `json:"pinned"`
	Custom       bool   `json:"custom"` // a binary path override is in use
	LoggedIn     bool   `json:"loggedIn"`
	Email        string `json:"email,omitempty"`
	Subscription string `json:"subscription,omitempty"`
	Error        string `json:"error,omitempty"`
}

// CLIStatus reports whether the CLI is installed and signed in. The auth
// check runs the CLI, so its result is cached briefly.
func (s *Service) CLIStatus(ctx context.Context, fresh bool) CLIStatus {
	st := CLIStatus{Version: s.cliVersion(), Pinned: s.Installer.Pinned(), Custom: s.overridePath() != ""}
	bin, _, err := s.binary(ctx)
	if err != nil {
		return st
	}
	st.Installed = true
	if st.Custom {
		st.Version = "custom"
	}
	s.authMu.Lock()
	cached := s.auth
	if fresh || cached == nil || time.Since(s.authTime) > 30*time.Second {
		cached = nil
	}
	s.authMu.Unlock()
	if cached == nil {
		info, err := s.authStatus(ctx, bin)
		if err != nil {
			st.Error = err.Error()
			return st
		}
		s.authMu.Lock()
		s.auth, s.authTime = &info, time.Now()
		s.authMu.Unlock()
		cached = &info
	}
	st.LoggedIn, st.Email, st.Subscription = cached.LoggedIn, cached.Email, cached.Subscription
	return st
}

func (s *Service) authStatus(ctx context.Context, bin string) (core.AuthInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := local.Run(ctx, s.Adapter.AuthStatusCommand(bin))
	// "auth status" exits 1 when signed out but still prints its JSON.
	if info, perr := s.Adapter.ParseAuthStatus(out); perr == nil {
		return info, nil
	}
	if err != nil {
		return core.AuthInfo{}, fmt.Errorf("could not check sign-in: %w", err)
	}
	return core.AuthInfo{}, errors.New("could not read the CLI's sign-in status")
}

// InstallCLI downloads the selected CLI version, reporting progress.
func (s *Service) InstallCLI(ctx context.Context) (CLIStatus, error) {
	_, err := s.Installer.Ensure(ctx, s.cliVersion(), func(done, total int64) {
		s.emit.Emit(EvtCLIProgress, Progress{Done: done, Total: total})
	})
	if err != nil {
		return s.CLIStatus(ctx, true), err
	}
	st := s.CLIStatus(ctx, true)
	s.emit.Emit(EvtCLIStatus, st)
	return st, nil
}

// SignIn starts the CLI's browser sign-in and reports the new status
// when it completes.
func (s *Service) SignIn(ctx context.Context) error {
	bin, _, err := s.binary(ctx)
	if err != nil {
		return err
	}
	wait, err := local.StartDetached(s.Adapter.LoginCommand(bin))
	if err != nil {
		return err
	}
	go func() {
		_ = wait()
		s.emit.Emit(EvtCLIStatus, s.CLIStatus(context.Background(), true))
	}()
	return nil
}

// SetCLIVersion selects a CLI version; empty returns to the pinned one.
func (s *Service) SetCLIVersion(ctx context.Context, version string) (CLIStatus, error) {
	version = strings.TrimSpace(version)
	if version == s.Installer.Pinned() {
		version = ""
	}
	if err := s.Store.SetSetting(SettingCLIVersion, version); err != nil {
		return CLIStatus{}, err
	}
	return s.InstallCLI(ctx)
}

func (s *Service) CLIChannels(ctx context.Context) (map[string]string, error) {
	return s.Installer.Channels(ctx)
}

type Bootstrap struct {
	Profiles     []profile.Profile     `json:"profiles"`
	Modifiers    []profile.Modifier    `json:"modifiers"`
	Toolbar      []profile.ToolbarItem `json:"toolbar"`
	Sessions     []session.View        `json:"sessions"`
	Models       []core.ModelInfo      `json:"models"`
	Capabilities core.Capabilities     `json:"capabilities"`
	Platform     string                `json:"platform"`
}

func (s *Service) Bootstrap() Bootstrap {
	return Bootstrap{
		Profiles: s.Profiles.Profiles, Modifiers: s.Profiles.Modifiers, Toolbar: s.Profiles.Toolbar,
		Sessions: s.Sessions.List(), Models: s.Sessions.Models(), Capabilities: s.Adapter.Capabilities(),
		Platform: runtime.GOOS,
	}
}

// OpenFolder shows a folder in the OS file manager.
func OpenFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
