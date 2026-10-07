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
	MCPConfig       []string          // MCP server config files to load, besides the user's own
	Env             map[string]string //
}

type UserTurn struct {
	Text        string       // as typed
	Directives  string       // rendered turn-scope modifiers, hidden in the UI
	Attachments []Attachment // files sent with the turn (Images / Documents capabilities)
}

// Attachment is a file sent with a turn. The adapter picks the provider's
// form from the media type: image/* (Images), application/pdf or
// text/plain (Documents).
type Attachment struct {
	Name      string `json:"name"`           // shown to the model as the document title
	Path      string `json:"path,omitempty"` // where it came from; empty for pasted data
	MediaType string `json:"mediaType"`
	Data      []byte `json:"data"`
}

type ControlKind string

const (
	CtlInitialize ControlKind = "initialize"
	CtlInterrupt  ControlKind = "interrupt"
	CtlSetModel   ControlKind = "set_model"
	CtlApprove    ControlKind = "approve" // answer to EvApprovalAsked (phase 2)
	// CtlSetPermissionMode changes the CLI's own permission mode on the
	// running process (LivePermissionMode).
	CtlSetPermissionMode ControlKind = "set_permission_mode"
	// CtlToolHints asks for the MCP tools' annotations; the answer comes
	// back as EvToolHints (ToolHints).
	CtlToolHints ControlKind = "tool_hints"
)

type Control struct {
	Kind         ControlKind
	RequestID    string          // for CtlApprove: the request being answered
	Model        string          // CtlSetModel
	Mode         string          // CtlSetPermissionMode: default | acceptEdits | dontAsk | plan; never bypass
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
	// LivePermissionMode: the CLI's permission mode can change without a
	// respawn (CtlSetPermissionMode).
	LivePermissionMode bool `json:"livePermissionMode"`
	// ToolHints: the CLI reports its MCP tools' annotations (CtlToolHints).
	ToolHints        bool `json:"toolHints"`
	Images           bool `json:"images"`
	Documents        bool `json:"documents"` // PDF and text attachments
	UsageReporting   bool `json:"usageReporting"`
	ThinkingEvents   bool `json:"thinkingEvents"`
	SlashPassthrough bool `json:"slashPassthrough"`
	// Import: the adapter is a TranscriptReader (saved conversations can be imported).
	Import bool `json:"import"`
}

// StatePather is an adapter whose CLI keeps files of its own outside the
// session folder (plans, memory, todo lists): writing there is the CLI's
// bookkeeping, not a change to the user's files. Optional.
type StatePather interface {
	StatePaths() []string // absolute folders
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

// TranscriptReader is an adapter that can list the conversations its CLI
// has saved and turn one into UNCLI's terms, so a conversation from the
// terminal (or a deleted UNCLI session) can be opened and continued.
// Optional; adapters that have it set the Import capability. The format is
// the CLI's own and undocumented, so a reader skips what it doesn't know.
type TranscriptReader interface {
	ListTranscripts(loc TranscriptLocation) ([]TranscriptInfo, error)
	ReadTranscript(loc TranscriptLocation, id string) (Transcript, error)
}

// TranscriptLocation says where to look: the runtime knows (the user's
// home locally, a volume in a container), so the reader never assumes.
type TranscriptLocation struct {
	Home      string // the home folder the CLI keeps its state under
	ConfigDir string // the CLI's own state folder, when set apart from Home (Claude: CLAUDE_CONFIG_DIR)
}

// TranscriptInfo describes a saved conversation without reading it all.
type TranscriptInfo struct {
	ID            string `json:"id"`      // the provider's session id: what --resume takes
	Workdir       string `json:"workdir"` // the folder it ran in
	Started       int64  `json:"started"` // ms
	Updated       int64  `json:"updated"` // ms
	FirstQuestion string `json:"firstQuestion"`
	Turns         int    `json:"turns"`
	CLIVersion    string `json:"cliVersion,omitempty"`
}

// Transcript is a saved conversation as UNCLI pages it.
type Transcript struct {
	Info  TranscriptInfo   `json:"info"`
	Turns []TranscriptTurn `json:"turns"`
}

// TranscriptTurn is one question and what came of it.
type TranscriptTurn struct {
	Question   string           `json:"question"`
	Answer     string           `json:"answer"`
	Model      string           `json:"model,omitempty"` // as the provider names it (a full model id)
	Usage      Usage            `json:"usage"`
	StartedAt  int64            `json:"startedAt"`
	FinishedAt int64            `json:"finishedAt"`
	Tools      []TranscriptTool `json:"tools"`
	Files      []FileTouched    `json:"files"`
	Command    bool             `json:"command,omitempty"` // a slash command (/compact), not a question
	// Origin "cli": the CLI started the turn by itself (a background task
	// finished), so it has no question.
	Origin string `json:"origin,omitempty"`
	Error  bool   `json:"error,omitempty"` // the turn ended in an error
}

// TranscriptTool is a tool use in a turn.
type TranscriptTool struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Done    bool   `json:"done"`
	OK      bool   `json:"ok"`
	Output  string `json:"output,omitempty"`
}
