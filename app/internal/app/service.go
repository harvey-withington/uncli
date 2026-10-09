// Package app wires UNCLI together: store, profiles, the Claude adapter and
// installer, the local runtime and the session manager. It knows nothing
// about Wails; the bridge exposes it to the UI.
package app

import (
	"context"
	"encoding/base64"
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
	"uncli/internal/adapter/antigravity"
	"uncli/internal/adapter/claude"
	"uncli/internal/artifacts"
	"uncli/internal/attach"
	"uncli/internal/core"
	"uncli/internal/hook"
	"uncli/internal/ide"
	"uncli/internal/notify"
	"uncli/internal/profile"
	"uncli/internal/runtime/local"
	"uncli/internal/runtime/wsl"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Settings keys.
const (
	SettingCLIVersion = "cli.claude.version" // empty = the pinned version (cliVersionKey)
	SettingCLIPath    = "cli.claude.path"    // a binary to use instead of a managed one (development)
)

// HookCommandEnv runs a different command as the CLIs' hook (tests);
// otherwise it is UNCLI itself in hook mode (main.go).
const HookCommandEnv = "UNCLI_HOOK_COMMAND"

// hookCommand is the command line that runs UNCLI's hook (decision 0012).
func hookCommand() string {
	if c := os.Getenv(HookCommandEnv); c != "" {
		return c
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return `"` + exe + `" hook`
}

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
	Adapter   *claude.Adapter // the first provider
	Installer *claude.Installer
	Sessions  *session.Manager

	clis    []*cliProvider // every provider's CLI, the first first (clis.go), plugins last
	plugins []*plugin      // provider plugins, loaded or not (plugins.go)
	hooks   *hook.Server   // where CLIs' hooks ask about tool calls

	// ShowWindow brings UNCLI's window to the front (set by the bridge);
	// clicking a notification uses it.
	ShowWindow func()

	emit          Emitter
	notifier      notify.Notifier
	artifactStore *artifacts.Store
	containers    containers
	providers     []Provider               // names for the interface (providers.yaml)
	transcripts   *core.TranscriptLocation // where saved conversations are read from; nil: the CLI's own (tests set it)

	signInMu        sync.Mutex
	containerSignIn *containerSignIn              // setup-token waiting for its code
	accountLogin    *accountSignIn                // the full account sign-in waiting for its code
	watching        map[string]bool               // providers whose terminal sign-in is being watched for
	devices         map[string]*local.Interactive // device sign-ins in progress, by provider

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
	agyInst := antigravity.NewInstaller(filepath.Join(paths.Cache, "cli", "antigravity"))
	agy := antigravity.New(agyInst, filepath.Join(paths.Config, "agy"), hookCommand())
	s.clis = []*cliProvider{
		{adapter: s.Adapter, installer: inst, binEnv: "UNCLI_CLAUDE_BIN"},
		{adapter: agy, installer: agyInst, binEnv: "UNCLI_AGY_BIN"},
	}
	s.plugins = s.loadPlugins([]string{s.Adapter.ID(), agy.ID()})
	s.clis = append(s.clis, s.pluginAdapters()...)
	others := []core.Adapter{}
	for _, c := range s.clis[1:] {
		others = append(others, c.adapter)
	}
	if s.hooks, err = hook.Listen(); err != nil {
		db.Close()
		return nil, err
	}
	s.notifier = newNotifier(s.notifyClicked)
	s.containers.cfg, s.containers.err = loadContainers(defaults, paths.Config)
	if s.providers, err = loadProviders(defaults); err != nil {
		db.Close()
		return nil, err
	}
	s.containers.building, s.containers.failed, s.containers.runtimes = map[string]string{}, map[string]string{}, map[string]*wsl.Runtime{}
	s.containers.percent = map[string]int{}
	s.artifactStore = artifacts.NewStore(filepath.Join(paths.Config, "artifact-store"))
	sink := newAttention(newCoalescer(emit, 50*time.Millisecond),
		func(id string) bool { return s.Sessions != nil && s.Sessions.Focused(id) },
		func() string { return s.Preferences().Notifications },
		func(n notify.Note) { go s.showNote(n) })
	s.Sessions, err = session.NewManager(session.Deps{
		Store: db, Adapter: s.Adapter, Others: others, Hooks: s.hooks,
		Runtime: local.New(), Runtimes: s.runtimeFor, Profiles: set,
		Binary: s.binaryOf, Sink: sink, ScratchDir: paths.Scratch, Artifacts: s.artifactStore,
		Judge: s.judgeCommand, JudgeTools: s.judgeTools, Unknown: func() string { return s.Preferences().UnknownCommands }, DeciderKey: s.deciderKey,
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Service) Close() {
	s.CancelSignIn()
	s.signInMu.Lock()
	for id := range s.devices {
		defer s.CancelDeviceSignIn(id)
	}
	s.signInMu.Unlock()
	s.Sessions.Close()
	s.CancelContainerSignIn()
	s.CancelContainerAccountSignIn()
	s.closeContainers()
	s.notifier.Close()
	_ = s.hooks.Close()
	s.Store.Close()
}

// cliVersion is the version the user runs: the pinned one unless they
// chose another at their own risk.
func (s *Service) cliVersion() string { return s.versionOf(s.clis[0]) }

// binary is the first provider's CLI binary and its version.
func (s *Service) binary(ctx context.Context) (string, string, error) {
	return s.binaryOf(ctx, s.Adapter.ID())
}

type CLIStatus struct {
	Provider     string `json:"provider"` // the provider's id
	Installed    bool   `json:"installed"`
	Version      string `json:"version"`
	Pinned       string `json:"pinned"`
	Custom       bool   `json:"custom"` // a binary path override is in use
	LoggedIn     bool   `json:"loggedIn"`
	Email        string `json:"email,omitempty"`
	Subscription string `json:"subscription,omitempty"`
	Error        string `json:"error,omitempty"`
	// Models the account can use, for CLIs whose sign-in check lists them.
	Models []core.ModelInfo `json:"models,omitempty"`
}

// CLIStatus reports whether the first provider's CLI is installed and
// signed in (the setup screen's question).
func (s *Service) CLIStatus(ctx context.Context, fresh bool) CLIStatus {
	return s.ProviderStatus(ctx, s.Adapter.ID(), fresh)
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// InstallCLI downloads the first provider's selected CLI version.
func (s *Service) InstallCLI(ctx context.Context) (CLIStatus, error) {
	return s.InstallProvider(ctx, s.Adapter.ID())
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
			s.emitStatus(st)
			if st.LoggedIn {
				return SignInStart{SignedIn: true, Status: st}, nil
			}
			return SignInStart{}, fmt.Errorf("the CLI's sign-in stopped without showing a link (%s). Details are in %s",
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
	s.emitStatus(st)
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
		s.emitStatus(st)
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
	s.emitStatus(st)
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

// SetCLIVersion selects the first provider's CLI version; empty returns to
// the pinned one.
func (s *Service) SetCLIVersion(ctx context.Context, version string) (CLIStatus, error) {
	return s.SetProviderVersion(ctx, s.Adapter.ID(), version)
}

func (s *Service) CLIChannels(ctx context.Context) (map[string]string, error) {
	return s.ProviderChannels(ctx, s.Adapter.ID())
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
	Editors      []ide.Editor          `json:"editors"`
}

// NewSessionChoices is what the user picked last time in the new-session
// dialog, the best guess for next time: the type, and per type the model
// and folder.
type NewSessionChoices struct {
	ProfileID string            `json:"profileId,omitempty"`
	Models    map[string]string `json:"models"`
	Folders   map[string]string `json:"folders"`
	// Containers is where each profile last ran: a container profile id, or
	// none for this machine.
	Containers map[string]string `json:"containers"`
	Providers  map[string]string `json:"providers"` // the AI provider each profile last ran on
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
	if c.Containers == nil {
		c.Containers = map[string]string{}
	}
	if c.Providers == nil {
		c.Providers = map[string]string{}
	}
	return c
}

// CreateSession creates a session and remembers the choices for next time.
func (s *Service) CreateSession(profileID, workdir, model string) (session.View, NewSessionChoices, error) {
	return s.CreateSessionIn(profileID, workdir, model, "")
}

// CreateSessionIn creates a session that runs in a container profile (empty:
// on this machine) and remembers the choices for next time.
func (s *Service) CreateSessionIn(profileID, workdir, model, container string) (session.View, NewSessionChoices, error) {
	return s.CreateSessionWith(session.NewSession{Profile: profileID, Workdir: workdir, Model: model, Container: container})
}

// CreateSessionWith creates a session on a provider (empty: the first) and
// remembers the choices for next time.
func (s *Service) CreateSessionWith(n session.NewSession) (session.View, NewSessionChoices, error) {
	if err := s.pluginAllowed(n.Provider); err != nil {
		return session.View{}, s.lastNewSession(), err
	}
	v, err := s.Sessions.CreateWith(n)
	if err != nil {
		return v, s.lastNewSession(), err
	}
	c := s.lastNewSession()
	c.ProfileID = v.ProfileID
	c.Models[v.ProfileID] = v.Model
	c.Containers[v.ProfileID] = v.RuntimeRef
	c.Providers[v.ProfileID] = v.Adapter
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
		Preferences: s.Preferences(), Providers: s.Providers(), Editors: s.Editors(),
	}
}

// DroppedPath is a file or folder dropped onto the window.
type DroppedPath struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Dir   string `json:"dir"` // the folder itself, or the file's folder
}

// DescribePaths says which dropped paths are folders; paths that no longer
// exist are left out.
func DescribePaths(paths []string) []DroppedPath {
	out := []DroppedPath{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		d := DroppedPath{Path: p, IsDir: st.IsDir(), Dir: p}
		if !d.IsDir {
			d.Dir = filepath.Dir(p)
		}
		out = append(out, d)
	}
	return out
}

// AttachmentRef is a file the UI wants sent with a turn: a path to read,
// or pasted image data (base64) with no file behind it.
type AttachmentRef struct {
	Path      string `json:"path,omitempty"`
	Name      string `json:"name,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Data      string `json:"data,omitempty"`
}

// Send starts a turn in a session, reading its attachments first; one that
// can't be attached stops the turn with the reason.
func (s *Service) Send(ctx context.Context, sessionID, text string, refs []AttachmentRef) error {
	files := make([]core.Attachment, 0, len(refs))
	for _, r := range refs {
		var f core.Attachment
		var err error
		if r.Path != "" {
			f, err = attach.Load(r.Path)
		} else {
			var data []byte
			if data, err = base64.StdEncoding.DecodeString(r.Data); err == nil {
				f, err = attach.FromData(r.Name, r.MediaType, data)
			}
		}
		if err != nil {
			return err
		}
		files = append(files, f)
	}
	return s.Sessions.Send(ctx, sessionID, text, files...)
}

// DescribeAttachments says, for each dropped or pasted path, whether it
// can be attached (and as what) or why not.
func DescribeAttachments(paths []string) []attach.Info {
	out := make([]attach.Info, 0, len(paths))
	for _, p := range paths {
		out = append(out, attach.Describe(p))
	}
	return out
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
