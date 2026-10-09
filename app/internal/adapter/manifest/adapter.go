package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"uncli/internal/core"
	"uncli/internal/hook"
)

// Adapter runs a CLI from its manifest (decision 0013).
type Adapter struct {
	M         *Manifest
	installer *Installer
	// StateDir is the CLI's state folder, UNCLI's own ({state}).
	StateDir string
	// Hook is the command line that runs UNCLI in hook mode.
	Hook string
	// Allowed says whether the user has enabled this plugin, as it is now;
	// a CLI is never started otherwise. nil: allowed (tests).
	Allowed func() error

	models *regexp.Regexp
	signed *regexp.Regexp
}

func New(m *Manifest, inst *Installer, stateDir, hookCmd string) *Adapter {
	a := &Adapter{M: m, installer: inst, StateDir: stateDir, Hook: hookCmd}
	if m.Models.Line != "" {
		a.models = regexp.MustCompile(m.Models.Line)
	}
	if m.Status.SignedIn != "" {
		a.signed = regexp.MustCompile(m.Status.SignedIn)
	}
	return a
}

func (a *Adapter) ID() string { return a.M.ID }

func (a *Adapter) Installer() core.Installer { return a.installer }

func (a *Adapter) StatePaths() []string {
	var out []string
	for _, p := range a.M.StatePaths {
		out = append(out, filepath.FromSlash(a.expand(p, nil)))
	}
	return out
}

// Capabilities: what the manifest says, with approvals when UNCLI's hook
// decides tool calls, and never what needs a control protocol.
func (a *Adapter) Capabilities() core.Capabilities {
	var c core.Capabilities
	b, _ := json.Marshal(a.M.Capabilities)
	_ = json.Unmarshal(b, &c)
	c.Approvals, c.HookApprovals = a.M.HookGated(), a.M.HookGated()
	c.LiveModelSwitch, c.Interrupt, c.LivePermissionMode, c.ToolHints = false, false, false, false
	if a.ACP() {
		// The agent asks UNCLI about tool calls, can be interrupted and resumes.
		c.Approvals, c.HookApprovals, c.Interrupt, c.Resume, c.PartialStreaming = true, false, true, true, true
	}
	c.Images, c.Documents, c.SlashPassthrough, c.Import = false, false, false, false
	return c
}

// expand fills {state} and the given names.
func (a *Adapter) expand(s string, more map[string]string) string {
	m := map[string]string{"state": a.StateDir}
	for k, v := range more {
		m[k] = v
	}
	return expand(s, vars(m, nil))
}

func (a *Adapter) expandAll(args []string, more map[string]string) []string {
	out := make([]string, len(args))
	for i, s := range args {
		out[i] = a.expand(s, more)
	}
	return out
}

func (a *Adapter) env(extra map[string]string) map[string]string {
	env := map[string]string{}
	for k, v := range a.M.Env {
		env[k] = a.expand(v, nil) // {state}
	}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

func (a *Adapter) BuildCommand(bin string, s core.LaunchSpec) (core.Command, error) {
	if a.Allowed != nil {
		if err := a.Allowed(); err != nil {
			return core.Command{}, err
		}
	}
	if a.M.State && a.StateDir == "" {
		return core.Command{}, errors.New("this provider has no state folder")
	}
	if err := a.writeFiles(a.M.Files, nil); err != nil {
		return core.Command{}, err
	}
	gated := s.Approvals && a.M.HookGated()
	if gated {
		if err := a.WriteHooks(); err != nil {
			return core.Command{}, err
		}
	}
	vals := map[string]string{"resume": s.ResumeID, "id": s.SessionID, "model": s.Model, "effort": s.Effort}
	args := a.expandAll(a.M.Launch.Args, vals)
	if gated {
		args = append(args, a.expandAll(a.M.Launch.Approvals, vals)...)
	}
	switch {
	case s.ResumeID != "":
		args = append(args, a.expandAll(a.M.Launch.Resume, vals)...)
	case s.SessionID != "":
		args = append(args, a.expandAll(a.M.Launch.NewID, vals)...)
	}
	if s.Model != "" {
		args = append(args, a.expandAll(a.M.Launch.Model, vals)...)
	}
	if s.Effort != "" {
		args = append(args, a.expandAll(a.M.Launch.Effort, vals)...)
	}
	if s.PermissionMode == "plan" {
		args = append(args, a.expandAll(a.M.Launch.Plan, vals)...)
	}
	args = append(args, a.expandAll(a.M.Launch.Tail, vals)...)
	env := a.env(s.Env)
	if gated {
		env[hook.CmdEnv] = a.Hook
	}
	return core.Command{Path: bin, Args: args, Env: env}, nil
}

// WriteHooks writes the hook's files into the state folder.
func (a *Adapter) WriteHooks() error {
	h := a.M.Approvals.Hook
	if h == nil {
		return nil
	}
	if a.Hook == "" {
		return fmt.Errorf("no hook command for %s", a.M.Name)
	}
	lookup := func(name string) (string, bool) {
		if ev, ok := strings.CutPrefix(name, "hook:"); ok {
			return hook.Command(runtime.GOOS, ev), true
		}
		return "", false
	}
	return a.writeFiles(h.Files, lookup)
}

// writeFiles writes files into the state folder: text as it is, anything
// else as JSON, with lookup's placeholders filled.
func (a *Adapter) writeFiles(files map[string]any, lookup func(string) (string, bool)) error {
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	for rel, content := range files {
		path := filepath.FromSlash(a.expand(rel, nil))
		var b []byte
		if s, ok := content.(string); ok {
			b = []byte(expand(s, lookup))
		} else {
			var err error
			if b, err = json.MarshalIndent(fill(content, lookup), "", "  "); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := writeIfChanged(path, b); err != nil {
			return err
		}
	}
	return nil
}

func writeIfChanged(path string, b []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *Adapter) AuthStatusCommand(bin string) core.Command {
	return core.Command{Path: bin, Args: a.expandAll(a.M.Status.Args, nil), Env: a.env(nil)}
}

// ParseAuthStatus: signed in when the output says so, or doesn't say it
// isn't, or lists models.
func (a *Adapter) ParseAuthStatus(out []byte) (core.AuthInfo, error) {
	var in bool
	switch {
	case a.signed != nil:
		in = a.signed.Match(out)
	case a.M.Status.SignedOut != "":
		in = len(strings.TrimSpace(string(out))) > 0 && !regexp.MustCompile(a.M.Status.SignedOut).Match(out)
	default:
		in = len(a.ParseModels(out)) > 0
	}
	return core.AuthInfo{LoggedIn: in, Method: a.M.Status.Method}, nil
}

// ACP says whether the CLI speaks the Agent Client Protocol (decision 0014).
func (a *Adapter) ACP() bool { return a.M.Protocol == "acp" }

// NewDriver gives an ACP CLI's process its driver; nil for the others,
// which the session reads with the parser.
func (a *Adapter) NewDriver(spec core.LaunchSpec) core.Driver {
	if !a.ACP() {
		return nil
	}
	return newACP(a, spec)
}

// ParseModels reads the models, one a line: an id and maybe a name.
func (a *Adapter) ParseModels(out []byte) []core.ModelInfo {
	if a.models == nil {
		return nil
	}
	var ms []core.ModelInfo
	for _, l := range strings.Split(string(out), "\n") {
		g := a.models.FindStringSubmatch(strings.TrimSpace(l))
		if len(g) < 2 || g[1] == "" {
			continue
		}
		m := core.ModelInfo{Value: g[1], Resolved: g[1]}
		if len(g) > 2 {
			m.DisplayName = strings.TrimSpace(g[2])
		}
		ms = append(ms, m)
	}
	return ms
}

// LoginCommand: plugins' CLIs sign in in their own interface, which UNCLI
// opens in a terminal window.
func (a *Adapter) LoginCommand(bin string) core.Command {
	return core.Command{Path: bin, Args: a.expandAll(a.M.Status.Login, nil), Env: a.env(nil)}
}

func (a *Adapter) LoginURL([]byte) (string, bool) { return "", false }

// EncodeTurn writes the turn line with the message in it, directives first.
func (a *Adapter) EncodeTurn(t core.UserTurn) ([]byte, error) {
	if len(t.Attachments) > 0 {
		return nil, fmt.Errorf("%s can't be sent files", a.M.Name)
	}
	msg := t.Text
	if t.Directives != "" {
		msg = strings.TrimSpace(t.Directives + "\n\n" + t.Text)
	}
	b, err := json.Marshal(fill(a.M.Turn.Line, func(name string) (string, bool) {
		if name == "text" {
			return msg, true
		}
		return "", false
	}))
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// EncodeControl: plugins' CLIs take no control messages.
func (a *Adapter) EncodeControl(core.Control) ([]byte, bool) { return nil, false }

func (a *Adapter) NewParser() core.Parser { return newParser(a) }

func (a *Adapter) LostConversation(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, w := range a.M.LostConversation {
		if w != "" && strings.Contains(s, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

var (
	ansiRE        = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07`)
	defaultURLRE  = regexp.MustCompile(`https://\S+`)
	defaultCodeRE = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)
)

// DeviceSignIn finds the link and code a device sign-in printed, once it
// has printed both.
func (a *Adapter) DeviceSignIn(out []byte) (url, code string, ok bool) {
	if a.M.SignIn != "device" {
		return "", "", false
	}
	s := ansiRE.ReplaceAllString(string(out), "")
	ure, cre := defaultURLRE, defaultCodeRE
	if a.M.Status.DeviceURL != "" {
		ure = regexp.MustCompile(a.M.Status.DeviceURL)
	}
	if a.M.Status.DeviceCode != "" {
		cre = regexp.MustCompile(a.M.Status.DeviceCode)
	}
	url, code = ure.FindString(s), cre.FindString(s)
	return url, code, url != "" && code != ""
}
