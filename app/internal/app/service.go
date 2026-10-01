// Package app wires UNCLI together: store, profiles, the Claude adapter and
// installer, the local runtime and the session manager. It knows nothing
// about Wails; the bridge exposes it to the UI.
package app

import (
	"context"
	"encoding/json"
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

	loginMu sync.Mutex
	login   *local.Interactive
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
	s.CancelSignIn()
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

// authStatus asks the CLI whether it is signed in. "auth status" exits 1
// when signed out but still prints its JSON, so the JSON decides. The first
// run of a freshly downloaded binary can be slow (antivirus scans), so it
// gets a generous timeout and one retry.
func (s *Service) authStatus(ctx context.Context, bin string) (core.AuthInfo, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		actx, cancel := context.WithTimeout(ctx, 60*time.Second)
		out, err := local.Run(actx, s.Adapter.AuthStatusCommand(bin))
		cancel()
		if info, perr := s.Adapter.ParseAuthStatus(out); perr == nil {
			return info, nil
		}
		switch {
		case err != nil:
			lastErr = err
		case len(strings.TrimSpace(string(out))) == 0:
			lastErr = errors.New("it printed nothing")
		default:
			lastErr = fmt.Errorf("unexpected output %q", truncate(string(out), 120))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return core.AuthInfo{}, fmt.Errorf("couldn't check whether Claude is signed in (%v)", lastErr)
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
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

// SignInStart is the result of starting sign-in: either a link for the
// user to open, or the news that the CLI signed in by itself.
type SignInStart struct {
	URL      string    `json:"url,omitempty"`
	SignedIn bool      `json:"signedIn"`
	Status   CLIStatus `json:"status"`
}

// SignIn starts the CLI's sign-in. Without a terminal the CLI uses its
// paste-the-code flow: it prints a link and waits for the code the page
// shows, which SubmitLoginCode hands back. If the machine is already signed
// in elsewhere the CLI may finish on its own; "auth status" decides.
// Every attempt is logged to <config>/logs/signin.log.
func (s *Service) SignIn(ctx context.Context) (SignInStart, error) {
	bin, _, err := s.binary(ctx)
	if err != nil {
		return SignInStart{}, err
	}
	s.CancelSignIn()
	cmd := s.Adapter.LoginCommand(bin)
	p, err := local.StartInteractive(cmd)
	if err != nil {
		s.logSignIn("start failed: %v", err)
		return SignInStart{}, err
	}
	s.loginMu.Lock()
	s.login = p
	s.loginMu.Unlock()
	s.logSignIn("started %s %s", bin, strings.Join(cmd.Args, " "))

	deadline := time.After(60 * time.Second)
	for {
		if u, ok := s.Adapter.LoginURL(p.Output()); ok {
			s.logSignIn("link shown")
			go s.watchSignIn(p)
			return SignInStart{URL: u}, nil
		}
		select {
		case <-p.Done():
			s.logSignIn("exited before a link: %v\noutput: %q", p.Err(), p.Output())
			st := s.CLIStatus(ctx, true)
			s.emit.Emit(EvtCLIStatus, st)
			if st.LoggedIn {
				return SignInStart{SignedIn: true, Status: st}, nil
			}
			return SignInStart{}, fmt.Errorf("the Claude CLI's sign-in stopped without showing a link (%s). Details are in %s",
				describeExit(p), s.signInLogPath())
		case <-deadline:
			s.logSignIn("no link after 60 s; output: %q", p.Output())
			s.CancelSignIn()
			return SignInStart{}, fmt.Errorf("sign-in didn't start within a minute. Details are in %s", s.signInLogPath())
		case <-ctx.Done():
			s.CancelSignIn()
			return SignInStart{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// watchSignIn notices when the CLI finishes sign-in on its own (before any
// code is pasted) and tells the UI.
func (s *Service) watchSignIn(p *local.Interactive) {
	<-p.Done()
	s.loginMu.Lock()
	mine := s.login == p
	s.loginMu.Unlock()
	if !mine { // SubmitLoginCode or CancelSignIn took it over
		return
	}
	s.logSignIn("exited on its own: %v\noutput: %q", p.Err(), p.Output())
	st := s.CLIStatus(context.Background(), true)
	s.emit.Emit(EvtCLIStatus, st)
}

func describeExit(p *local.Interactive) string {
	out := strings.TrimSpace(string(p.Output()))
	msg := "it exited"
	if err := p.Err(); err != nil {
		msg = err.Error()
	}
	if out == "" {
		return msg + " and printed nothing"
	}
	return msg + ": " + truncate(lastLine(out), 160)
}

func (s *Service) signInLogPath() string {
	return filepath.Join(s.Paths.Config, "logs", "signin.log")
}

func (s *Service) logSignIn(format string, args ...any) {
	path := s.signInLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

// SubmitLoginCode passes the code from the sign-in page to the CLI and
// confirms the result with "auth status": the CLI prints "Login
// successful." and exits 0 even for a code it rejects.
func (s *Service) SubmitLoginCode(ctx context.Context, code string) (CLIStatus, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return CLIStatus{}, errors.New("paste the code from the sign-in page")
	}
	s.loginMu.Lock()
	p := s.login
	s.loginMu.Unlock()
	finished := p == nil
	if p != nil {
		select {
		case <-p.Done():
			finished = true
		default:
		}
	}
	if finished {
		// The CLI already stopped (it may have signed in on its own).
		st := s.CLIStatus(ctx, true)
		s.emit.Emit(EvtCLIStatus, st)
		if st.LoggedIn {
			return st, nil
		}
		return st, errors.New("that sign-in attempt has ended; press Sign in to start again")
	}
	before := len(p.Output())
	s.logSignIn("code submitted (%d characters)", len(code))
	if err := p.Send(code); err != nil {
		s.logSignIn("couldn't write the code: %v", err)
		return CLIStatus{}, fmt.Errorf("couldn't pass the code to the CLI: %w", err)
	}
	select {
	case <-p.Done():
	case <-time.After(90 * time.Second):
		s.CancelSignIn()
		return CLIStatus{}, errors.New("the CLI didn't finish signing in within 90 seconds")
	case <-ctx.Done():
		return CLIStatus{}, ctx.Err()
	}
	s.loginMu.Lock()
	if s.login == p {
		s.login = nil
	}
	s.loginMu.Unlock()
	st := s.CLIStatus(ctx, true)
	s.emit.Emit(EvtCLIStatus, st)
	s.logSignIn("after the code: exit %v, signed in %v; output: %q", p.Err(), st.LoggedIn, p.Output()[before:])
	if !st.LoggedIn {
		msg := "the code wasn't accepted"
		if out := string(p.Output()); len(out) > before {
			if l := loginProblem(out[before:]); l != "" {
				msg = l
			}
		}
		return st, errors.New(msg)
	}
	return st, nil
}

// CancelSignIn stops a sign-in in progress, if any.
func (s *Service) CancelSignIn() {
	s.loginMu.Lock()
	p := s.login
	s.login = nil
	s.loginMu.Unlock()
	if p != nil {
		p.Kill()
	}
}

// loginProblem picks the CLI's complaint out of its output, skipping the
// "Login successful." it prints regardless.
func loginProblem(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" && !strings.Contains(l, "Login successful") && !strings.HasPrefix(l, "Paste code") {
			return l
		}
	}
	return ""
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
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
	LastNew      NewSessionChoices     `json:"lastNewSession"`
	Preferences  Preferences           `json:"preferences"`
	Providers    []Provider            `json:"providers"`
}

// NewSessionChoices is what the user picked last time in the new-session
// dialog, the best guess for next time: the type, and per type the model
// and folder.
type NewSessionChoices struct {
	ProfileID string            `json:"profileId,omitempty"`
	Models    map[string]string `json:"models"`
	Folders   map[string]string `json:"folders"`
}

const settingLastNew = "ui.newSession.last"

// CreatedSession is a new session plus the updated remembered choices.
type CreatedSession struct {
	Session session.View      `json:"session"`
	LastNew NewSessionChoices `json:"lastNewSession"`
}

func (s *Service) lastNewSession() NewSessionChoices {
	c := NewSessionChoices{}
	if raw, _ := s.Store.Setting(settingLastNew); raw != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	if c.Models == nil {
		c.Models = map[string]string{}
	}
	if c.Folders == nil {
		c.Folders = map[string]string{}
	}
	return c
}

// CreateSession creates a session and remembers the choices for next time.
func (s *Service) CreateSession(profileID, workdir, model string) (session.View, NewSessionChoices, error) {
	v, err := s.Sessions.Create(profileID, workdir, model)
	if err != nil {
		return v, s.lastNewSession(), err
	}
	c := s.lastNewSession()
	c.ProfileID = v.ProfileID
	c.Models[v.ProfileID] = v.Model
	if p, ok := s.Profiles.Profile(v.ProfileID); ok && p.Folder != "scratch" {
		c.Folders[v.ProfileID] = v.Workdir
	}
	if b, err := json.Marshal(c); err == nil {
		_ = s.Store.SetSetting(settingLastNew, string(b))
	}
	return v, c, nil
}

func (s *Service) Bootstrap() Bootstrap {
	return Bootstrap{
		Profiles: s.Profiles.Profiles, Modifiers: s.Profiles.Modifiers, Toolbar: s.Profiles.Toolbar,
		Sessions: s.Sessions.List(), Models: s.Sessions.Models(), Capabilities: s.Adapter.Capabilities(),
		Platform: runtime.GOOS, LastNew: s.lastNewSession(),
		Preferences: s.Preferences(), Providers: s.Providers(),
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
