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

// A push prompts. "Allow once" lets it run and teaches nothing; "This is
// safe" (this project) remembers the class, so the next push runs without
// a card here, a forced push still prompts, and another project still
// prompts.
func TestTeachingFromACard(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	push := map[string]string{"command": "git push origin main"}
	force := map[string]string{"command": "git push --force origin main"}
	h.rt.turns = [][]byte{
		// Turn 1: a push, allowed once.
		seg(toolUseLine("toolu_1", "PowerShell", push), askLine("req_1", "toolu_1", "PowerShell", push)),
		seg(toolResultLine("toolu_1", "ok", false), scriptedResult),
		// Turn 2: a push again, marked safe for this project.
		seg(toolUseLine("toolu_2", "PowerShell", push), askLine("req_2", "toolu_2", "PowerShell", push)),
		seg(toolResultLine("toolu_2", "ok", false), scriptedResult),
		// Turn 3: a push runs on its own; a forced push still prompts, and is denied.
		seg(toolUseLine("toolu_3", "PowerShell", push), askLine("req_3", "toolu_3", "PowerShell", push)),
		seg(toolResultLine("toolu_3", "ok", false), toolUseLine("toolu_4", "PowerShell", force), askLine("req_4", "toolu_4", "PowerShell", force)),
		seg(toolResultLine("toolu_4", deniedOnCard, true), scriptedResult),
	}
	dir := t.TempDir()
	v, _ := h.m.Create("code", dir, "")
	s, _ := h.m.get(v.ID)

	h.m.Send(context.Background(), v.ID, "push")
	waitUntil(t, "the card", func() bool { return len(s.View().Approvals) == 1 })
	a := s.View().Approvals[0]
	if a.Why[0].By != "unsafe" || a.Why[0].Risk != WhyPublishes || len(a.Learn) != 1 || a.Learn[0].Words != "git push" {
		t.Fatalf("card = %+v", a)
	}
	h.m.Answer(v.ID, a.RequestID, Allow, "")
	h.waitIdle(v.ID)
	if l, _ := h.m.SafeList(v.ID); len(l) != 0 {
		t.Errorf("allow once teaches nothing: %+v", l)
	}

	h.m.Send(context.Background(), v.ID, "push again")
	waitUntil(t, "the card", func() bool { return len(s.View().Approvals) == 1 })
	if err := h.m.Answer(v.ID, s.View().Approvals[0].RequestID, Safe, ScopeProject); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(v.ID)
	l, _ := h.m.SafeList(v.ID)
	if len(l) != 1 || l[0] != (store.SafeEntry{Kind: store.KindCommand, Words: "git push", Verdict: store.Safe, Folder: dir}) {
		t.Fatalf("safe list = %+v", l)
	}

	h.m.Send(context.Background(), v.ID, "and again")
	waitUntil(t, "the forced push's card", func() bool { a := s.View().Approvals; return len(a) == 1 && a[0].RequestID == "req_4" })
	if !strings.Contains(answerTo(h, "req_3"), `"behavior":"allow"`) {
		t.Error("the learned push should run on its own")
	}
	if f := s.View().Approvals[0]; f.Learn[0].Flags != "--force" {
		t.Errorf("a forced push is its own class: %+v", f.Learn)
	}
	h.m.Answer(v.ID, "req_4", Deny, "")
	pages := h.waitIdle(v.ID)
	tr := pages[len(pages)-1].Trace
	if tr[0].Approved != "listed" || tr[0].Why[0].Entry == nil || !tr[1].Denied {
		t.Errorf("trace = %+v", tr)
	}

	// Another project still prompts; promoting the entry to all projects
	// changes that.
	other, _ := h.m.Create("code", t.TempDir(), "")
	if e, _ := h.m.ExplainCommand(other.ID, "git push"); e.Action != actionPrompt {
		t.Errorf("another project: %+v", e)
	}
	if _, err := h.m.MoveSafeEntry(v.ID, l[0], ScopeAll); err != nil {
		t.Fatal(err)
	}
	if e, _ := h.m.ExplainCommand(other.ID, "git push"); e.Action != actionRun || e.Why[0].By != "listed" {
		t.Errorf("after promoting: %+v", e)
	}
	if err := h.m.Answer(v.ID, "nope", Safe, ScopeAll); err == nil {
		t.Error("only a waiting request can be answered")
	}
}

func TestApprovalInterrupt(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	write := map[string]string{"file_path": "~/.ssh/uncli-test", "content": "x"} // secrets: always prompts
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

func TestUnattended(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	push := map[string]string{"command": "git push"}
	tests := map[string]string{"command": "npm test"}
	question := map[string]any{"questions": []any{map[string]any{"question": "Which one?"}}}
	h.rt.turns = [][]byte{
		// Turn 1: a card is waiting when the user walks away.
		seg(toolUseLine("toolu_1", "Bash", push), askLine("req_1", "toolu_1", "Bash", push)),
		seg(toolResultLine("toolu_1", unattendedDeclined, true), scriptedResult),
		// Turn 2: safe work runs; the push and the question are declined.
		seg(toolUseLine("toolu_2", "Bash", tests), askLine("req_2", "toolu_2", "Bash", tests)),
		seg(toolResultLine("toolu_2", "ok", false), toolUseLine("toolu_3", "Bash", push), askLine("req_3", "toolu_3", "Bash", push)),
		seg(toolResultLine("toolu_3", unattendedDeclined, true), toolUseLine("toolu_4", "AskUserQuestion", question), askLine("req_4", "toolu_4", "AskUserQuestion", question)),
		seg(toolResultLine("toolu_4", unattendedQuestion, true), scriptedResult),
		// Turn 3, under "Always prompt me": even the tests are declined.
		seg(toolUseLine("toolu_5", "Bash", tests), askLine("req_5", "toolu_5", "Bash", tests)),
		seg(toolResultLine("toolu_5", unattendedDeclined, true), scriptedResult),
		seg(scriptedResult), // turn 4
	}
	v, _ := h.m.Create("code", t.TempDir(), "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "push")
	waitUntil(t, "the request", func() bool { return len(s.View().Approvals) == 1 })

	// Turning it on declines the waiting card.
	view, err := h.m.SetUnattended(v.ID, true)
	if err != nil || !view.Unattended || len(view.Approvals) != 0 {
		t.Fatalf("view = %+v, %v", view, err)
	}
	if c := lastControl(h); c["behavior"] != "deny" || c["request_id"] != "req_1" || c["message"] != unattendedDeclined {
		t.Errorf("waiting card answer = %v", c)
	}
	h.waitIdle(v.ID)

	h.m.Send(context.Background(), v.ID, "carry on")
	pages := h.waitIdle(v.ID)
	if !strings.Contains(answerTo(h, "req_2"), `"behavior":"allow"`) {
		t.Errorf("safe work should run: %s", answerTo(h, "req_2"))
	}
	if !strings.Contains(answerTo(h, "req_3"), "separate commands") || !strings.Contains(answerTo(h, "req_4"), "can't answer questions") {
		t.Errorf("push = %s\nquestion = %s", answerTo(h, "req_3"), answerTo(h, "req_4"))
	}
	if p := pages[len(pages)-1]; p.Trace[0].Approved != "safe" || !p.Trace[1].Denied || !p.Trace[2].Denied {
		t.Errorf("turn 2 = %+v", p.Trace)
	}

	h.m.SetMode(v.ID, ModeAlways)
	h.m.Send(context.Background(), v.ID, "test")
	h.waitIdle(v.ID)
	if !strings.Contains(answerTo(h, "req_5"), `"behavior":"deny"`) {
		t.Errorf("Always + Unattended declines anything but reading: %s", answerTo(h, "req_5"))
	}

	// Turn 4, after switching off, says it's off.
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

// The model hears the prompt level when it changes, and once after a
// restore, so a conversation told it was read-only (an older version's
// mode) stops refusing to try tools.
func TestPromptLevelDirectives(t *testing.T) {
	dir := t.TempDir()
	h := openHarness(t, dir, "multi-turn-partial")
	v, _ := h.m.Create("chat", "", "")
	h.send(v.ID, "one")
	h.m.SetMode(v.ID, ModeAlways)
	h.send(v.ID, "two")
	pages, _ := h.m.Pages(v.ID)
	if strings.Contains(pages[0].Directives, "Prompt level") || !strings.Contains(pages[1].Directives, levelAlways) {
		t.Fatalf("directives = %q / %q", pages[0].Directives, pages[1].Directives)
	}
	h.m.Close()
	h.db.Close()

	h2 := openHarness(t, dir, "resume-partial")
	s, _ := h2.m.get(v.ID)
	s.mu.Lock()
	told := s.modeDirectives()
	s.toldLocked()
	again := s.modeDirectives()
	s.mu.Unlock()
	if len(told) != 1 || told[0] != levelAlways || len(again) != 0 {
		t.Errorf("after restore = %v, then %v", told, again)
	}
}

// "This is safe" toggles on the card: on, the class is on the safe list
// and the card waits for the user; off, the list is as it was before.
func TestMarkSafeToggle(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	push := map[string]string{"command": "git push origin main"}
	h.rt.turns = [][]byte{
		seg(toolUseLine("toolu_1", "PowerShell", push), askLine("req_1", "toolu_1", "PowerShell", push)),
		seg(toolResultLine("toolu_1", "ok", false), scriptedResult),
	}
	dir := t.TempDir()
	v, _ := h.m.Create("code", dir, "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "push")
	waitUntil(t, "the card", func() bool { return len(s.View().Approvals) == 1 })
	rid := s.View().Approvals[0].RequestID
	list := func() []store.SafeEntry { l, _ := h.m.SafeList(v.ID); return l }

	// No entry before: on adds one, the card stays; off removes it again.
	if err := h.m.MarkSafe(v.ID, rid, true, ScopeAll); err != nil {
		t.Fatal(err)
	}
	if l := list(); len(l) != 1 || l[0].Verdict != store.Safe || l[0].Folder != "" {
		t.Fatalf("marked = %+v", l)
	}
	if a := s.View().Approvals; len(a) != 1 || !a[0].Marked || a[0].MarkedScope != ScopeAll {
		t.Fatalf("the card must keep waiting, marked: %+v", a)
	}
	h.m.MarkSafe(v.ID, rid, false, "")
	if l := list(); len(l) != 0 {
		t.Errorf("unmarked = %+v", l)
	}

	// An Unsafe entry before (with a label): on makes it Safe, off puts it back.
	h.m.SetSafeEntry(v.ID, store.SafeEntry{Kind: store.KindCommand, Words: "git push", Verdict: store.Unsafe, Label: "Publishes"}, ScopeProject)
	h.m.MarkSafe(v.ID, rid, true, ScopeProject)
	if l := list(); len(l) != 1 || l[0].Verdict != store.Safe {
		t.Fatalf("marked over unsafe = %+v", l)
	}
	h.m.MarkSafe(v.ID, rid, false, "")
	if l := list(); len(l) != 1 || l[0].Verdict != store.Unsafe || l[0].Label != "Publishes" {
		t.Errorf("restored = %+v", l)
	}

	// Marked, then allowed once: the push runs and the mark stays.
	h.m.MarkSafe(v.ID, rid, true, ScopeProject)
	if err := h.m.Answer(v.ID, rid, Allow, ""); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(v.ID)
	if !strings.Contains(answerTo(h, rid), `"behavior":"allow"`) || list()[0].Verdict != store.Safe {
		t.Errorf("answer = %s, list = %+v", answerTo(h, rid), list())
	}
}
