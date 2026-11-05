package session

import "testing"

// A session named at creation keeps its name through its first question,
// which only names a session that has none; blank is none. The user can
// still rename it.
func TestTitleFromTheStart(t *testing.T) {
	h := newHarness(t, "single-turn")
	named, err := h.m.CreateWith(NewSession{Profile: "chat", Model: "haiku", Title: "  Trip to Lisbon "})
	if err != nil {
		t.Fatal(err)
	}
	if named.Title != "Trip to Lisbon" {
		t.Errorf("created as %q", named.Title)
	}
	h.send(named.ID, "What should we pack for a weekend away?")
	if v, _ := h.m.View(named.ID); v.Title != "Trip to Lisbon" {
		t.Errorf("after the first question: %q", v.Title)
	}
	if v, err := h.m.Rename(named.ID, "Lisbon, December"); err != nil || v.Title != "Lisbon, December" {
		t.Errorf("renamed: %q, %v", v.Title, err)
	}

	blank, _ := h.m.CreateWith(NewSession{Profile: "chat", Model: "haiku", Title: "   "})
	if blank.Title != "" {
		t.Errorf("a blank title made %q", blank.Title)
	}
	h.rt.mu.Lock()
	h.rt.next = 0 // the recorded turn again, for this session
	h.rt.mu.Unlock()
	h.send(blank.ID, "What should we pack for a weekend away?")
	if v, _ := h.m.View(blank.ID); v.Title != "What should we pack for a weekend away?" {
		t.Errorf("named by its first question: %q", v.Title)
	}
}
