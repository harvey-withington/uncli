// Package antigravity is the Antigravity CLI (agy) adapter: installer,
// command builder, stdin encoder, stream parser and the hook protocol UNCLI
// approves its tool calls through (decision 0012). Behaviour is pinned by
// the recorded streams in testdata/streams/antigravity/<version>/.
package antigravity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"uncli/internal/core"
)

// PinnedVersion is the Antigravity CLI release this build of UNCLI is tested with.
const PinnedVersion = "1.3.1"

type Adapter struct {
	installer *Installer
	// StateDir is the CLI's state folder, UNCLI's own (--gemini_dir): its
	// conversations, logs and configuration, including UNCLI's hooks.
	StateDir string
	// Hook is the command line that runs UNCLI in hook mode; the event name
	// is appended (pre-tool, pre-invocation).
	Hook string
}

func New(installer *Installer, stateDir, hook string) *Adapter {
	return &Adapter{installer: installer, StateDir: stateDir, Hook: hook}
}

func (a *Adapter) ID() string { return "antigravity" }

func (a *Adapter) Installer() core.Installer { return a.installer }

// StatePaths: writing to the CLI's own state (its brain, artifacts and
// transcripts) is its bookkeeping, not a change to the user's files.
func (a *Adapter) StatePaths() []string {
	return []string{filepath.Join(a.StateDir, "antigravity-cli")}
}

// Capabilities: approvals come through UNCLI's hook, not a control
// protocol; nothing can be changed on a running process, so interrupting
// stops it and a new model means a respawn. Only text can be sent.
func (a *Adapter) Capabilities() core.Capabilities {
	return core.Capabilities{
		PartialStreaming: true, Resume: true, Approvals: true, HookApprovals: true, UsageReporting: true,
	}
}

// childEnv is applied to every CLI process: the pinned binary must never
// update itself.
func childEnv(extra map[string]string) map[string]string {
	env := map[string]string{"AGY_CLI_DISABLE_AUTO_UPDATE": "true"}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

func (a *Adapter) stateArg() string { return "--gemini_dir=" + a.StateDir }

// BuildCommand starts a streaming session. With approvals, the CLI's own
// permission checks are off and UNCLI's hook decides every tool call
// (decision 0012); without them the CLI denies anything that needs
// permission, as it does headless. Profiles' tool lists and permission
// modes are UNCLI's to apply (through the hook), not the CLI's.
func (a *Adapter) BuildCommand(bin string, s core.LaunchSpec) (core.Command, error) {
	if a.StateDir == "" {
		return core.Command{}, errors.New("the Antigravity adapter has no state folder")
	}
	if s.Approvals {
		if err := a.WriteHooks(); err != nil {
			return core.Command{}, err
		}
	}
	args := []string{a.stateArg(), "--input-format", "stream-json", "--output-format", "stream-json"}
	if s.Approvals {
		args = append(args, "--dangerously-skip-permissions")
	}
	if s.ResumeID != "" {
		args = append(args, "--conversation", s.ResumeID)
	}
	if s.Model != "" {
		args = append(args, "--model", s.Model)
	}
	if s.Effort != "" {
		args = append(args, "--effort", s.Effort)
	}
	if s.PermissionMode == "plan" {
		args = append(args, "--mode", "plan")
	}
	env := childEnv(s.Env)
	if s.Approvals {
		env[HookCmdEnv] = a.Hook
	}
	return core.Command{Path: bin, Args: args, Env: env}, nil
}

// AuthStatusCommand lists the models, which needs a signed-in account; the
// CLI has no status command of its own.
func (a *Adapter) AuthStatusCommand(bin string) core.Command {
	return core.Command{Path: bin, Args: []string{a.stateArg(), "models"}, Env: childEnv(nil)}
}

// ParseAuthStatus: a model list means the CLI is signed in.
func (a *Adapter) ParseAuthStatus(out []byte) (core.AuthInfo, error) {
	ms := ParseModels(out)
	return core.AuthInfo{LoggedIn: len(ms) > 0, Method: "Google"}, nil
}

// ParseModels lists the models from the sign-in check's output.
func (a *Adapter) ParseModels(out []byte) []core.ModelInfo { return ParseModels(out) }

// ParseModels reads "agy models": one model a line, its id and display
// name separated by a tab.
func ParseModels(out []byte) []core.ModelInfo {
	var ms []core.ModelInfo
	for _, l := range strings.Split(string(out), "\n") {
		id, name, ok := strings.Cut(strings.TrimSpace(l), "\t")
		if !ok || id == "" || strings.ContainsAny(id, " :") {
			continue
		}
		ms = append(ms, core.ModelInfo{Value: id, Resolved: id, DisplayName: strings.TrimSpace(name)})
	}
	return ms
}

// LoginCommand: the CLI signs in only in its own terminal interface, and
// shares the sign-in with the Antigravity app through the OS keyring.
func (a *Adapter) LoginCommand(bin string) core.Command {
	return core.Command{Path: bin, Args: []string{a.stateArg()}, Env: childEnv(nil)}
}

func (a *Adapter) LoginURL([]byte) (string, bool) { return "", false }

// EncodeTurn writes one user message. Directives go first, so the model
// reads them before the question; the UI shows only the question.
func (a *Adapter) EncodeTurn(t core.UserTurn) ([]byte, error) {
	if len(t.Attachments) > 0 {
		return nil, errors.New("the Antigravity CLI can't be sent files")
	}
	text := t.Text
	if t.Directives != "" {
		text = strings.TrimSpace(t.Directives + "\n\n" + t.Text)
	}
	return marshalLine(map[string]any{"event": "user", "message": map[string]any{"content": text}})
}

// EncodeControl: the CLI takes no control messages; slash commands sent
// as text end a streaming session, so UNCLI doesn't send them either.
func (a *Adapter) EncodeControl(core.Control) ([]byte, bool) { return nil, false }

func (a *Adapter) NewParser() core.Parser { return newParser() }

// LostConversation: asked to resume a conversation it doesn't have.
func (a *Adapter) LostConversation(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "conversation") && (strings.Contains(s, "not found") || strings.Contains(s, "no such"))
}

func marshalLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// HookCmdEnv carries UNCLI's hook command line to the CLI's hooks.
const HookCmdEnv = "UNCLI_HOOK_CMD"

// hookCommand is what hooks.json runs for an event. The CLI hands it to
// cmd /c on Windows, escaping any quotes in it so cmd can't read them, and
// doesn't run it in a folder of UNCLI's choosing (recorded 1.3.1). So the
// command names an environment variable that holds UNCLI's command line,
// quotes and all: cmd expands it after the escaping, and sh reads it again
// with eval. Without the variable the command isn't found, and the call
// is refused.
func hookCommand(event string) string {
	if runtime.GOOS == "windows" {
		return "%" + HookCmdEnv + "% " + event
	}
	return `eval "$` + HookCmdEnv + ` ` + event + `"`
}

// WriteHooks puts UNCLI's hooks into the CLI's state folder: every tool
// call asks UNCLI (pre-tool), and every model call gets the session's
// instructions (pre-invocation). The pre-tool hook waits as long as the
// user takes; anything but UNCLI's "allow" blocks the call.
func (a *Adapter) WriteHooks() error {
	if a.Hook == "" {
		return errors.New("no hook command for the Antigravity CLI")
	}
	dir := filepath.Join(a.StateDir, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	hooks := map[string]any{
		"uncli": map[string]any{
			"PreToolUse": []any{map[string]any{"matcher": "*", "hooks": []any{
				map[string]any{"type": "command", "command": hookCommand(EventPreTool), "timeout": HookTimeout},
			}}},
			"PreInvocation": []any{
				map[string]any{"type": "command", "command": hookCommand(EventPreInvocation), "timeout": 30},
			},
		},
	}
	b, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		return err
	}
	return writeIfChanged(filepath.Join(dir, "hooks.json"), b, 0o644)
}

// HookTimeout is how long a tool call may wait for the user, in seconds.
// The CLI blocks the call when it passes.
const HookTimeout = 24 * 60 * 60

func writeIfChanged(path string, b []byte, mode os.FileMode) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
