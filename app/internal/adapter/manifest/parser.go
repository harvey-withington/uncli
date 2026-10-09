package manifest

import (
	"encoding/json"
	"runtime"
	"strings"

	"uncli/internal/core"
)

// parser applies the manifest's event rules to each output line. Its
// state is what rules carry across lines: the session's id, text gathered
// per block, the tools started, and the turn's usage.
type parser struct {
	a        *Adapter
	session  string
	blocks   map[string]*strings.Builder
	started  map[string]bool
	turn     core.Usage
	duration float64 // the process's running total at the last result
	cost     float64
}

func newParser(a *Adapter) *parser {
	return &parser{a: a, blocks: map[string]*strings.Builder{}, started: map[string]bool{}}
}

func (p *parser) Feed(raw []byte) ([]core.Event, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, nil
	}
	var line any
	if err := json.Unmarshal(raw, &line); err != nil {
		return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
	}
	if _, ok := line.(map[string]any); !ok {
		return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
	}
	var out []core.Event
	known := false
	for _, r := range p.a.M.Events {
		if !matches(line, r.When) {
			continue
		}
		known = true
		out = append(out, p.apply(r, line, raw)...)
	}
	if !known {
		return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
	}
	return out, nil
}

func (p *parser) apply(r Rule, line any, raw []byte) []core.Event {
	var out []core.Event
	if s := r.Session; s != nil {
		if id := str(line, s.ID); id != "" {
			p.session = id
		}
		var tools []string
		if l, ok := get(line, s.Tools).([]any); ok {
			for _, t := range l {
				if ts, ok := t.(string); ok {
					tools = append(tools, ts)
				}
			}
		}
		out = append(out, core.NewEvent(core.EvSessionReady, core.SessionReady{
			ProviderSID: p.session, Model: str(line, s.Model), Tools: tools, PermissionMode: str(line, s.PermissionMode),
		}, raw))
	}
	if r.TurnStarted {
		p.turn = core.Usage{}
		out = append(out, core.NewEvent(core.EvTurnStarted, nil, raw))
	}
	if t := r.Text; t != nil {
		out = append(out, p.text(t, line, raw)...)
	}
	if u := r.Usage; u != nil {
		p.turn.InputTokens += sum(line, u.Input)
		p.turn.OutputTokens += sum(line, u.Output)
		p.turn.CacheRead += sum(line, u.CacheRead)
		p.turn.CacheWrite += sum(line, u.CacheWrite)
	}
	if t := r.Tool; t != nil {
		out = append(out, p.tool(t, line, raw)...)
	}
	if res := r.Result; res != nil {
		out = append(out, p.result(res, line, raw)...)
	}
	return out
}

func (p *parser) text(t *TextRule, line any, raw []byte) []core.Event {
	var out []core.Event
	if t.Whole != "" {
		if s := strings.TrimSpace(str(line, t.Whole)); s != "" {
			out = append(out, core.NewEvent(core.EvTextBlock, core.TextBlock{Text: s}, nil))
		}
		return out
	}
	key := text(get(line, t.Block))
	if d := str(line, t.Delta); d != "" {
		b := p.blocks[key]
		if b == nil {
			b = &strings.Builder{}
			p.blocks[key] = b
		}
		b.WriteString(d)
		out = append(out, core.NewEvent(core.EvTextDelta, core.TextDelta{Index: int(num(line, t.Block)), Text: d}, raw))
	}
	if matches(line, t.Done) {
		if b := p.blocks[key]; b != nil {
			if s := strings.TrimSpace(b.String()); s != "" {
				out = append(out, core.NewEvent(core.EvTextBlock, core.TextBlock{Text: s}, nil))
			}
			delete(p.blocks, key)
		}
	}
	return out
}

func (p *parser) id(tmpl string, line any) string {
	return expand(tmpl, vars(map[string]string{"session": p.session}, line))
}

func (p *parser) tool(t *ToolRule, line any, raw []byte) []core.Event {
	var out []core.Event
	id := p.id(t.ID, line)
	name := first(line, t.Name)
	var input json.RawMessage
	if v := get(line, t.Input); v != nil {
		input, _ = json.Marshal(v)
	}
	if !p.started[id] {
		p.started[id] = true
		out = append(out, core.NewEvent(core.EvToolStarted, core.ToolStarted{ID: id, Name: name, Input: input, Summary: p.a.summary(name, input)}, raw))
	}
	if !matches(line, t.Done) {
		return out
	}
	delete(p.started, id)
	fin := core.ToolFinished{ID: id, OK: matches(line, t.OK), Output: truncate(str(line, t.Output))}
	if get(line, t.Error) != nil {
		msg := str(line, t.Error)
		fin.OK, fin.Output = false, truncate(msg)
		fin.Denied = t.Denied != "" && strings.Contains(msg, t.Denied)
	}
	out = append(out, core.NewEvent(core.EvToolFinished, fin, raw))
	if fin.OK {
		if how := p.a.M.Tools.Touched[name]; how != "" {
			if path := p.a.ActionOf(name, input).Path; path != "" {
				out = append(out, core.NewEvent(core.EvFileTouched, core.FileTouched{Path: path, How: how}, nil))
			}
		}
	}
	return out
}

func (p *parser) result(r *ResultRule, line any, raw []byte) []core.Event {
	tr := core.TurnResult{
		IsError:     !matches(line, r.OK),
		Text:        strings.TrimSpace(str(line, r.Text)),
		Interrupted: matches(line, r.Interrupted),
	}
	switch u := r.Usage.(type) {
	case string:
		if u == "turn" {
			tr.Usage = p.turn
		}
	case map[string]any:
		paths := func(k string) Paths {
			switch v := u[k].(type) {
			case string:
				return Paths{v}
			case []any:
				var ps Paths
				for _, x := range v {
					if s, ok := x.(string); ok {
						ps = append(ps, s)
					}
				}
				return ps
			}
			return nil
		}
		tr.Usage = core.Usage{InputTokens: sum(line, paths("input")), OutputTokens: sum(line, paths("output")),
			CacheRead: sum(line, paths("cacheRead")), CacheWrite: sum(line, paths("cacheWrite"))}
	}
	var d float64
	switch {
	case r.DurationSeconds != "":
		d = num(line, r.DurationSeconds)
		if r.DurationTotal {
			d, p.duration = d-p.duration, d
		}
		tr.DurationMS = int(d * 1000)
	case r.DurationMS != "":
		d = num(line, r.DurationMS)
		if r.DurationTotal {
			d, p.duration = d-p.duration, d
		}
		tr.DurationMS = int(d)
	}
	if r.Cost != "" {
		tr.TotalCost, tr.CostIsTotal = num(line, r.Cost), r.CostTotal
	}
	switch v := get(line, r.Denials).(type) {
	case []any:
		tr.Denials = len(v)
	case float64:
		tr.Denials = int(v)
	}
	p.turn = core.Usage{}
	var out []core.Event
	if tr.IsError {
		tr.ErrorCode = strings.ToLower(str(line, r.ErrorCode))
		if e := str(line, r.Error); e != "" {
			tr.Text = firstLine(e)
			out = append(out, core.NewEvent(core.EvError, core.ErrorInfo{Message: firstLine(e)}, nil))
		}
	}
	return append(out, core.NewEvent(core.EvTurnResult, tr, raw))
}

// ActionOf says what a tool call does, in UNCLI's terms.
func (a *Adapter) ActionOf(tool string, args json.RawMessage) core.ToolAction {
	t := a.M.Tools
	act := core.ToolAction{Kind: t.Kinds[tool], Tool: tool, Input: args}
	if act.Kind == "" {
		for prefix, k := range t.Prefixes {
			if strings.HasPrefix(tool, prefix) {
				act.Kind = k
			}
		}
	}
	if act.Kind == "" {
		act.Kind = core.ActOther
	}
	var in any
	_ = json.Unmarshal(args, &in)
	switch act.Kind {
	case core.ActShell:
		act.Command = first(in, t.Command)
		act.Dialect = t.Shell.Dialect[runtime.GOOS]
		if act.Dialect == "" {
			act.Dialect = t.Shell.Dialect["other"]
		}
		if act.Dialect == "" {
			act.Dialect = "bash"
		}
	case core.ActRead, core.ActWrite, core.ActEdit, core.ActSearch:
		act.Path = first(in, t.Path)
	case core.ActMCP:
		if s, n := first(in, t.MCP.Server), first(in, t.MCP.Tool); s != "" && n != "" {
			act.Tool = "mcp__" + s + "__" + n
		}
	}
	return act
}

// summary is the trace's one line for a tool call.
func (a *Adapter) summary(tool string, args json.RawMessage) string {
	var in any
	_ = json.Unmarshal(args, &in)
	for _, k := range a.M.Tools.Summary {
		if s := str(in, k); strings.TrimSpace(s) != "" {
			return firstLine(s)
		}
	}
	return tool
}

// ReadHook reads a call from UNCLI's hook.
func (a *Adapter) ReadHook(event string, payload []byte) (core.HookCall, error) {
	h := a.M.Approvals.Hook
	if h == nil {
		return core.HookCall{}, errorf("%s has no hook", a.M.Name)
	}
	e, ok := h.Events[event]
	if !ok {
		return core.HookCall{}, errorf("unknown hook event %q", event)
	}
	if e.Kind == "instructions" {
		return core.HookCall{Kind: core.HookInstructions}, nil
	}
	var call any
	if err := json.Unmarshal(payload, &call); err != nil {
		return core.HookCall{}, errorf("hook call: %v", err)
	}
	name := str(call, e.Name)
	if name == "" {
		return core.HookCall{}, errorf("hook call names no tool")
	}
	var input json.RawMessage
	if v := get(call, e.Input); v != nil {
		input, _ = json.Marshal(v)
	}
	id := expand(e.ID, vars(nil, call))
	return core.HookCall{Kind: core.HookTool, Approval: core.ApprovalAsked{
		RequestID: id, Tool: name, Input: input, Description: a.summary(name, input), ToolUseID: id, Action: a.ActionOf(name, input),
	}}, nil
}

// AnswerHook writes UNCLI's answer as the CLI reads it.
func (a *Adapter) AnswerHook(call core.HookCall, ans core.HookAnswer) ([]byte, error) {
	h := a.M.Approvals.Hook
	if h == nil {
		return nil, errorf("%s has no hook", a.M.Name)
	}
	var kind string
	switch call.Kind {
	case core.HookTool:
		kind = "tool"
	case core.HookInstructions:
		kind = "instructions"
	default:
		return nil, errorf("unknown hook call %q", call.Kind)
	}
	for _, e := range h.Events {
		if e.Kind != kind {
			continue
		}
		var tmpl any
		switch {
		case kind == "tool" && ans.Allow:
			tmpl = e.Allow
		case kind == "tool":
			tmpl = e.Deny
		case ans.Instructions == "":
			tmpl = e.None
			if tmpl == nil {
				tmpl = map[string]any{}
			}
		default:
			tmpl = e.Answer
		}
		reason := ans.Reason
		if reason == "" {
			reason = "Denied in UNCLI."
		}
		return json.Marshal(fill(tmpl, vars(map[string]string{"reason": reason, "instructions": ans.Instructions}, nil)))
	}
	return nil, errorf("%s's hook has no %s event", a.M.Name, kind)
}
