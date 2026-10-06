// Package core holds UNCLI's interfaces and normalized event model. It
// imports nothing else from UNCLI: adapters, runtimes, the session layer
// and the bridge all depend on it, never the other way round.
package core

import (
	"encoding/json"
	"time"
)

type EventKind string

const (
	EvSessionReady  EventKind = "session_ready"  // provider session id, model, tools (also when the id changes)
	EvAccount       EventKind = "account"        // models and account from the provider's handshake
	EvTurnStarted   EventKind = "turn_started"   //
	EvThinking      EventKind = "thinking"       // estimated tokens; text only if the provider exposes it
	EvTextDelta     EventKind = "text_delta"     //
	EvTextBlock     EventKind = "text_block"     // a completed assistant text block
	EvToolStarted   EventKind = "tool_started"   // id, name, input
	EvToolFinished  EventKind = "tool_finished"  // id, ok, denied, output (truncated)
	EvFileTouched   EventKind = "file_touched"   // path, line, how (edit/write), once the tool has succeeded
	EvApprovalAsked EventKind = "approval_asked" // request id, tool, input (phase 2)
	EvToolHints     EventKind = "tool_hints"     // what the MCP servers say about their tools
	EvNotice        EventKind = "notice"         // model changed, compacted, conversation reset, command output
	EvUsageLimit    EventKind = "usage_limit"    // subscription window utilisation and reset times
	EvTurnResult    EventKind = "turn_result"    // usage, cost, duration, is_error, error code
	EvError         EventKind = "error"          //
	EvExited        EventKind = "exited"         // exit code
	EvUnknown       EventKind = "unknown"        // raw line kept, never dropped
)

type Event struct {
	Kind    EventKind       `json:"kind"`
	TurnSeq int             `json:"turnSeq"` // page number within the session; set by the session
	At      time.Time       `json:"at"`
	Data    json.RawMessage `json:"data,omitempty"` // kind-specific payload
	Raw     json.RawMessage `json:"-"`              // original provider line, for the log and fixtures
}

// NewEvent builds an event with its payload marshalled into Data.
func NewEvent(kind EventKind, payload any, raw []byte) Event {
	ev := Event{Kind: kind, At: time.Now(), Raw: raw}
	if payload != nil {
		ev.Data, _ = json.Marshal(payload)
	}
	return ev
}

// Decode unmarshals an event's payload into T.
func Decode[T any](ev Event) (T, error) {
	var v T
	if len(ev.Data) == 0 {
		return v, nil
	}
	err := json.Unmarshal(ev.Data, &v)
	return v, err
}

type SessionReady struct {
	ProviderSID    string   `json:"providerSid"`
	Model          string   `json:"model"`
	CLIVersion     string   `json:"cliVersion,omitempty"`
	Tools          []string `json:"tools,omitempty"`
	PermissionMode string   `json:"permissionMode,omitempty"`
}

type ModelInfo struct {
	Value        string   `json:"value"`        // what to pass to --model or set_model
	Resolved     string   `json:"resolved"`     // full model id
	DisplayName  string   `json:"displayName"`  //
	Description  string   `json:"description"`  //
	EffortLevels []string `json:"effortLevels"` // empty = effort not supported
}

type Account struct {
	Models       []ModelInfo `json:"models"`
	Subscription string      `json:"subscription,omitempty"`
}

type Thinking struct {
	EstimatedTokens int    `json:"estimatedTokens,omitempty"`
	Text            string `json:"text,omitempty"`
}

type TextDelta struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

type TextBlock struct {
	Text      string `json:"text"`
	Synthetic bool   `json:"synthetic,omitempty"` // produced by the CLI itself (slash command output, errors)
}

type ToolStarted struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input,omitempty"`
	Summary string          `json:"summary"` // one line for the trace strip
}

type ToolFinished struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Denied bool   `json:"denied,omitempty"`
	Output string `json:"output,omitempty"` // truncated
}

type FileTouched struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	How  string `json:"how"` // write | edit (UNCLI adds command and deleted for changes it finds itself)
	// Lines the tool added and removed, when it says (its patch, or a new
	// file's content); zero for both when it doesn't.
	Added   int `json:"added,omitempty"`
	Removed int `json:"removed,omitempty"`
}

type ApprovalAsked struct {
	RequestID   string          `json:"requestId"`
	Tool        string          `json:"tool"`
	Input       json.RawMessage `json:"input,omitempty"`
	Description string          `json:"description,omitempty"`
	ToolUseID   string          `json:"toolUseId,omitempty"` // the tool call it is for, as in EvToolStarted
	Action      ToolAction      `json:"action"`              // what it does, in provider-neutral terms
}

// ToolAction is what a tool use does, in terms that don't depend on the
// provider: the adapter maps its CLI's tools onto these kinds, and UNCLI
// decides when to prompt from them alone.
type ToolAction struct {
	Kind    string          `json:"kind"`              // one of the Act… kinds
	Tool    string          `json:"tool"`              // the provider's own name for the tool (an identity, e.g. for the safe list)
	Dialect string          `json:"dialect,omitempty"` // shell: bash | powershell
	Command string          `json:"command,omitempty"` // shell: the command line
	Path    string          `json:"path,omitempty"`    // read, write, edit: the file
	Input   json.RawMessage `json:"input,omitempty"`   // as the provider gave it (for showing it, or for a model to read)
}

// Kinds of tool action.
const (
	ActShell    = "shell"    // runs a command line
	ActRead     = "read"     // reads a file
	ActSearch   = "search"   // searches or lists files
	ActWrite    = "write"    // writes a file
	ActEdit     = "edit"     // edits a file
	ActWeb      = "web"      // searches or fetches from the web
	ActMCP      = "mcp"      // an MCP server's tool (its annotations are in EvToolHints)
	ActQuestion = "question" // asks the user something
	ActPlan     = "plan"     // asks the user to approve a plan
	ActAgent    = "agent"    // starts a sub-agent (its own tool uses are asked about one by one)
	ActInternal = "internal" // the CLI's own bookkeeping (to-do lists, tool search, messages between agents)
	ActOther    = "other"    // anything else
)

// ToolHint is what a tool's server says about it (the MCP tool
// annotations). They are hints, not guarantees: the server may be wrong.
type ToolHint struct {
	ReadOnly    bool `json:"readOnly,omitempty"`    // doesn't change anything
	Destructive bool `json:"destructive,omitempty"` // may delete or overwrite
	OpenWorld   bool `json:"openWorld,omitempty"`   // reaches outside this computer (web, email, other services)
}

// ToolHints maps the CLI's tool names (as in EvApprovalAsked) to their
// hints, for every tool of every connected MCP server.
type ToolHints struct {
	Tools map[string]ToolHint `json:"tools"`
}

// Notice kinds.
const (
	NoticeModelChanged      = "model_changed"
	NoticeCompacting        = "compacting"
	NoticeCompacted         = "compacted"
	NoticeConversationReset = "conversation_reset"
	NoticeCommandOutput     = "command_output"
	NoticeInterrupted       = "interrupted"
)

type Notice struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
}

type UsageWindow struct {
	Utilization float64 `json:"utilization"` // 0..1
	ResetsAt    int64   `json:"resetsAt"`    // unix seconds
}

type UsageLimit struct {
	Status  string                 `json:"status"`
	Windows map[string]UsageWindow `json:"windows"` // e.g. five_hour, seven_day
}

type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	CacheRead    int `json:"cacheRead"`
	CacheWrite   int `json:"cacheWrite"`
}

type TurnResult struct {
	IsError     bool    `json:"isError"`
	ErrorCode   string  `json:"errorCode,omitempty"` // e.g. authentication_failed, model_not_found
	Interrupted bool    `json:"interrupted,omitempty"`
	Text        string  `json:"text,omitempty"`
	Usage       Usage   `json:"usage"`
	TotalCost   float64 `json:"totalCost"` // as reported; may be cumulative for the process
	CostIsTotal bool    `json:"costIsTotal"`
	DurationMS  int     `json:"durationMs"`
	Denials     int     `json:"denials,omitempty"`
}

type ErrorInfo struct {
	Message string `json:"message"`
}

type Exited struct {
	Code   int    `json:"code"`
	Stderr string `json:"stderr,omitempty"` // tail, for diagnosis
}

type Unknown struct {
	Type string `json:"type,omitempty"`
}
