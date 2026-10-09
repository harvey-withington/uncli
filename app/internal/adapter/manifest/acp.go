package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"uncli/internal/core"
)

// The Agent Client Protocol (decision 0014): JSON-RPC 2.0 over stdio, with
// requests both ways. UNCLI is the client:
//
//	initialize → session/new (or session/load, whose replay is ignored)
//	session/prompt per turn, answered when the turn ends (stopReason)
//	session/update notifications: message chunks, tool calls, diffs
//	session/request_permission from the agent → an approval card, answered
//	  with the agent's allow_once or reject_once option
//	session/cancel to stop a turn
//
// UNCLI offers the agent no files or terminal: it uses its own tools, and
// UNCLI's approvals decide what runs. Any other request from the agent is
// answered "method not found", so it never waits on UNCLI.

const acpVersion = 1

type acp struct {
	a        *Adapter
	spec     core.LaunchSpec
	nextID   int
	pending  map[string]string // our requests' ids → their methods
	session  string
	ready    bool // the session exists: prompts can go
	loading  bool // session/load is replaying the conversation
	failed   error
	queued   []core.UserTurn // turns sent before the session was ready
	perms    map[string]acpPerm
	denied   map[string]bool     // tool calls the user refused
	tools    map[string]*acpTool // tool calls seen, by id
	text     strings.Builder
	block    int
	promptAt time.Time
	model    string
}

type acpPerm struct {
	id       json.RawMessage // the agent's request id, answered as it was
	options  []acpOption
	toolCall string
}

type acpOption struct {
	OptionID string `json:"optionId"`
	Kind     string `json:"kind"`
}

type acpTool struct {
	name, kind, title string
	input             json.RawMessage
	locations         []acpLocation
	done              bool
}

type acpLocation struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type rpc struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	} `json:"error"`
}

func newACP(a *Adapter, spec core.LaunchSpec) *acp {
	return &acp{a: a, spec: spec, pending: map[string]string{}, perms: map[string]acpPerm{},
		denied: map[string]bool{}, tools: map[string]*acpTool{}, model: spec.Model}
}

func line(v any) []byte {
	b, _ := json.Marshal(v)
	return append(b, '\n')
}

func (d *acp) request(method string, params any) []byte {
	d.nextID++
	d.pending[strconv.Itoa(d.nextID)] = method
	return line(map[string]any{"jsonrpc": "2.0", "id": d.nextID, "method": method, "params": params})
}

func idKey(id json.RawMessage) string { return strings.Trim(string(id), `"`) }

func (d *acp) Start() [][]byte {
	return [][]byte{d.request("initialize", map[string]any{
		"protocolVersion":    acpVersion,
		"clientCapabilities": map[string]any{"fs": map[string]bool{"readTextFile": false, "writeTextFile": false}, "terminal": false},
		"clientInfo":         map[string]string{"name": "uncli", "version": "1"},
	})}
}

func (d *acp) Feed(raw []byte) ([]core.Event, [][]byte) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, nil
	}
	var m rpc
	if err := json.Unmarshal(raw, &m); err != nil {
		return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
	}
	hasID := len(m.ID) > 0 && string(m.ID) != "null"
	switch {
	case m.Method == "" && hasID:
		return d.response(m, raw)
	case m.Method != "" && hasID:
		return d.agentRequest(m, raw)
	case m.Method != "":
		return d.notification(m, raw), nil
	}
	return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{}, raw)}, nil
}

func (d *acp) response(m rpc, raw []byte) ([]core.Event, [][]byte) {
	key := idKey(m.ID)
	method := d.pending[key]
	delete(d.pending, key)
	if m.Error != nil {
		msg := m.Error.Message
		if s, ok := m.Error.Data.(string); ok && s != "" {
			msg += " (" + s + ")"
		}
		if strings.Contains(strings.ToLower(m.Error.Message), "auth") {
			msg = d.a.M.Name + " isn't signed in: sign in under Settings → AI Providers."
		}
		switch method {
		case "initialize", "session/new", "session/load":
			d.failed = errors.New(msg)
			evs := []core.Event{core.NewEvent(core.EvError, core.ErrorInfo{Message: msg}, raw)}
			if len(d.queued) > 0 {
				d.queued = nil
				evs = append(evs, core.NewEvent(core.EvTurnResult, core.TurnResult{IsError: true, ErrorCode: "not_ready", Text: msg}, nil))
			}
			return evs, nil
		case "session/prompt":
			evs := d.flushText()
			return append(evs, core.NewEvent(core.EvTurnResult, core.TurnResult{IsError: true, Text: msg, DurationMS: d.elapsed()}, raw)), nil
		}
		return nil, nil
	}
	var res any
	_ = json.Unmarshal(m.Result, &res)
	switch method {
	case "initialize":
		var evs []core.Event
		if ms, cur := acpModels(get(res, "_meta.modelState")); len(ms) > 0 {
			evs = append(evs, core.NewEvent(core.EvAccount, core.Account{Models: ms}, nil))
			if d.model == "" {
				d.model = cur
			}
		}
		params := map[string]any{"cwd": d.spec.Workdir, "mcpServers": []any{}}
		if d.spec.ResumeID != "" {
			d.loading = true
			params["sessionId"] = d.spec.ResumeID
			return evs, [][]byte{d.request("session/load", params)}
		}
		return evs, [][]byte{d.request("session/new", params)}
	case "session/new", "session/load":
		if id := str(res, "sessionId"); id != "" {
			d.session = id
		} else if method == "session/load" {
			d.session = d.spec.ResumeID
		}
		d.ready, d.loading = true, false
		var evs []core.Event
		if ms, cur := acpModels(get(res, "models")); len(ms) > 0 {
			evs = append(evs, core.NewEvent(core.EvAccount, core.Account{Models: ms}, nil))
			if d.model == "" {
				d.model = cur
			}
		}
		evs = append(evs, core.NewEvent(core.EvSessionReady, core.SessionReady{ProviderSID: d.session, Model: d.model}, raw))
		var out [][]byte
		for _, t := range d.queued {
			if b, err := d.prompt(t); err == nil {
				out = append(out, b)
			}
		}
		d.queued = nil
		return evs, out
	case "session/prompt":
		stop := str(res, "stopReason")
		evs := d.flushText()
		tr := core.TurnResult{Interrupted: stop == "cancelled", DurationMS: d.elapsed(), Usage: d.usage(res)}
		if u := d.a.M.ACP.Usage; u.Cost != "" {
			scale := u.CostScale
			if scale == 0 {
				scale = 1
			}
			tr.TotalCost = num(res, u.Cost) * scale
		}
		switch stop {
		case "end_turn", "cancelled", "":
		default: // max_tokens, max_turn_requests, refusal
			tr.IsError, tr.ErrorCode, tr.Text = true, stop, "The turn ended: "+strings.ReplaceAll(stop, "_", " ")
		}
		return append(evs, core.NewEvent(core.EvTurnResult, tr, raw)), nil
	}
	return nil, nil
}

// usage is the turn's tokens, where the manifest says the agent puts them.
func (d *acp) usage(res any) core.Usage {
	u := d.a.M.ACP.Usage
	return core.Usage{InputTokens: sum(res, u.Input), OutputTokens: sum(res, u.Output),
		CacheRead: sum(res, u.CacheRead), CacheWrite: sum(res, u.CacheWrite)}
}

func (d *acp) elapsed() int {
	if d.promptAt.IsZero() {
		return 0
	}
	return int(time.Since(d.promptAt).Milliseconds())
}

// acpModels reads a model state ({currentModelId, availableModels}).
func acpModels(v any) ([]core.ModelInfo, string) {
	list, _ := get(v, "availableModels").([]any)
	var ms []core.ModelInfo
	for _, x := range list {
		id := str(x, "modelId")
		if id == "" {
			continue
		}
		m := core.ModelInfo{Value: id, Resolved: id, DisplayName: str(x, "name"), Description: str(x, "description")}
		if efforts, ok := get(x, "_meta.reasoningEfforts").([]any); ok {
			for _, e := range efforts {
				if v := str(e, "value"); v != "" {
					m.EffortLevels = append(m.EffortLevels, v)
				}
			}
		}
		ms = append(ms, m)
	}
	return ms, str(v, "currentModelId")
}

func (d *acp) agentRequest(m rpc, raw []byte) ([]core.Event, [][]byte) {
	if m.Method != "session/request_permission" {
		return nil, [][]byte{line(map[string]any{"jsonrpc": "2.0", "id": m.ID,
			"error": map[string]any{"code": -32601, "message": "UNCLI doesn't offer " + m.Method}})}
	}
	var p struct {
		ToolCall json.RawMessage `json:"toolCall"`
		Options  []acpOption     `json:"options"`
	}
	_ = json.Unmarshal(m.Params, &p)
	var call any
	_ = json.Unmarshal(p.ToolCall, &call)
	t := d.tool(call)
	req := "acp-" + idKey(m.ID)
	d.perms[req] = acpPerm{id: m.ID, options: p.Options, toolCall: str(call, "toolCallId")}
	return []core.Event{core.NewEvent(core.EvApprovalAsked, core.ApprovalAsked{
		RequestID: req, Tool: t.identity(), Input: t.input, Description: firstLine(t.title), ToolUseID: str(call, "toolCallId"),
		Action: d.a.acpAction(t),
	}, raw)}, nil
}

// tool records what a tool call (or an update of one) says about it.
func (d *acp) tool(u any) *acpTool {
	id := str(u, "toolCallId")
	t := d.tools[id]
	if t == nil {
		t = &acpTool{}
		d.tools[id] = t
	}
	if s := str(u, "name"); s != "" {
		t.name = s
	} else if s, k := metaTool(u); s != "" && t.name == "" {
		// Agents describe their own tools in _meta (Grok: "x.ai/tool").
		t.name = s
		if t.kind == "" {
			t.kind = k
		}
	}
	if s := str(u, "kind"); s != "" {
		t.kind = s
	}
	if s := str(u, "title"); s != "" {
		t.title = s
	}
	if v := get(u, "rawInput"); v != nil {
		t.input, _ = json.Marshal(v)
	}
	if v := get(u, "locations"); v != nil {
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &t.locations)
	}
	return t
}

// metaTool finds an agent's description of its own tool in _meta: an
// object there with a name (and maybe a kind).
func metaTool(u any) (name, kind string) {
	meta, _ := get(u, "_meta").(map[string]any)
	for _, v := range meta {
		if m, ok := v.(map[string]any); ok {
			if n, _ := m["name"].(string); n != "" {
				k, _ := m["kind"].(string)
				return n, k
			}
		}
	}
	return "", ""
}

// summary is the trace's one line: the command or file, else the title.
func (t *acpTool) summary() string {
	var in any
	_ = json.Unmarshal(t.input, &in)
	for _, k := range []string{"command", "cmd", "file_path", "target_file", "path", "description"} {
		if s := str(in, k); s != "" {
			return firstLine(s)
		}
	}
	return firstLine(firstNonEmpty(t.title, t.identity()))
}

// identity is the tool's name for the safe list: its name, else its kind.
func (t *acpTool) identity() string {
	switch {
	case t.name != "":
		return t.name
	case t.kind != "":
		return t.kind
	}
	return "tool"
}

func (d *acp) notification(m rpc, raw []byte) []core.Event {
	if m.Method != "session/update" {
		return nil // the agent's own notifications (_x.ai/…): nothing for UNCLI
	}
	if d.loading {
		return nil // the conversation replayed by session/load
	}
	var p struct {
		Update json.RawMessage `json:"update"`
	}
	_ = json.Unmarshal(m.Params, &p)
	var u any
	_ = json.Unmarshal(p.Update, &u)
	switch str(u, "sessionUpdate") {
	case "agent_message_chunk":
		if s := str(u, "content.text"); s != "" {
			d.text.WriteString(s)
			return []core.Event{core.NewEvent(core.EvTextDelta, core.TextDelta{Index: d.block, Text: s}, raw)}
		}
		return nil
	case "agent_thought_chunk":
		return []core.Event{core.NewEvent(core.EvThinking, core.Thinking{}, raw)}
	case "tool_call", "tool_call_update":
		evs := d.flushText()
		id := str(u, "toolCallId")
		_, seen := d.tools[id]
		t := d.tool(u)
		if !seen {
			evs = append(evs, core.NewEvent(core.EvToolStarted, core.ToolStarted{ID: id, Name: t.identity(), Input: t.input,
				Summary: t.summary()}, raw))
		}
		if st := str(u, "status"); (st == "completed" || st == "failed") && !t.done {
			t.done = true
			evs = append(evs, d.finish(id, t, st == "completed", u, raw)...)
		}
		return evs
	case "plan", "available_commands_update", "current_mode_update", "config_option_update", "session_info_update", "usage_update", "user_message_chunk":
		return nil
	}
	return []core.Event{core.NewEvent(core.EvUnknown, core.Unknown{Type: str(u, "sessionUpdate")}, raw)}
}

func (d *acp) finish(id string, t *acpTool, ok bool, u any, raw []byte) []core.Event {
	fin := core.ToolFinished{ID: id, OK: ok, Denied: d.denied[id]}
	var out []string
	var files []core.Event
	content, _ := get(u, "content").([]any)
	for _, c := range content {
		switch str(c, "type") {
		case "content":
			if s := str(c, "content.text"); s != "" {
				out = append(out, s)
			}
		case "diff":
			if ok && str(c, "path") != "" {
				f := core.FileTouched{Path: str(c, "path"), How: "edit"}
				if old := get(c, "oldText"); old == nil || old == "" {
					f.How = "write" // a new file (null, or empty as Grok sends it)
				}
				for _, l := range t.locations {
					if l.Path == f.Path && l.Line > 0 {
						f.Line = l.Line
					}
				}
				files = append(files, core.NewEvent(core.EvFileTouched, f, nil))
			}
		}
	}
	if len(out) == 0 {
		if v := get(u, "rawOutput"); v != nil {
			out = append(out, text(v))
		}
	}
	fin.Output = truncate(strings.Join(out, "\n"))
	return append([]core.Event{core.NewEvent(core.EvToolFinished, fin, raw)}, files...)
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

// flushText ends the answer's current block: text after a tool call is a
// block of its own.
func (d *acp) flushText() []core.Event {
	if d.text.Len() == 0 {
		return nil
	}
	s := strings.TrimSpace(d.text.String())
	d.text.Reset()
	d.block++
	if s == "" {
		return nil
	}
	return []core.Event{core.NewEvent(core.EvTextBlock, core.TextBlock{Text: s}, nil)}
}

func (d *acp) Turn(t core.UserTurn) ([]byte, error) {
	if d.failed != nil {
		return nil, d.failed
	}
	if len(t.Attachments) > 0 {
		return nil, fmt.Errorf("%s can't be sent files", d.a.M.Name)
	}
	if !d.ready {
		d.queued = append(d.queued, t)
		return nil, nil
	}
	return d.prompt(t)
}

func (d *acp) prompt(t core.UserTurn) ([]byte, error) {
	msg := t.Text
	if t.Directives != "" {
		msg = strings.TrimSpace(t.Directives + "\n\n" + t.Text)
	}
	d.promptAt = time.Now()
	d.block = 0
	return d.request("session/prompt", map[string]any{"sessionId": d.session,
		"prompt": []any{map[string]string{"type": "text", "text": msg}}}), nil
}

func (d *acp) Control(c core.Control) ([]byte, bool) {
	switch c.Kind {
	case core.CtlApprove:
		p, ok := d.perms[c.RequestID]
		if !ok {
			return nil, false
		}
		delete(d.perms, c.RequestID)
		if !c.Allow {
			d.denied[p.toolCall] = true
		}
		outcome := map[string]any{"outcome": "cancelled"}
		if id := chooseOption(p.options, c.Allow); id != "" {
			outcome = map[string]any{"outcome": "selected", "optionId": id}
		}
		return line(map[string]any{"jsonrpc": "2.0", "id": p.id, "result": map[string]any{"outcome": outcome}}), true
	case core.CtlInterrupt:
		if d.session == "" {
			return nil, false
		}
		return line(map[string]any{"jsonrpc": "2.0", "method": "session/cancel", "params": map[string]string{"sessionId": d.session}}), true
	}
	return nil, false
}

// chooseOption picks the agent's option for UNCLI's answer: once, never
// "always" (UNCLI's safe list remembers what the user teaches it).
func chooseOption(opts []acpOption, allow bool) string {
	want, also := "reject_once", "reject_always"
	if allow {
		want, also = "allow_once", ""
	}
	for _, k := range []string{want, also} {
		for _, o := range opts {
			if k != "" && o.Kind == k {
				return o.OptionID
			}
		}
	}
	return ""
}

// acpKinds maps ACP's tool kinds to UNCLI's action kinds.
var acpKinds = map[string]string{
	"execute": core.ActShell, "read": core.ActRead, "edit": core.ActEdit, "search": core.ActSearch,
	"fetch": core.ActWeb, "think": core.ActInternal, "switch_mode": core.ActPlan,
	"write":  core.ActWrite, // not ACP's, but agents' own metadata says it (Grok)
	"delete": core.ActOther, "move": core.ActOther, "other": core.ActOther,
}

// acpAction says what an ACP tool call does, in UNCLI's terms: the
// manifest's kinds by name first, then ACP's own kind.
func (a *Adapter) acpAction(t *acpTool) core.ToolAction {
	tl := a.M.Tools
	name := t.identity()
	act := core.ToolAction{Kind: tl.Kinds[name], Tool: name, Input: t.input}
	if act.Kind == "" {
		for prefix, k := range tl.Prefixes {
			if strings.HasPrefix(name, prefix) {
				act.Kind = k
			}
		}
	}
	if act.Kind == "" {
		act.Kind = acpKinds[t.kind]
	}
	if act.Kind == "" {
		act.Kind = core.ActOther
	}
	var in any
	_ = json.Unmarshal(t.input, &in)
	switch act.Kind {
	case core.ActShell:
		keys := tl.Command
		if len(keys) == 0 {
			keys = Paths{"command", "cmd", "commandLine", "CommandLine"}
		}
		for _, k := range keys {
			switch v := get(in, k).(type) {
			case string:
				act.Command = v
			case []any:
				parts := make([]string, 0, len(v))
				for _, p := range v {
					parts = append(parts, text(p))
				}
				act.Command = strings.Join(parts, " ")
			}
			if act.Command != "" {
				break
			}
		}
		if act.Command == "" {
			act.Command = t.title
		}
		act.Dialect = tl.Shell.Dialect[runtime.GOOS]
		if act.Dialect == "" {
			act.Dialect = tl.Shell.Dialect["other"]
		}
		if act.Dialect == "" {
			act.Dialect = "bash"
		}
	case core.ActRead, core.ActWrite, core.ActEdit, core.ActSearch:
		if len(t.locations) > 0 {
			act.Path = t.locations[0].Path
		}
		if act.Path == "" {
			keys := tl.Path
			if len(keys) == 0 {
				keys = Paths{"path", "file_path", "filePath", "absolute_path", "target_file"}
			}
			act.Path = first(in, keys)
		}
	case core.ActMCP:
		if s, n := first(in, tl.MCP.Server), first(in, tl.MCP.Tool); s != "" && n != "" {
			act.Tool = "mcp__" + s + "__" + n
		}
	}
	return act
}
