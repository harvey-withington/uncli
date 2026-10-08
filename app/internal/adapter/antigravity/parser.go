package antigravity

import (
	"encoding/json"
	"fmt"
	"strings"

	"uncli/internal/core"
)

// The stream (CLI 1.3.1, testdata/streams/antigravity/1.3.1):
//
//	{"event":"init","conversation_id":…,"init":{"model":…,"cwd":…,"tools":[…],"permission_mode":…}}
//	{"event":"step_update","step_update":{"step_index":n,"state":"ACTIVE|DONE|ERROR","step_type":…,…}}
//	{"event":"result","result":{"status":"SUCCESS|ERROR|…","response":…,"usage":{…},…}}
//
// init comes once per process; each turn is a run of step updates and one
// result. A step is user_input, agent_response (text_delta while ACTIVE,
// its own usage when DONE), tool (tool_info: name, parameters, output,
// error), subagent or system_message. The result's usage, duration and
// turn count are totals for the process.

type line struct {
	Event          string          `json:"event"`
	ConversationID string          `json:"conversation_id"`
	Init           *initInfo       `json:"init"`
	Step           *step           `json:"step_update"`
	Result         *result         `json:"result"`
	Raw            json.RawMessage `json:"-"`
}

type initInfo struct {
	Model          string   `json:"model"`
	Cwd            string   `json:"cwd"`
	Tools          []string `json:"tools"`
	PermissionMode string   `json:"permission_mode"`
}

type usage struct {
	Input     int `json:"input_tokens"`
	Output    int `json:"output_tokens"`
	Thinking  int `json:"thinking_tokens"`
	CacheRead int `json:"cache_read_tokens"`
}

type step struct {
	ConversationID string   `json:"conversation_id"`
	Index          int      `json:"step_index"`
	State          string   `json:"state"`
	Type           string   `json:"step_type"`
	ToolName       string   `json:"tool_name"`
	TextDelta      string   `json:"text_delta"`
	Usage          *usage   `json:"usage"`
	Tool           *tool    `json:"tool_info"`
	Duration       float64  `json:"duration_seconds"`
	Subagents      *subinfo `json:"subagent_info"`
}

type tool struct {
	Name   string          `json:"name"`
	Params json.RawMessage `json:"parameters"`
	Output string          `json:"output"`
	Error  *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type subinfo struct {
	Subagents []struct {
		Role string `json:"role"`
	} `json:"subagents"`
}

type result struct {
	Status   string  `json:"status"`
	Response string  `json:"response"`
	Error    string  `json:"error"`
	Duration float64 `json:"duration_seconds"`
	Usage    usage   `json:"usage"`
	Denied   []struct {
		Action string `json:"action"`
	} `json:"denied_actions"`
}

type parser struct {
	conv     string
	text     map[int]*strings.Builder // answer text per step, until the step is done
	started  map[int]bool             // tool steps reported as started
	turn     usage                    // this turn's usage, summed from its steps
	duration float64                  // the process's total so far, at the last result
}

func newParser() *parser {
	return &parser{text: map[int]*strings.Builder{}, started: map[int]bool{}}
}

// ToolID is a tool step's id in UNCLI's trace: the conversation and the
// step's index, which the hook's call carries too (stepIdx).
func ToolID(conversation string, step int) string { return fmt.Sprintf("%s:%d", conversation, step) }

func (p *parser) Feed(raw []byte) ([]core.Event, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, nil
	}
	var l line
	if err := json.Unmarshal(raw, &l); err != nil || l.Event == "" {
		return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
	}
	switch {
	case l.Event == "init" && l.Init != nil:
		p.conv = l.ConversationID
		return []core.Event{core.NewEvent(core.EvSessionReady, core.SessionReady{
			ProviderSID: l.ConversationID, Model: l.Init.Model, Tools: l.Init.Tools, PermissionMode: l.Init.PermissionMode,
		}, raw)}, nil
	case l.Event == "step_update" && l.Step != nil:
		return p.step(l.Step, raw), nil
	case l.Event == "result" && l.Result != nil:
		return p.result(l.Result, raw), nil
	}
	return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{Type: l.Event}, raw)}, nil
}

func (p *parser) step(s *step, raw []byte) []core.Event {
	if s.ConversationID != "" && p.conv == "" {
		p.conv = s.ConversationID
	}
	id := ToolID(p.conv, s.Index)
	switch s.Type {
	case "user_input":
		if s.State == "DONE" {
			p.turn = usage{}
			return []core.Event{core.NewEvent(core.EvTurnStarted, nil, raw)}
		}
	case "agent_response":
		var out []core.Event
		if s.TextDelta != "" {
			b := p.text[s.Index]
			if b == nil {
				b = &strings.Builder{}
				p.text[s.Index] = b
			}
			b.WriteString(s.TextDelta)
			out = append(out, core.NewEvent(core.EvTextDelta, core.TextDelta{Index: s.Index, Text: s.TextDelta}, raw))
		}
		if s.Usage != nil {
			p.turn.Input += s.Usage.Input
			p.turn.Output += s.Usage.Output
			p.turn.Thinking += s.Usage.Thinking
			p.turn.CacheRead += s.Usage.CacheRead
		}
		if s.State != "ACTIVE" {
			if b := p.text[s.Index]; b != nil {
				if t := strings.TrimSpace(b.String()); t != "" {
					out = append(out, core.NewEvent(core.EvTextBlock, core.TextBlock{Text: t}, nil))
				}
				delete(p.text, s.Index)
			}
		}
		return out
	case "tool", "subagent":
		var out []core.Event
		name := s.ToolName
		var params json.RawMessage
		if s.Tool != nil {
			if s.Tool.Name != "" {
				name = s.Tool.Name
			}
			params = s.Tool.Params
		}
		if !p.started[s.Index] {
			p.started[s.Index] = true
			out = append(out, core.NewEvent(core.EvToolStarted, core.ToolStarted{ID: id, Name: name, Input: params, Summary: summary(name, params)}, raw))
		}
		if s.State == "ACTIVE" {
			return out
		}
		delete(p.started, s.Index)
		fin := core.ToolFinished{ID: id, OK: s.State == "DONE"}
		if s.Tool != nil {
			fin.Output = truncate(s.Tool.Output)
			if e := s.Tool.Error; e != nil {
				fin.OK = false
				fin.Output = truncate(e.Message)
				fin.Denied = strings.Contains(e.Message, "denied by pre-tool hook")
			}
		}
		out = append(out, core.NewEvent(core.EvToolFinished, fin, raw))
		if fin.OK {
			if f, ok := touched(name, params); ok {
				out = append(out, core.NewEvent(core.EvFileTouched, f, nil))
			}
		}
		return out
	}
	return nil
}

func (p *parser) result(r *result, raw []byte) []core.Event {
	tr := core.TurnResult{
		IsError: r.Status != "SUCCESS",
		Text:    strings.TrimSpace(r.Response),
		Usage: core.Usage{
			InputTokens:  p.turn.Input,
			OutputTokens: p.turn.Output + p.turn.Thinking, // thinking is billed as output
			CacheRead:    p.turn.CacheRead,
		},
		DurationMS:  int((r.Duration - p.duration) * 1000),
		Interrupted: r.Status == "CANCELED" || r.Status == "INTERRUPTED",
		Denials:     len(r.Denied),
	}
	p.duration = r.Duration
	p.turn = usage{}
	var out []core.Event
	if tr.IsError {
		tr.ErrorCode = strings.ToLower(r.Status)
		if r.Error != "" {
			tr.Text = firstLine(r.Error)
			out = append(out, core.NewEvent(core.EvError, core.ErrorInfo{Message: firstLine(r.Error)}, nil))
		}
	}
	return append(out, core.NewEvent(core.EvTurnResult, tr, raw))
}

// touched is the file a successful write or edit changed.
func touched(tool string, params json.RawMessage) (core.FileTouched, bool) {
	how := map[string]string{"write_to_file": "write", "replace_file_content": "edit", "multi_replace_file_content": "edit", "sed_file": "edit"}[tool]
	if how == "" {
		return core.FileTouched{}, false
	}
	a := ActionOf(tool, params)
	if a.Path == "" {
		return core.FileTouched{}, false
	}
	return core.FileTouched{Path: a.Path, How: how}, true
}

func truncate(s string) string {
	const max = 4000
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
