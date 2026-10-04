package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"uncli/internal/store"
)

// hintsLine is the CLI's answer to mcp_status for the notes server of the
// mcp-status-tool-hints fixture.
func hintsLine() string {
	b, _ := json.Marshal(map[string]any{"type": "control_response", "response": map[string]any{
		"subtype": "success", "request_id": "uncli_1", "response": map[string]any{"mcpServers": []any{map[string]any{
			"name": "notes", "status": "connected", "tools": []any{
				map[string]any{"name": "read_note", "annotations": map[string]any{"readOnly": true}},
				map[string]any{"name": "delete_note", "annotations": map[string]any{"destructive": true}},
				map[string]any{"name": "touch_note", "annotations": map[string]any{}},
			},
		}}},
	}})
	return string(b)
}

// answerTo is the answer UNCLI wrote for a request.
func answerTo(h *harness, reqID string) string {
	for _, l := range h.rt.lines() {
		if strings.Contains(l, `"control_response"`) && strings.Contains(l, reqID) {
			return l
		}
	}
	return ""
}

func controlsSent(h *harness, subtype string) []string {
	var out []string
	for _, l := range h.rt.lines() {
		if strings.Contains(l, `"subtype":"`+subtype+`"`) {
			out = append(out, l)
		}
	}
	return out
}

func TestPromptLevels(t *testing.T) {
	dir := t.TempDir()
	h := openHarness(t, dir, "perm-stdio-allow")
	h.rt.onControl = true
	note := map[string]string{"name": "x"}
	tests := map[string]string{"command": "npm test"}
	h.rt.turns = [][]byte{
		// Turn 1, Prompt when unsafe: the hints arrive; an unannotated tool prompts.
		seg(hintsLine(), toolUseLine("toolu_1", "mcp__notes__touch_note", note), askLine("req_1", "toolu_1", "mcp__notes__touch_note", note)),
		// After the switch to Run without prompting it runs, and so does a destructive one.
		seg(toolResultLine("toolu_1", "ok", false), toolUseLine("toolu_2", "mcp__notes__delete_note", note), askLine("req_2", "toolu_2", "mcp__notes__delete_note", note)),
		seg(toolResultLine("toolu_2", "ok", false), scriptedResult),
		// Turn 2, Always prompt me: reading runs, the tests prompt.
		seg(toolUseLine("toolu_3", "mcp__notes__read_note", note), askLine("req_3", "toolu_3", "mcp__notes__read_note", note)),
		seg(toolResultLine("toolu_3", "ok", false), toolUseLine("toolu_4", "Bash", tests), askLine("req_4", "toolu_4", "Bash", tests)),
		seg(toolResultLine("toolu_4", "ok", false), scriptedResult),
	}
	v, _ := h.m.Create("code", t.TempDir(), "")
	if v.Mode != "" {
		t.Fatalf("a new session = %+v", v.Session)
	}
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "go")
	waitUntil(t, "the card", func() bool { return len(s.View().Approvals) == 1 })
	args := strings.Join(h.rt.starts[0].Args, " ")
	if strings.Contains(args, "--allowedTools") || !strings.Contains(args, "--permission-mode acceptEdits") {
		t.Errorf("UNCLI applies the session type's list itself, and keeps its mode: %s", args)
	}

	// Run without prompting answers the waiting card and what follows.
	view, err := h.m.SetMode(v.ID, ModeNever)
	if err != nil || view.Mode != ModeNever || len(view.Approvals) != 0 {
		t.Fatalf("view = %+v, %v", view, err)
	}
	pages := h.waitIdle(v.ID)
	if tr := pages[0].Trace; tr[0].Approved != "never" || tr[1].Approved != "never" {
		t.Errorf("trace = %+v", tr)
	}
	if _, err := h.m.SetMode(v.ID, "readonly"); err == nil {
		t.Error("old level names are refused")
	}

	// Always prompt me: the CLI stops approving edits itself.
	h.m.SetMode(v.ID, ModeAlways)
	if m := controlsSent(h, "set_permission_mode"); len(m) != 1 || !strings.Contains(m[0], `"mode":"default"`) {
		t.Errorf("mode controls = %v", m)
	}
	h.m.Send(context.Background(), v.ID, "test")
	waitUntil(t, "the tests' card", func() bool { a := s.View().Approvals; return len(a) == 1 && a[0].RequestID == "req_4" })
	if !strings.Contains(answerTo(h, "req_3"), `"behavior":"allow"`) {
		t.Error("reading runs under Always prompt me")
	}
	if a := s.View().Approvals[0]; a.Why[0].By != "always" {
		t.Errorf("card = %+v", a.Why)
	}
	h.m.Answer(v.ID, "req_4", Allow, "")
	h.waitIdle(v.ID)

	// The level outlasts a restart, and an old Read-only session comes back
	// as Always prompt me.
	h.m.Close()
	h.db.Close()
	h2 := openHarness(t, dir, "multi-turn-partial")
	if list := h2.m.List(); len(list) != 1 || list[0].Mode != ModeAlways {
		t.Fatalf("restored = %+v", list)
	}
	h2.send(v.ID, "hello")
	if args := strings.Join(h2.rt.starts[0].Args, " "); !strings.Contains(args, "--permission-mode default") {
		t.Errorf("Always starts the CLI in its default mode: %s", args)
	}
}

// Explaining a command uses the session type's list even before the CLI
// has started, and follows the level and the safe list.
func TestExplainCommand(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	v, _ := h.m.Create("code", t.TempDir(), "")
	e, err := h.m.ExplainCommand(v.ID, `git status --short; npm --prefix "packages\core" test; npm run lint`)
	if err != nil || e.Action != actionRun || len(e.Why) != 3 {
		t.Fatalf("explanation = %+v, %v", e, err)
	}
	if e.Why[0].By != "looks" || e.Why[1].By != "safe" || e.Why[2].By != "safe" { // the same for every stack
		t.Errorf("why = %+v", e.Why)
	}
	h.m.SetSafeEntry(v.ID, store.SafeEntry{Kind: store.KindCommand, Words: "npm run lint", Verdict: store.Unsafe}, ScopeProject)
	if e, _ := h.m.ExplainCommand(v.ID, "npm run lint"); e.Action != actionPrompt || e.Why[0].Entry == nil {
		t.Errorf("marked unsafe = %+v", e)
	}
	h.m.SetMode(v.ID, ModeAlways)
	if e, _ := h.m.ExplainCommand(v.ID, "npm test"); e.Action != actionPrompt || e.Why[0].By != "always" {
		t.Errorf("always = %+v", e)
	}
	pv, _ := h.m.PreviewClasses(v.ID, "npm run test -- -u src/a.test.ts; cp a /etc/x/; node -e \"x()\"")
	if len(pv) != 3 || pv[0].Class == nil || pv[0].Class.Words != "npm run test" || pv[1].Fixed != FixedContext || pv[2].Fixed != FixedInline {
		t.Errorf("preview = %+v", pv)
	}
	if list, _ := h.m.SessionAllowlist(v.ID); len(list) == 0 || list[0] != "Bash(git status:*)" {
		t.Errorf("allowlist = %v", list)
	}
}

// A command UNCLI doesn't recognise waits, without a card, while the model
// judges it; the verdict is kept, so the same command isn't judged again.
func TestModelJudgesUnknownCommands(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	calls := make(chan JudgeQuery, 4)
	release := make(chan store.Judgement)
	h.m.d.Judge = func(ctx context.Context, q JudgeQuery) (store.Judgement, error) {
		calls <- q
		return <-release, nil
	}
	h.m.d.Unknown = func() string { return UnknownModel }
	frob := map[string]string{"command": "frobnicate --all"}
	wipe := map[string]string{"command": "wipe-cache"}
	h.rt.turns = [][]byte{
		seg(toolUseLine("toolu_1", "Bash", frob), askLine("req_1", "toolu_1", "Bash", frob)),
		seg(toolResultLine("toolu_1", "ok", false), toolUseLine("toolu_2", "Bash", wipe), askLine("req_2", "toolu_2", "Bash", wipe)),
		seg(toolResultLine("toolu_2", "ok", false), scriptedResult),
		seg(toolUseLine("toolu_3", "Bash", frob), askLine("req_3", "toolu_3", "Bash", frob)),
		seg(toolResultLine("toolu_3", "ok", false), scriptedResult),
	}
	v, _ := h.m.Create("code", t.TempDir(), "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "go")

	q := <-calls
	if q.Part != "frobnicate --all" || q.Workdir == "" {
		t.Errorf("query = %+v", q)
	}
	view := s.View()
	if len(view.Approvals) != 1 || !view.Approvals[0].Judging || view.State == NeedsApproval {
		t.Fatalf("while judging: state %s, approvals %+v", view.State, view.Approvals)
	}
	release <- store.Judgement{Level: RiskRoutine, Note: "Rebuilds the index.", Model: "haiku"}
	waitUntil(t, "the routine command to run", func() bool { return strings.Contains(answerTo(h, "req_1"), `"behavior":"allow"`) })

	// A risky verdict brings a card that says why.
	<-calls
	release <- store.Judgement{Level: RiskRisky, Risk: WhyDeletes, Note: "Deletes the build cache for good.", Model: "haiku"}
	waitUntil(t, "the card", func() bool { a := s.View().Approvals; return len(a) == 1 && !a[0].Judging })
	a := s.View().Approvals[0]
	if a.Why[0].Risk != WhyDeletes || a.Why[0].Note != "Deletes the build cache for good." || s.View().State != NeedsApproval {
		t.Errorf("card = %+v", a)
	}
	h.m.Answer(v.ID, a.RequestID, Allow, "")
	pages := h.waitIdle(v.ID)
	if tr := pages[0].Trace[0]; tr.Approved != "safe" || tr.Why[0].Judged != "haiku" {
		t.Errorf("trace = %+v", tr)
	}

	// The same command again: no second judgement.
	h.m.Send(context.Background(), v.ID, "again")
	h.waitIdle(v.ID)
	select {
	case q := <-calls:
		t.Errorf("judged again: %+v", q)
	default:
	}
	if !strings.Contains(answerTo(h, "req_3"), `"behavior":"allow"`) {
		t.Error("the cached verdict should allow it")
	}
}
