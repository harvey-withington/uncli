package app

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/session"
)

// An embedding host hears of each approval card once, when it starts to
// wait on the user (not while a decision model is still judging it), and of
// each turn's end; whatever the desktop's notification setting and focus.
func TestHostHearsOfCardsAndTurnEnds(t *testing.T) {
	a, _ := watch(true, NotifyOff) // focused, notifications off: the host hears anyway
	var mu sync.Mutex
	var cards []string
	var idle int
	a.onApproval = func(id string, ap session.Approval) {
		mu.Lock()
		cards = append(cards, id+":"+ap.RequestID)
		mu.Unlock()
	}
	a.onIdle = func(id string) {
		mu.Lock()
		idle++
		mu.Unlock()
	}
	a.SessionChanged(view(session.Idle))
	a.SessionChanged(view(session.Thinking))
	v := view(session.NeedsApproval)
	v.Approvals = []session.Approval{{RequestID: "r1", Tool: "Bash", Judging: true}}
	a.SessionChanged(v) // being judged: not yet the user's
	v.Approvals = []session.Approval{{RequestID: "r1", Tool: "Bash"}}
	a.SessionChanged(v)
	a.SessionChanged(v) // the same card again
	v.Approvals = append(v.Approvals, session.Approval{RequestID: "r2", Tool: "Write"})
	a.SessionChanged(v) // a second card while still waiting
	a.SessionChanged(view(session.RunningTools))
	a.SessionChanged(view(session.Unread)) // the turn ends
	a.SessionChanged(view(session.Idle))   // read: not another end
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		got, ends := append([]string(nil), cards...), idle
		mu.Unlock()
		if len(got) == 2 && ends == 1 || time.Now().After(deadline) {
			if len(got) != 2 || got[0] != "s1:r1" || got[1] != "s1:r2" || ends != 1 {
				t.Fatalf("cards = %v, ends = %d", got, ends)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// What a card asks, in words for an alert.
func TestApprovalPrompt(t *testing.T) {
	q, _ := json.Marshal(map[string]any{"questions": []any{map[string]any{"question": "Which colour?"}}})
	for _, c := range []struct {
		a             session.Approval
		title, detail string
	}{
		{session.Approval{Tool: "mcp__board__set", Host: &session.HostPrompt{Title: "Rename card 42", Detail: "New title: Melbourne"}}, "Rename card 42", "New title: Melbourne"},
		{session.Approval{Tool: "AskUserQuestion", Ask: "question", Input: q}, "A question for you", "Which colour?"},
		{session.Approval{Tool: "ExitPlanMode", Ask: "plan"}, "A plan to approve", ""},
		{session.Approval{Tool: "Bash", Description: "Push the branch", Action: core.ToolAction{Command: "git push origin main"}}, "Push the branch", "git push origin main"},
		{session.Approval{Tool: "Write", Action: core.ToolAction{Path: `C:\notes\plan.md`}}, "Write", `C:\notes\plan.md`},
	} {
		if title, detail := ApprovalPrompt(c.a); title != c.title || detail != c.detail {
			t.Errorf("%s: %q, %q; want %q, %q", c.a.Tool, title, detail, c.title, c.detail)
		}
	}
}
