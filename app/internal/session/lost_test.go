package session

import (
	"os"
	"path/filepath"
	"testing"
)

// A session whose conversation the CLI no longer has (fixture resume-lost:
// its files went with a rebuilt container) isn't stuck: the page says so,
// the session is ready, and the next message starts a new conversation.
func TestLostConversationStartsAfresh(t *testing.T) {
	h := newHarness(t, "single-turn")
	lost := segments(t, "resume-lost")
	stderr, err := os.ReadFile(filepath.Join(fixtures, "resume-lost.stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := h.m.Create("chat", "", "haiku")
	s, _ := h.m.get(v.ID)
	s.mu.Lock()
	s.rec.ProviderSID = "0b0e8d7e-1111-4222-8333-944455556666" // a conversation to resume
	s.mu.Unlock()
	ok := h.rt.turns
	h.rt.turns = lost
	h.rt.fail = string(stderr)

	pages := h.send(v.ID, "hi")
	waitUntil(t, "the session ready again", func() bool { return s.View().State == Idle })
	if len(pages) != 1 || pages[0].Status != "error" || pages[0].Error != LostConversationNote {
		t.Fatalf("page = %+v", pages[0])
	}
	if vw := s.View(); vw.ProviderSID != "" || vw.Error != "" {
		t.Errorf("session = %+v", vw.Session)
	}
	if !containsArg(h.rt.starts[0].Args, "--resume") {
		t.Errorf("first start didn't resume: %v", h.rt.starts[0].Args)
	}

	// Sent again: a new conversation, answered.
	h.rt.turns, h.rt.next = ok, 0
	pages = h.send(v.ID, "hi again")
	if len(pages) != 2 || pages[1].Status != "done" {
		t.Fatalf("second page = %+v", pages[len(pages)-1])
	}
	if containsArg(h.rt.starts[1].Args, "--resume") || !containsArg(h.rt.starts[1].Args, "--session-id") {
		t.Errorf("second start = %v", h.rt.starts[1].Args)
	}
}

func containsArg(args []string, a string) bool {
	for _, x := range args {
		if x == a {
			return true
		}
	}
	return false
}
