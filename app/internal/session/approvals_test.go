package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"uncli/internal/store"
)

// Scripted CLI output for approvals: what the CLI prints around a
// can_use_tool request (shapes as recorded in perm-stdio-allow).
func toolUseLine(id, tool string, input any) string {
	b, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{
		"model": "claude-haiku-4-5", "role": "assistant",
		"content": []any{map[string]any{"type": "tool_use", "id": id, "name": tool, "input": input}},
	}, "session_id": "s1"})
	return string(b)
}

func askLine(reqID, toolUseID, tool string, input any) string {
	b, _ := json.Marshal(map[string]any{"type": "control_request", "request_id": reqID, "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": tool, "input": input, "description": tool, "tool_use_id": toolUseID,
	}})
	return string(b)
}

func toolResultLine(toolUseID, text string, isErr bool) string {
	b, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": toolUseID, "content": text, "is_error": isErr}}}})
	return string(b)
}

const scriptedResult = `{"type":"result","subtype":"success","is_error":false,"duration_ms":100,"num_turns":2,"result":"Done.","session_id":"s1","total_cost_usd":0.001,"usage":{"input_tokens":5,"output_tokens":5}}`

func seg(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// lastControl is the last approval answer written to the CLI.
func lastControl(h *harness) map[string]any {
	ls := h.rt.lines()
	for i := len(ls) - 1; i >= 0; i-- {
		if strings.Contains(ls[i], `"control_response"`) {
			var m struct {
				Response struct {
					RequestID string         `json:"request_id"`
					Response  map[string]any `json:"response"`
				} `json:"response"`
			}
			_ = json.Unmarshal([]byte(ls[i]), &m)
			if m.Response.Response == nil {
				continue
			}
			m.Response.Response["request_id"] = m.Response.RequestID
			return m.Response.Response
		}
	}
	return nil
}

func TestApprovalsFlow(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	write := map[string]string{"file_path": "hello.txt", "content": "hi"}
	push := map[string]string{"command": "git push origin main"}
	h.rt.turns = [][]byte{
		// Turn 1: Claude wants to write a file, then to push.
		seg(toolUseLine("toolu_W", "Write", write), askLine("req_W", "toolu_W", "Write", write)),
		seg(toolResultLine("toolu_W", "ok", false), toolUseLine("toolu_P", "Bash", push), askLine("req_P", "toolu_P", "Bash", push)),
		seg(toolResultLine("toolu_P", deniedByUser, true), scriptedResult),
		// Turn 2: the same write (a rule allows it), then the push (a rule denies it).
		seg(toolUseLine("toolu_W2", "Write", write), askLine("req_W2", "toolu_W2", "Write", write)),
		seg(toolResultLine("toolu_W2", "ok", false), toolUseLine("toolu_P2", "Bash", push), askLine("req_P2", "toolu_P2", "Bash", push)),
		seg(toolResultLine("toolu_P2", deniedByRule, true), scriptedResult),
	}
	dir := t.TempDir()
	v, err := h.m.Create("code", dir, "")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := h.m.get(v.ID)
	if err := h.m.Send(context.Background(), v.ID, "write and push"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the write request", func() bool { return len(s.View().Approvals) == 1 })
	view := s.View()
	a := view.Approvals[0]
	if view.State != NeedsApproval || a.Tool != "Write" || a.ToolUseID != "toolu_W" || len(a.Suggestions) != 1 || a.Suggestions[0].Tool != "Write" {
		t.Fatalf("view = %+v", view)
	}
	if args := strings.Join(h.rt.starts[0].Args, " "); !strings.Contains(args, "--permission-prompt-tool stdio") {
		t.Errorf("approvals must be routed to UNCLI: %s", args)
	}

	// Always allow writes in this project.
	if err := h.m.Answer(v.ID, a.RequestID, Always, &a.Suggestions[0]); err != nil {
		t.Fatal(err)
	}
	if c := lastControl(h); c["behavior"] != "allow" || c["request_id"] != "req_W" || c["updatedInput"] == nil {
		t.Errorf("answer sent = %v", c)
	}
	waitUntil(t, "the push request", func() bool { v := s.View(); return len(v.Approvals) == 1 && v.Approvals[0].Tool == "Bash" })
	p := s.View().Approvals[0]
	var sugg []string
	for _, r := range p.Suggestions {
		sugg = append(sugg, r.Prefix)
	}
	if strings.Join(sugg, ",") != "git push,git:publish,git" {
		t.Errorf("push suggestions = %v", sugg)
	}

	// Deny the push: the model is told, and the trace marks it.
	if err := h.m.Answer(v.ID, p.RequestID, Deny, nil); err != nil {
		t.Fatal(err)
	}
	if c := lastControl(h); c["behavior"] != "deny" || c["message"] != deniedByUser {
		t.Errorf("deny sent = %v", c)
	}
	pages := h.waitIdle(v.ID)
	last := pages[len(pages)-1]
	var denied []string
	for _, it := range last.Trace {
		if it.Denied {
			denied = append(denied, it.ID)
		}
	}
	if last.Trace[0].Approved != "you" {
		t.Errorf("the allowed write should say who allowed it: %+v", last.Trace[0])
	}
	if strings.Join(denied, ",") != "toolu_P" || last.Status != "done" {
		t.Errorf("trace = %+v, status %s", last.Trace, last.Status)
	}
	if err := h.m.Answer(v.ID, p.RequestID, Allow, nil); err == nil {
		t.Error("an answered request can't be answered again")
	}

	// Rules belong to the project: another session in the same folder sees them.
	other, _ := h.m.Create("code", dir, "")
	rules, _ := h.m.Rules(other.ID)
	if len(rules) != 1 || rules[0] != (store.ToolRule{Tool: "Write", Action: store.RuleAllow}) {
		t.Errorf("project rules = %+v", rules)
	}
	if _, err := h.m.SetRule(v.ID, store.ToolRule{Tool: "Bash", Prefix: "git push", Action: store.RuleDeny}); err != nil {
		t.Fatal(err)
	}

	// Turn 2: both requests are answered by rules, without a card.
	if err := h.m.Send(context.Background(), v.ID, "again"); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(v.ID)
	if len(s.View().Approvals) != 0 {
		t.Errorf("rules should have answered: %+v", s.View().Approvals)
	}
	if p2, _ := h.m.Pages(v.ID); p2[len(p2)-1].Trace[0].Approved != "rule" {
		t.Errorf("the rule-allowed write should say so: %+v", p2[len(p2)-1].Trace[0])
	}
	answers := map[string]string{}
	for _, l := range h.rt.lines() {
		if strings.Contains(l, "req_W2") {
			answers["W2"] = l
		}
		if strings.Contains(l, "req_P2") {
			answers["P2"] = l
		}
	}
	if !strings.Contains(answers["W2"], `"behavior":"allow"`) || !strings.Contains(answers["P2"], `"behavior":"deny"`) || !strings.Contains(answers["P2"], deniedByRule) {
		t.Errorf("rule answers = %v", answers)
	}
	h.sink.mu.Lock()
	sawWaiting := false
	for _, st := range h.sink.states {
		if st == NeedsApproval {
			sawWaiting = true
		}
	}
	h.sink.mu.Unlock()
	if !sawWaiting {
		t.Error("the sidebar should have shown Needs approval during turn 1")
	}
}

// Stopping a turn that waits on a card denies the request and ends the turn.
func TestApprovalInterrupt(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	write := map[string]string{"file_path": "x.txt", "content": "x"}
	h.rt.turns = [][]byte{seg(toolUseLine("toolu_X", "Write", write), askLine("req_X", "toolu_X", "Write", write))}
	h.m.InterruptGrace = 50 * time.Millisecond
	v, _ := h.m.Create("code", t.TempDir(), "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "write")
	waitUntil(t, "the request", func() bool { return len(s.View().Approvals) == 1 })
	if err := h.m.Interrupt(v.ID); err != nil {
		t.Fatal(err)
	}
	ls := h.rt.lines()
	var denyAt, interruptAt int
	for i, l := range ls {
		if strings.Contains(l, "req_X") && strings.Contains(l, `"deny"`) {
			denyAt = i
		}
		if strings.Contains(l, `"interrupt"`) {
			interruptAt = i
		}
	}
	if denyAt == 0 || interruptAt <= denyAt {
		t.Errorf("stop should deny the waiting request, then interrupt: %v", ls)
	}
	pages := h.waitIdle(v.ID)
	if pages[len(pages)-1].Status != "interrupted" || len(s.View().Approvals) != 0 {
		t.Errorf("status = %s, approvals = %v", pages[len(pages)-1].Status, s.View().Approvals)
	}
}

// "Allow for this session": a rule that lasts as long as this session in
// UNCLI, never reaches the project, and wins ties against project rules.
func TestSessionRules(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	push := map[string]string{"command": "git push origin main"}
	h.rt.turns = [][]byte{
		seg(toolUseLine("toolu_1", "PowerShell", push), askLine("req_1", "toolu_1", "PowerShell", push)),
		seg(toolResultLine("toolu_1", "ok", false), scriptedResult),
		seg(toolUseLine("toolu_2", "PowerShell", push), askLine("req_2", "toolu_2", "PowerShell", push)),
		seg(toolResultLine("toolu_2", "ok", false), scriptedResult),
	}
	dir := t.TempDir()
	v, _ := h.m.Create("code", dir, "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "push")
	waitUntil(t, "the request", func() bool { return len(s.View().Approvals) == 1 })
	a := s.View().Approvals[0]
	if err := h.m.Answer(v.ID, a.RequestID, AllowSession, &a.Suggestions[0]); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(v.ID)
	if r, _ := h.m.Rules(v.ID); len(r) != 0 {
		t.Errorf("a session rule must not reach the project: %+v", r)
	}
	if r, _ := h.m.SessionRules(v.ID); len(r) != 1 || r[0].Prefix != "git push" {
		t.Errorf("session rules = %+v", r)
	}
	// A project rule saying ask, equally specific: the session's allow wins.
	h.m.SetRule(v.ID, store.ToolRule{Tool: "Bash", Prefix: "git push", Action: store.RuleAsk})
	h.m.Send(context.Background(), v.ID, "push again")
	pages := h.waitIdle(v.ID)
	if len(s.View().Approvals) != 0 || pages[len(pages)-1].Trace[0].Approved != "session" {
		t.Errorf("the session rule should have answered: %+v / %+v", s.View().Approvals, pages[len(pages)-1].Trace)
	}
	// Another session in the same folder doesn't share it.
	other, _ := h.m.Create("code", dir, "")
	if r, _ := h.m.SessionRules(other.ID); len(r) != 0 {
		t.Errorf("session rules leaked: %+v", r)
	}
	// Make it permanent: it moves to the project.
	if err := h.m.PromoteSessionRule(v.ID, store.ToolRule{Tool: "PowerShell", Prefix: "git push", Action: store.RuleAllow}); err != nil {
		t.Fatal(err)
	}
	sr, _ := h.m.SessionRules(v.ID)
	pr, _ := h.m.Rules(other.ID)
	if len(sr) != 0 || len(pr) != 2 {
		t.Errorf("after promote: session %+v, project %+v", sr, pr)
	}
}

func TestDecidePrecedence(t *testing.T) {
	sess := []store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}
	proj := []store.ToolRule{{Tool: "Bash", Prefix: "git push", Action: store.RuleDeny}}
	if a, from := decide(sess, proj, "Bash", bash("git push")); a != store.RuleDeny || from != "rule" {
		t.Errorf("the more specific project rule must win: %s %s", a, from)
	}
	if a, from := decide(sess, proj, "Bash", bash("git log")); a != store.RuleAllow || from != "session" {
		t.Errorf("= %s %s", a, from)
	}
	if a, from := decide(nil, nil, "Bash", bash("git log")); a != "" || from != "" {
		t.Errorf("no rules = %s %s", a, from)
	}
}

// Unattended: rules still answer, but whatever would wait for the user is
// declined with a note to the model, and the model is told when the mode
// changes.
func TestUnattended(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	write := map[string]string{"file_path": "a.txt", "content": "a"}
	chained := map[string]string{"command": "npm test && git push"}
	question := map[string]any{"questions": []any{map[string]any{"question": "Which one?"}}}
	h.rt.turns = [][]byte{
		// Turn 1: a card is waiting when the user walks away.
		seg(toolUseLine("toolu_1", "Write", write), askLine("req_1", "toolu_1", "Write", write)),
		seg(toolResultLine("toolu_1", unattendedDeclined, true), scriptedResult),
		// Turn 2: a rule allows the write; the chained command and the question are declined.
		seg(toolUseLine("toolu_2", "Write", write), askLine("req_2", "toolu_2", "Write", write)),
		seg(toolResultLine("toolu_2", "ok", false), toolUseLine("toolu_3", "Bash", chained), askLine("req_3", "toolu_3", "Bash", chained)),
		seg(toolResultLine("toolu_3", unattendedDeclined, true), toolUseLine("toolu_4", "AskUserQuestion", question), askLine("req_4", "toolu_4", "AskUserQuestion", question)),
		seg(toolResultLine("toolu_4", unattendedQuestion, true), scriptedResult),
		seg(scriptedResult), // turn 3
		seg(scriptedResult), // turn 4
	}
	v, _ := h.m.Create("code", t.TempDir(), "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "write")
	waitUntil(t, "the request", func() bool { return len(s.View().Approvals) == 1 })

	// Turning it on declines the waiting card.
	view, err := h.m.SetUnattended(v.ID, true)
	if err != nil || !view.Unattended || len(view.Approvals) != 0 {
		t.Fatalf("view = %+v, %v", view, err)
	}
	if c := lastControl(h); c["behavior"] != "deny" || c["request_id"] != "req_1" || c["message"] != unattendedDeclined {
		t.Errorf("waiting card answer = %v", c)
	}
	pages := h.waitIdle(v.ID)
	if !pages[len(pages)-1].Trace[0].Denied {
		t.Errorf("trace = %+v", pages[len(pages)-1].Trace)
	}

	// Turn 2: told it's unattended; rules still apply; nothing waits.
	h.m.SetRule(v.ID, store.ToolRule{Tool: "Write", Action: store.RuleAllow})
	h.m.Send(context.Background(), v.ID, "carry on")
	pages = h.waitIdle(v.ID)
	answers := map[string]string{}
	for _, l := range h.rt.lines() {
		for _, id := range []string{"req_2", "req_3", "req_4"} {
			if strings.Contains(l, id) {
				answers[id] = l
			}
		}
	}
	if !strings.Contains(answers["req_2"], `"behavior":"allow"`) {
		t.Errorf("the rule should still allow the write: %s", answers["req_2"])
	}
	if !strings.Contains(answers["req_3"], `"behavior":"deny"`) || !strings.Contains(answers["req_3"], "separate commands") {
		t.Errorf("chained command answer = %s", answers["req_3"])
	}
	if !strings.Contains(answers["req_4"], `"behavior":"deny"`) || !strings.Contains(answers["req_4"], "can't answer questions") {
		t.Errorf("question answer = %s", answers["req_4"])
	}
	if p := pages[len(pages)-1]; p.Status != "done" || p.Trace[0].Approved != "rule" || !p.Trace[1].Denied || !p.Trace[2].Denied {
		t.Errorf("turn 2 = %s %+v", p.Status, p.Trace)
	}

	// Turn 3 says nothing new; turn 4, after switching off, says it's off.
	h.m.Send(context.Background(), v.ID, "more")
	h.waitIdle(v.ID)
	h.m.SetUnattended(v.ID, false)
	h.m.Send(context.Background(), v.ID, "back")
	h.waitIdle(v.ID)
	if told := toldPerTurn(h); told != "-,on,-,off" {
		t.Errorf("directives per turn = %v", told)
	}
}

// toldPerTurn is what each turn sent told the model about unattended mode.
func toldPerTurn(h *harness) string {
	var told []string
	for _, l := range h.rt.lines() {
		if !strings.Contains(l, `"type":"user"`) {
			continue
		}
		switch {
		case strings.Contains(l, "Unattended mode is on"):
			told = append(told, "on")
		case strings.Contains(l, "Unattended mode is off"):
			told = append(told, "off")
		default:
			told = append(told, "-")
		}
	}
	return strings.Join(told, ",")
}
