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
	EvFileTouched   EventKind = "file_touched"   // path, line, how (edit/write/bash-detected)
	EvApprovalAsked EventKind = "approval_asked" // request id, tool, input (phase 2)
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
	How  string `json:"how"` // edit | write | bash-detected
}

type ApprovalAsked struct {
	RequestID   string          `json:"requestId"`
	Tool        string          `json:"tool"`
	Input       json.RawMessage `json:"input,omitempty"`
	Description string          `json:"description,omitempty"`
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
