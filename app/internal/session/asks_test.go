package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The input AskUserQuestion was asked with (fixture ask-user-question).
var cityAndMeals = map[string]any{"questions": []any{
	map[string]any{"question": "Which city do you prefer?", "header": "City", "multiSelect": false,
		"options": []any{map[string]any{"label": "Lisbon"}, map[string]any{"label": "Porto"}, map[string]any{"label": "Faro"}}},
	map[string]any{"question": "Which meals would you like to plan?", "header": "Meals", "multiSelect": true,
		"options": []any{map[string]any{"label": "Breakfast"}, map[string]any{"label": "Lunch"}, map[string]any{"label": "Dinner"}}},
}}

// A question is answered on its card: the answers go back with the allow,
// by question, as the CLI takes them; Allow alone isn't enough.
func TestAnsweringAQuestion(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	h.rt.turns = [][]byte{
		seg(toolUseLine("toolu_q", "AskUserQuestion", cityAndMeals), askLine("req_q", "toolu_q", "AskUserQuestion", cityAndMeals)),
		seg(toolResultLine("toolu_q", "answered", false), scriptedResult),
	}
	v, _ := h.m.Create("chat", "", "haiku")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "ask me")
	waitUntil(t, "the question", func() bool { return len(s.View().Approvals) == 1 })
	a := s.View().Approvals[0]
	if a.Ask != AskQuestion {
		t.Fatalf("ask = %q", a.Ask)
	}
	if err := h.m.AnswerAsk(v.ID, a.RequestID, true, nil, ""); err == nil {
		t.Error("allowing a question with no answers should be refused")
	}
	answers := map[string]string{"Which city do you prefer?": "Porto", "Which meals would you like to plan?": "Breakfast, Lunch"}
	if err := h.m.AnswerAsk(v.ID, a.RequestID, true, answers, ""); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(v.ID)
	got := lastControl(h)
	in, _ := json.Marshal(got["updatedInput"])
	if got["behavior"] != "allow" || !strings.Contains(string(in), `"answers":{"Which city do you prefer?":"Porto","Which meals would you like to plan?":"Breakfast, Lunch"}`) ||
		!strings.Contains(string(in), `"questions":[`) {
		t.Errorf("reply = %v", got)
	}
}

// A plan is approved, or sent back with what to change (fixture
// exit-plan-mode: the model plans again and asks again).
func TestAnsweringAPlan(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	plan := map[string]any{"plan": "# Plan\n\n1. Read notes.txt\n2. Add the date"}
	h.rt.turns = [][]byte{
		seg(toolUseLine("toolu_p1", "ExitPlanMode", plan), askLine("req_p1", "toolu_p1", "ExitPlanMode", plan)),
		seg(toolResultLine("toolu_p1", "changes wanted", true), toolUseLine("toolu_p2", "ExitPlanMode", plan), askLine("req_p2", "toolu_p2", "ExitPlanMode", plan)),
		seg(toolResultLine("toolu_p2", "approved", false), scriptedResult),
	}
	v, _ := h.m.Create("code", t.TempDir(), "")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "plan it")
	waitUntil(t, "the plan", func() bool { return len(s.View().Approvals) == 1 })
	if a := s.View().Approvals[0]; a.Ask != AskPlan {
		t.Fatalf("ask = %q", a.Ask)
	}
	if err := h.m.AnswerAsk(v.ID, "req_p1", false, nil, "Also say which file you'd read first."); err != nil {
		t.Fatal(err)
	}
	if got := lastControl(h); got["behavior"] != "deny" || !strings.Contains(got["message"].(string), "Also say which file you'd read first.") {
		t.Errorf("keep planning = %v", got)
	}
	waitUntil(t, "the plan again", func() bool { a := s.View().Approvals; return len(a) == 1 && a[0].RequestID == "req_p2" })
	if err := h.m.AnswerAsk(v.ID, "req_p2", true, nil, ""); err != nil {
		t.Fatal(err)
	}
	h.waitIdle(v.ID)
	if got := lastControl(h); got["behavior"] != "allow" {
		t.Errorf("approve = %v", got)
	}

	// An ordinary tool use isn't answered this way.
	if err := h.m.AnswerAsk(v.ID, "nope", true, nil, ""); err == nil {
		t.Error("only a waiting question or plan can be answered")
	}
}
