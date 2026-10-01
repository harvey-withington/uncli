package core

import (
	"context"
	"encoding/json"
	"io"
)

// Adapter: one per CLI provider. Phase 1: claude.
type Adapter interface {
	ID() string
	Capabilities() Capabilities
	Installer() Installer
	BuildCommand(bin string, spec LaunchSpec) (Command, error) // args, env
	AuthStatusCommand(bin string) Command                      // e.g. claude auth status
	ParseAuthStatus(out []byte) (AuthInfo, error)
	LoginCommand(bin string) Command        // the provider's browser sign-in; reads a pasted code on stdin
	LoginURL(output []byte) (string, bool)  // the sign-in link the login command printed, once it has
	EncodeTurn(t UserTurn) ([]byte, error)  // one stdin line
	EncodeControl(c Control) ([]byte, bool) // false = unsupported
	NewParser() Parser                      // stateful, one per process
}

type Parser interface {
	// Feed parses one stdout line. Unknown types become EvUnknown, never an
	// error; a line the adapter knows but has nothing to report yields no events.
	Feed(line []byte) ([]Event, error)
}

// Installer: UNCLI downloads and pins each CLI itself, so a CLI update
// on the user's machine never breaks UNCLI.
type Installer interface {
	Pinned() string                                                                               // version this build is tested with
	Installed() ([]string, error)                                                                 // versions in UNCLI's own cache
	Path(version string) (bin string, ok bool)                                                    // installed binary, if present
	Ensure(ctx context.Context, version string, progress func(done, total int64)) (string, error) // download if needed
	Channels(ctx context.Context) (map[string]string, error)                                      // e.g. stable, latest -> version
}

// Runtime: where the process lives. Phase 1: local. Phase 5: container.
type Runtime interface {
	ID() string
	Start(ctx context.Context, cmd Command, workdir string) (Proc, error)
}

type Proc interface {
	Stdin() io.Writer
	Stdout() io.Reader
	Stderr() io.Reader
	Wait() error
	Kill() error
}

// Command is what an adapter asks a runtime to run.
type Command struct {
	Path    string            `json:"path"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`     // added or overridden
	EnvDrop []string          `json:"envDrop,omitempty"` // prefixes removed from the inherited environment
}

// LaunchSpec: everything resolved from profile + model + system-scope modifiers.
type LaunchSpec struct {
	SessionID       string            // provider session id chosen by UNCLI for a new conversation
	ResumeID        string            // provider session id to resume; empty = new
	Model           string            //
	Effort          string            // low | medium | high | xhigh | max; empty = CLI default
	Workdir         string            //
	SystemPrompt    string            // replace (chat profile)
	AppendPrompt    string            // append (code, co-work)
	Tools           []string          // tools that exist at all; empty = CLI default set
	AllowedTools    []string          // tools pre-approved without a prompt
	DisallowedTools []string          //
	PermissionMode  string            // default | acceptEdits | dontAsk | plan; never bypass
	Approvals       bool              // phase 2: route prompts to UNCLI; false = deny automatically
	Isolated        bool              // ignore user settings, MCP servers and skills (chat)
	Env             map[string]string //
}

type UserTurn struct {
	Text        string       // as typed
	Directives  string       // rendered turn-scope modifiers, hidden in the UI
	Attachments []Attachment // images, phase 3
}

type Attachment struct {
	MediaType string `json:"mediaType"`
	Data      []byte `json:"data"`
}

type ControlKind string

const (
	CtlInitialize ControlKind = "initialize"
	CtlInterrupt  ControlKind = "interrupt"
	CtlSetModel   ControlKind = "set_model"
	CtlApprove    ControlKind = "approve" // answer to EvApprovalAsked (phase 2)
)

type Control struct {
	Kind         ControlKind
	RequestID    string          // for CtlApprove: the request being answered
	Model        string          // CtlSetModel
	Allow        bool            // CtlApprove
	Message      string          // CtlApprove deny reason
	UpdatedInput json.RawMessage // CtlApprove allow
}

type AuthInfo struct {
	LoggedIn     bool   `json:"loggedIn"`
	Method       string `json:"method,omitempty"`
	Email        string `json:"email,omitempty"`
	Subscription string `json:"subscription,omitempty"`
}

// Capabilities: the UI hides or greys out controls the adapter can't back.
type Capabilities struct {
	PartialStreaming bool `json:"partialStreaming"` // token deltas
	Resume           bool `json:"resume"`
	LiveModelSwitch  bool `json:"liveModelSwitch"` // control message instead of respawn
	Interrupt        bool `json:"interrupt"`
	Approvals        bool `json:"approvals"` // permission prompt routing
	Images           bool `json:"images"`
	UsageReporting   bool `json:"usageReporting"`
	ThinkingEvents   bool `json:"thinkingEvents"`
	SlashPassthrough bool `json:"slashPassthrough"`
}

// TextTasker is an adapter that can run one-off text tasks (page summaries,
// commit messages) outside any session: no tools, no history, a single
// answer. The user picks one provider and model for all such tasks.
type TextTasker interface {
	TextTaskCommand(bin string, t TextTask) Command // the prompt goes on stdin
	ParseTextTask(out []byte) (TextResult, error)
}

type TextTask struct {
	Model  string
	System string
	Prompt string
	Schema json.RawMessage // optional JSON Schema for a structured answer
}

type TextResult struct {
	Text       string          `json:"text"`
	Structured json.RawMessage `json:"structured,omitempty"`
	Usage      Usage           `json:"usage"`
	CostUSD    float64         `json:"costUsd"`
}
