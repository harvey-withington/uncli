package claude

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"uncli/internal/core"
)

// maxToolOutput caps tool output kept in events; the trace strip only
// needs enough to show what happened.
const maxToolOutput = 2000

// parser turns Claude Code stream-json lines into normalized events. It is
// stateful: system/init repeats every turn and is reported only on change,
// and a permission denial and the tool result that follows it are reported once.
// A file a tool writes is reported once the tool has succeeded, not when
// it is asked for, so a denied or failed write isn't a touched file.
type parser struct {
	sid, model  string
	denied      map[string]bool
	touching    map[string]core.FileTouched // tool use id -> the file it will write
	errorCode   string                      // from a synthetic assistant message, folded into the result
	interrupted bool
}

func newParser() *parser {
	return &parser{denied: map[string]bool{}, touching: map[string]core.FileTouched{}}
}

type line struct {
	Type            string  `json:"type"`
	Subtype         string  `json:"subtype"`
	SessionID       string  `json:"session_id"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	Error           string  `json:"error"`

	// system/init
	Model          string   `json:"model"`
	Tools          []string `json:"tools"`
	PermissionMode string   `json:"permissionMode"`
	Version        string   `json:"claude_code_version"`

	// system/status
	Status *string `json:"status"`

	// system/thinking_tokens
	EstimatedTokens int `json:"estimated_tokens"`

	// system/permission_denied
	ToolUseID string          `json:"tool_use_id"`
	Message   json.RawMessage `json:"message"`

	// stream_event
	Event *struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
		Delta *struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`

	// result
	IsError        bool              `json:"is_error"`
	Result         string            `json:"result"`
	TotalCostUSD   float64           `json:"total_cost_usd"`
	DurationMS     int               `json:"duration_ms"`
	Usage          *resultUsage      `json:"usage"`
	Denials        []json.RawMessage `json:"permission_denials"`
	TerminalReason string            `json:"terminal_reason"`

	// rate_limit_event
	RateLimitInfo *struct {
		Status         string `json:"status"`
		UnifiedWindows map[string]struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    int64   `json:"resetsAt"`
		} `json:"unifiedWindows"`
	} `json:"rate_limit_info"`

	// control_request / control_response
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
	Response  json.RawMessage `json:"response"`

	IsSynthetic bool `json:"isSynthetic"`

	// user: what a tool reported beyond its text (an edit's patch)
	ToolUseResult json.RawMessage `json:"tool_use_result"`
}

type resultUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read_input_tokens"`
	CacheWrite   int `json:"cache_creation_input_tokens"`
}

type message struct {
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

func (p *parser) Feed(raw []byte) ([]core.Event, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	raw = bytes.Clone(raw)
	var l line
	if err := json.Unmarshal(raw, &l); err != nil {
		return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
	}
	ev := func(kind core.EventKind, payload any) core.Event { return core.NewEvent(kind, payload, raw) }
	unknown := func() []core.Event {
		t := l.Type
		if l.Subtype != "" {
			t += "/" + l.Subtype
		}
		return []core.Event{ev(core.EvUnknown, core.Unknown{Type: t})}
	}

	// Subagent traffic is out of scope for phase 1; keep it in the log only.
	if l.ParentToolUseID != nil && *l.ParentToolUseID != "" && (l.Type == "assistant" || l.Type == "user" || l.Type == "stream_event") {
		return unknown(), nil
	}

	switch l.Type {
	case "system":
		switch l.Subtype {
		case "init":
			if l.SessionID == p.sid && l.Model == p.model {
				return nil, nil
			}
			p.sid, p.model = l.SessionID, l.Model
			return []core.Event{ev(core.EvSessionReady, core.SessionReady{
				ProviderSID: l.SessionID, Model: l.Model, CLIVersion: l.Version,
				Tools: l.Tools, PermissionMode: l.PermissionMode,
			})}, nil
		case "status":
			switch {
			case l.Status != nil && *l.Status == "requesting": // repeats per API call within a turn
				return []core.Event{ev(core.EvTurnStarted, nil)}, nil
			case l.Status != nil && *l.Status == "compacting":
				return []core.Event{ev(core.EvNotice, core.Notice{Kind: core.NoticeCompacting})}, nil
			}
			return nil, nil
		case "thinking_tokens":
			return []core.Event{ev(core.EvThinking, core.Thinking{EstimatedTokens: l.EstimatedTokens})}, nil
		case "compact_boundary":
			return []core.Event{ev(core.EvNotice, core.Notice{Kind: core.NoticeCompacted})}, nil
		case "permission_denied":
			p.denied[l.ToolUseID] = true
			delete(p.touching, l.ToolUseID)
			return []core.Event{ev(core.EvToolFinished, core.ToolFinished{
				ID: l.ToolUseID, Denied: true, Output: truncate(rawString(l.Message)),
			})}, nil
		}
		return unknown(), nil

	case "stream_event":
		if l.Event != nil && l.Event.Type == "content_block_delta" && l.Event.Delta != nil && l.Event.Delta.Type == "text_delta" {
			return []core.Event{ev(core.EvTextDelta, core.TextDelta{Index: l.Event.Index, Text: l.Event.Delta.Text})}, nil
		}
		return nil, nil

	case "assistant":
		var m message
		_ = json.Unmarshal(fieldOf(raw, "message"), &m)
		if l.Error != "" {
			p.errorCode = l.Error
		}
		synthetic := m.Model == "<synthetic>"
		var out []core.Event
		for _, b := range blocks(m.Content) {
			switch b.Type {
			case "text":
				out = append(out, ev(core.EvTextBlock, core.TextBlock{Text: b.Text, Synthetic: synthetic}))
			case "tool_use":
				out = append(out, ev(core.EvToolStarted, core.ToolStarted{
					ID: b.ID, Name: b.Name, Input: b.Input, Summary: toolSummary(b.Name, b.Input),
				}))
				if ft, ok := fileTouched(b.Name, b.Input); ok {
					p.touching[b.ID] = ft
				}
			}
		}
		return out, nil

	case "user":
		var m message
		_ = json.Unmarshal(fieldOf(raw, "message"), &m)
		if l.IsSynthetic { // compaction summary and similar: the CLI talking to itself
			return nil, nil
		}
		var s string
		if json.Unmarshal(m.Content, &s) == nil {
			return p.userText(s, ev), nil
		}
		var out []core.Event
		bs := blocks(m.Content)
		for _, b := range bs {
			switch b.Type {
			case "tool_result":
				ft, touches := p.touching[b.ToolUseID]
				delete(p.touching, b.ToolUseID)
				if p.denied[b.ToolUseID] {
					delete(p.denied, b.ToolUseID)
					continue
				}
				out = append(out, ev(core.EvToolFinished, core.ToolFinished{
					ID: b.ToolUseID, OK: !b.IsError, Output: truncate(resultText(b.Content)),
				}))
				if touches && !b.IsError {
					if len(bs) == 1 { // the line's tool_use_result is this tool's
						ft.Line, ft.Added, ft.Removed = patchStats(l.ToolUseResult)
					}
					out = append(out, ev(core.EvFileTouched, ft))
				}
			case "text":
				out = append(out, p.userText(b.Text, ev)...)
			}
		}
		return out, nil

	case "result":
		r := core.TurnResult{
			IsError: l.IsError, ErrorCode: p.errorCode, Text: l.Result,
			TotalCost: l.TotalCostUSD, CostIsTotal: true, DurationMS: l.DurationMS,
			Denials:     len(l.Denials),
			Interrupted: p.interrupted || l.TerminalReason == "aborted_streaming" || l.TerminalReason == "aborted_tools",
		}
		if l.Usage != nil {
			r.Usage = core.Usage{InputTokens: l.Usage.InputTokens, OutputTokens: l.Usage.OutputTokens,
				CacheRead: l.Usage.CacheRead, CacheWrite: l.Usage.CacheWrite}
		}
		if r.Interrupted {
			r.IsError = false
		}
		p.errorCode, p.interrupted = "", false
		return []core.Event{ev(core.EvTurnResult, r)}, nil

	case "rate_limit_event":
		if l.RateLimitInfo == nil {
			return unknown(), nil
		}
		u := core.UsageLimit{Status: l.RateLimitInfo.Status, Windows: map[string]core.UsageWindow{}}
		for k, w := range l.RateLimitInfo.UnifiedWindows {
			u.Windows[k] = core.UsageWindow{Utilization: w.Utilization, ResetsAt: w.ResetsAt}
		}
		return []core.Event{ev(core.EvUsageLimit, u)}, nil

	case "conversation_reset":
		p.sid = ""
		return []core.Event{ev(core.EvNotice, core.Notice{Kind: core.NoticeConversationReset})}, nil

	case "control_request":
		var req struct {
			Subtype     string          `json:"subtype"`
			ToolName    string          `json:"tool_name"`
			Input       json.RawMessage `json:"input"`
			Description string          `json:"description"`
			ToolUseID   string          `json:"tool_use_id"`
		}
		_ = json.Unmarshal(l.Request, &req)
		if req.Subtype == "can_use_tool" {
			return []core.Event{ev(core.EvApprovalAsked, core.ApprovalAsked{
				RequestID: l.RequestID, Tool: req.ToolName, Input: req.Input, Description: req.Description, ToolUseID: req.ToolUseID,
				Action: ActionOf(req.ToolName, req.Input),
			})}, nil
		}
		return unknown(), nil

	case "control_response":
		var resp struct {
			Subtype  string `json:"subtype"`
			Error    string `json:"error"`
			Response *struct {
				Models  []initModel `json:"models"`
				Account *struct {
					SubscriptionType string `json:"subscriptionType"`
				} `json:"account"`
				MCPServers []mcpServer `json:"mcpServers"`
			} `json:"response"`
		}
		_ = json.Unmarshal(l.Response, &resp)
		if resp.Subtype == "error" {
			return []core.Event{ev(core.EvError, core.ErrorInfo{Message: resp.Error})}, nil
		}
		if resp.Response != nil && resp.Response.MCPServers != nil {
			return []core.Event{ev(core.EvToolHints, toolHints(resp.Response.MCPServers))}, nil
		}
		if resp.Response != nil && len(resp.Response.Models) > 0 {
			a := core.Account{}
			for _, m := range resp.Response.Models {
				a.Models = append(a.Models, core.ModelInfo{Value: m.Value, Resolved: m.ResolvedModel,
					DisplayName: m.DisplayName, Description: m.Description, EffortLevels: m.SupportedEffortLevels})
			}
			if resp.Response.Account != nil {
				a.Subscription = resp.Response.Account.SubscriptionType
			}
			return []core.Event{ev(core.EvAccount, a)}, nil
		}
		return nil, nil
	}
	return unknown(), nil
}

type initModel struct {
	Value                 string   `json:"value"`
	ResolvedModel         string   `json:"resolvedModel"`
	DisplayName           string   `json:"displayName"`
	Description           string   `json:"description"`
	SupportedEffortLevels []string `json:"supportedEffortLevels"`
}

// mcpServer is one server in the answer to mcp_status. The CLI reports
// each tool's annotations without the "Hint" suffix and leaves out the
// false ones.
type mcpServer struct {
	Name  string `json:"name"`
	Tools []struct {
		Name        string `json:"name"`
		Annotations struct {
			ReadOnly    bool `json:"readOnly"`
			Destructive bool `json:"destructive"`
			OpenWorld   bool `json:"openWorld"`
		} `json:"annotations"`
	} `json:"tools"`
}

// mcpUnsafe is what the CLI replaces in a server or tool name to build
// the tool's name ("mcp__<server>__<tool>").
var mcpUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func toolHints(servers []mcpServer) core.ToolHints {
	out := core.ToolHints{Tools: map[string]core.ToolHint{}}
	for _, s := range servers {
		for _, t := range s.Tools {
			name := "mcp__" + mcpUnsafe.ReplaceAllString(s.Name, "_") + "__" + mcpUnsafe.ReplaceAllString(t.Name, "_")
			a := t.Annotations
			out.Tools[name] = core.ToolHint{ReadOnly: a.ReadOnly, Destructive: a.Destructive, OpenWorld: a.OpenWorld}
		}
	}
	return out
}

var localStdout = regexp.MustCompile(`(?s)^<local-command-stdout>(.*)</local-command-stdout>$`)

func (p *parser) userText(s string, ev func(core.EventKind, any) core.Event) []core.Event {
	s = strings.TrimSpace(s)
	if m := localStdout.FindStringSubmatch(s); m != nil {
		text := strings.TrimSpace(m[1])
		kind := core.NoticeCommandOutput
		if strings.HasPrefix(text, "Set model to") {
			kind = core.NoticeModelChanged
		}
		return []core.Event{ev(core.EvNotice, core.Notice{Kind: kind, Text: text})}
	}
	if strings.HasPrefix(s, "[Request interrupted by user") {
		p.interrupted = true
		return []core.Event{ev(core.EvNotice, core.Notice{Kind: core.NoticeInterrupted})}
	}
	return nil
}

func fieldOf(raw []byte, name string) json.RawMessage {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	return m[name]
}

func blocks(content json.RawMessage) []block {
	var bs []block
	_ = json.Unmarshal(content, &bs)
	return bs
}

func resultText(content json.RawMessage) string {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	var parts []string
	for _, b := range blocks(content) {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func rawString(r json.RawMessage) string {
	var s string
	if json.Unmarshal(r, &s) == nil {
		return s
	}
	return string(r)
}

func truncate(s string) string {
	if len(s) <= maxToolOutput {
		return s
	}
	cut := maxToolOutput
	for cut > 0 && s[cut-1]&0xC0 == 0x80 { // don't split a UTF-8 sequence
		cut--
	}
	if cut > 0 && s[cut-1] >= 0xC0 {
		cut--
	}
	return s[:cut] + "…"
}

type toolInput struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
	Command      string `json:"command"`
	Pattern      string `json:"pattern"`
	URL          string `json:"url"`
	Query        string `json:"query"`
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
}

func toolSummary(name string, input json.RawMessage) string {
	var in toolInput
	_ = json.Unmarshal(input, &in)
	arg := ""
	switch {
	case in.Command != "":
		arg = in.Command
	case in.FilePath != "":
		arg = filepath.Base(in.FilePath)
	case in.NotebookPath != "":
		arg = filepath.Base(in.NotebookPath)
	case in.Pattern != "":
		arg = in.Pattern
	case in.URL != "":
		arg = in.URL
	case in.Query != "":
		arg = in.Query
	case in.Description != "":
		arg = in.Description
	}
	arg = strings.Join(strings.Fields(arg), " ")
	if r := []rune(arg); len(r) > 80 {
		arg = string(r[:79]) + "…"
	}
	if arg == "" {
		return name
	}
	return name + " " + arg
}

// patchStats reads what a tool that wrote a file reports: the first
// changed line in the new file, and the lines added and removed, from its
// structuredPatch (a hunk starts at newStart and leads with context lines,
// " ", then the change, "-" / "+"). A file the tool created has no patch:
// every line of its content is added. Zeros when it says neither.
func patchStats(result json.RawMessage) (line, added, removed int) {
	var r struct {
		Type            string `json:"type"`
		Content         string `json:"content"`
		StructuredPatch []struct {
			NewStart int      `json:"newStart"`
			Lines    []string `json:"lines"`
		} `json:"structuredPatch"`
	}
	if json.Unmarshal(result, &r) != nil {
		return 0, 0, 0
	}
	if len(r.StructuredPatch) == 0 {
		if r.Type == "create" {
			return 0, lineCount(r.Content), 0
		}
		return 0, 0, 0
	}
	for i, h := range r.StructuredPatch {
		at := h.NewStart
		for _, l := range h.Lines {
			switch {
			case strings.HasPrefix(l, "+"):
				added++
			case strings.HasPrefix(l, "-"):
				removed++
			default:
				at++
				continue
			}
			if i == 0 && line == 0 {
				line = at
			}
		}
	}
	if line == 0 {
		line = r.StructuredPatch[0].NewStart
	}
	return line, added, removed
}

// lineCount counts the lines in a file's content.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func fileTouched(name string, input json.RawMessage) (core.FileTouched, bool) {
	var in toolInput
	_ = json.Unmarshal(input, &in)
	path := in.FilePath
	if path == "" {
		path = in.NotebookPath
	}
	if path == "" {
		return core.FileTouched{}, false
	}
	switch name {
	case "Write":
		return core.FileTouched{Path: path, How: "write"}, true
	case "Edit", "MultiEdit", "NotebookEdit":
		return core.FileTouched{Path: path, How: "edit"}, true
	}
	return core.FileTouched{}, false
}
