// Package claude is the Claude Code CLI adapter: installer, command
// builder, stdin encoders and the stream-json parser. Behaviour is pinned by
// the recorded streams in testdata/streams/claude/<version>/.
package claude

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"uncli/internal/core"
)

// PinnedVersion is the Claude Code release this build of UNCLI is tested with.
const PinnedVersion = "2.1.285"

type Adapter struct {
	installer *Installer
}

func New(installer *Installer) *Adapter { return &Adapter{installer: installer} }

func (a *Adapter) ID() string { return "claude" }

func (a *Adapter) Installer() core.Installer { return a.installer }

// StatePaths are the folders the CLI keeps its own files in (plans,
// per-project memory and transcripts, todo lists): writing there is its
// bookkeeping. Its settings and credentials are not among them.
func (a *Adapter) StatePaths() []string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		dir = filepath.Join(home, ".claude")
	}
	return []string{filepath.Join(dir, "plans"), filepath.Join(dir, "projects"), filepath.Join(dir, "todos")}
}

func (a *Adapter) Capabilities() core.Capabilities {
	return core.Capabilities{
		PartialStreaming: true, Resume: true, LiveModelSwitch: true, Interrupt: true,
		Approvals: true, LivePermissionMode: true, ToolHints: true,
		Images: true, Documents: true, UsageReporting: true, ThinkingEvents: true, SlashPassthrough: true,
		Import: true,
	}
}

// childEnv is applied to every CLI process. UNCLI may itself be launched
// from a Claude session, whose variables (session ids, effort, entrypoint)
// would leak into the child, and the pinned binary must never update itself.
// The CLI runs on a JavaScript runtime, so debugger settings meant for Node
// break it too: VS Code's auto-attach puts NODE_OPTIONS=--require
// bootloader.js into every terminal, and with it the CLI exits 1 and prints
// nothing. CLAUDE_CONFIG_DIR and API keys are the user's choice and pass through.
func childEnv(extra map[string]string) (map[string]string, []string) {
	env := map[string]string{"DISABLE_AUTOUPDATER": "1"}
	for k, v := range extra {
		env[k] = v
	}
	return env, []string{"CLAUDE_CODE_", "CLAUDECODE", "CLAUDE_EFFORT", "CLAUDE_PID", "CLAUDE_AGENT_SDK",
		"NODE_OPTIONS", "NODE_INSPECT", "VSCODE_INSPECTOR_OPTIONS", "BUN_INSPECT"}
}

var permissionModes = map[string]bool{"": true, "default": true, "manual": true, "acceptEdits": true, "dontAsk": true, "plan": true}

func (a *Adapter) BuildCommand(bin string, s core.LaunchSpec) (core.Command, error) {
	if !permissionModes[s.PermissionMode] {
		return core.Command{}, errors.New("unsupported permission mode: " + s.PermissionMode)
	}
	args := []string{"-p",
		"--input-format", "stream-json", "--output-format", "stream-json",
		"--verbose", "--include-partial-messages"}
	switch {
	case s.ResumeID != "":
		args = append(args, "--resume", s.ResumeID)
	case s.SessionID != "":
		args = append(args, "--session-id", s.SessionID)
	}
	if s.Model != "" {
		args = append(args, "--model", s.Model)
	}
	if s.Effort != "" {
		args = append(args, "--effort", s.Effort)
	}
	if s.SystemPrompt != "" {
		args = append(args, "--system-prompt", s.SystemPrompt)
	}
	if s.AppendPrompt != "" {
		args = append(args, "--append-system-prompt", s.AppendPrompt)
	}
	if len(s.Tools) > 0 {
		args = append(args, "--tools", strings.Join(s.Tools, ","))
	}
	// Patterns such as "Bash(git status:*)" contain spaces, so each one is
	// its own argument and the list always ends before another flag.
	if len(s.AllowedTools) > 0 {
		args = append(append(args, "--allowedTools"), s.AllowedTools...)
	}
	if len(s.DisallowedTools) > 0 {
		args = append(append(args, "--disallowedTools"), s.DisallowedTools...)
	}
	if s.PermissionMode != "" {
		args = append(args, "--permission-mode", s.PermissionMode)
	}
	if s.Approvals {
		args = append(args, "--permission-prompt-tool", "stdio")
	} else {
		args = append(args, "--permission-prompts", "none")
	}
	for _, c := range s.MCPConfig {
		args = append(args, "--mcp-config", c)
	}
	if s.Isolated {
		args = append(args, "--strict-mcp-config", "--setting-sources", "", "--disable-slash-commands")
	}
	env, drop := childEnv(s.Env)
	return core.Command{Path: bin, Args: args, Env: env, EnvDrop: drop}, nil
}

func (a *Adapter) AuthStatusCommand(bin string) core.Command {
	env, drop := childEnv(nil)
	return core.Command{Path: bin, Args: []string{"auth", "status"}, Env: env, EnvDrop: drop}
}

func (a *Adapter) LoginCommand(bin string) core.Command {
	env, drop := childEnv(nil)
	return core.Command{Path: bin, Args: []string{"auth", "login", "--claudeai"}, Env: env, EnvDrop: drop}
}

var loginURLRe = regexp.MustCompile(`https://\S+/oauth/authorize\?\S+`)

// LoginURL finds the sign-in link in what "claude auth login" printed.
// Without a terminal the CLI can't reliably open the browser itself, so
// UNCLI opens this link and passes back the code the page shows.
func (a *Adapter) LoginURL(output []byte) (string, bool) {
	u := loginURLRe.Find(output)
	return string(u), u != nil
}

func (a *Adapter) ParseAuthStatus(out []byte) (core.AuthInfo, error) {
	var s struct {
		LoggedIn         bool   `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		Email            string `json:"email"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if err := json.Unmarshal(out, &s); err != nil {
		return core.AuthInfo{}, err
	}
	return core.AuthInfo{LoggedIn: s.LoggedIn, Method: s.AuthMethod, Email: s.Email, Subscription: s.SubscriptionType}, nil
}

// EncodeTurn writes one user message. Directives go first, so the model
// reads them before the question; the UI shows only the question.
func (a *Adapter) EncodeTurn(t core.UserTurn) ([]byte, error) {
	text := t.Text
	if t.Directives != "" {
		text = strings.TrimSpace(t.Directives + "\n\n" + t.Text)
	}
	var content any = text
	if len(t.Attachments) > 0 {
		// Attachments first, then the text (a text block can't be empty).
		parts := []map[string]any{}
		for _, at := range t.Attachments {
			block, err := attachmentBlock(at)
			if err != nil {
				return nil, err
			}
			parts = append(parts, block)
		}
		if text != "" {
			parts = append(parts, map[string]any{"type": "text", "text": text})
		}
		content = parts
	}
	return marshalLine(map[string]any{
		"type":               "user",
		"message":            map[string]any{"role": "user", "content": content},
		"parent_tool_use_id": nil,
		"session_id":         "",
	})
}

// attachmentBlock is a file as a content block: images as image blocks,
// PDFs and text as documents titled with the file name (recorded in
// image-input and document-input).
func attachmentBlock(at core.Attachment) (map[string]any, error) {
	b64 := func() string { return base64.StdEncoding.EncodeToString(at.Data) }
	switch {
	case strings.HasPrefix(at.MediaType, "image/"):
		return map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": at.MediaType, "data": b64()}}, nil
	case at.MediaType == "application/pdf":
		return map[string]any{"type": "document", "title": at.Name, "source": map[string]any{"type": "base64", "media_type": at.MediaType, "data": b64()}}, nil
	case at.MediaType == "text/plain":
		return map[string]any{"type": "document", "title": at.Name, "source": map[string]any{"type": "text", "media_type": at.MediaType, "data": string(at.Data)}}, nil
	}
	return nil, fmt.Errorf("can't send %s: %s attachments aren't supported", at.Name, at.MediaType)
}

func (a *Adapter) EncodeControl(c core.Control) ([]byte, bool) {
	var req map[string]any
	switch c.Kind {
	case core.CtlInitialize:
		req = map[string]any{"subtype": "initialize", "hooks": nil}
	case core.CtlInterrupt:
		req = map[string]any{"subtype": "interrupt"}
	case core.CtlSetModel:
		req = map[string]any{"subtype": "set_model", "model": c.Model}
	case core.CtlSetPermissionMode:
		if !permissionModes[c.Mode] || c.Mode == "" {
			return nil, false
		}
		req = map[string]any{"subtype": "set_permission_mode", "mode": c.Mode}
	case core.CtlToolHints:
		req = map[string]any{"subtype": "mcp_status"}
	case core.CtlApprove:
		resp := map[string]any{"behavior": "deny", "message": c.Message}
		if c.Allow {
			input := c.UpdatedInput
			if input == nil {
				input = json.RawMessage("{}")
			}
			resp = map[string]any{"behavior": "allow", "updatedInput": input}
		}
		b, err := marshalLine(map[string]any{"type": "control_response", "response": map[string]any{
			"subtype": "success", "request_id": c.RequestID, "response": resp,
		}})
		return b, err == nil
	default:
		return nil, false
	}
	id := c.RequestID
	if id == "" {
		id = newRequestID()
	}
	b, err := marshalLine(map[string]any{"type": "control_request", "request_id": id, "request": req})
	return b, err == nil
}

func (a *Adapter) NewParser() core.Parser { return newParser() }

func marshalLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "uncli_" + hex.EncodeToString(b[:])
}

// LostConversation reports whether the CLI, asked to resume a conversation,
// said it no longer has it (CLI 2.1.285: "No conversation found with session
// ID: <id>" on stderr, an error result with no turns, exit 1). Its files
// were removed: deleted, or with a container that was rebuilt.
func (a *Adapter) LostConversation(stderr string) bool {
	return strings.Contains(stderr, "No conversation found with session ID")
}
