package session

import (
	"context"
	"slices"
	"testing"
	"time"
)

// busyWith starts a turn the fake CLI never answers: a turn in progress.
func busyWith(t *testing.T, h *harness, id string) {
	t.Helper()
	h.rt.mu.Lock()
	h.rt.hangLast = true
	h.rt.next = len(h.rt.turns) // no answer left to give
	h.rt.mu.Unlock()
	if err := h.m.Send(context.Background(), id, "Second"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "a turn in progress", func() bool { v, _ := h.m.View(id); return v.Busy })
}

// Turns UNCLI's stopping cut off are told apart from turns the user
// stopped: quitting notes them, a crash leaves their pages open, and the
// next start reads both, once.
func TestCutOffTurns(t *testing.T) {
	dir := t.TempDir()
	h := openHarness(t, dir, "single-turn")
	cut, _ := h.m.Create("chat", "", "haiku")
	stopped, _ := h.m.Create("chat", "", "haiku")
	idle, _ := h.m.Create("chat", "", "haiku")
	h.send(idle.ID, "First")
	busyWith(t, h, cut.ID)
	_ = stopped
	h.m.Close()
	h.db.Close()

	h2 := openHarness(t, dir, "single-turn")
	if got := h2.m.CutOff(); !slices.Equal(got, []string{cut.ID}) {
		t.Fatalf("cut off = %v, want only %s", got, cut.ID)
	}
	pages, _ := h2.m.Pages(cut.ID)
	if p := pages[len(pages)-1]; p.Status != "interrupted" {
		t.Errorf("the cut-off page is %q", p.Status)
	}

	// A turn the user stops isn't one; read once, so a quiet restart has none.
	busyWith(t, h2, stopped.ID)
	h2.m.InterruptGrace = 50 * time.Millisecond // the fake CLI never answers the interrupt
	if err := h2.m.Interrupt(stopped.ID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the stop", func() bool { v, _ := h2.m.View(stopped.ID); return !v.Busy })
	h2.m.Close()
	h2.db.Close()
	h3 := openHarness(t, dir, "single-turn")
	if got := h3.m.CutOff(); len(got) != 0 {
		t.Errorf("after a quiet restart, cut off = %v", got)
	}

	// A crash: the page is left open, with nothing noted.
	busyWith(t, h3, idle.ID)
	h4 := openHarness(t, dir, "single-turn") // the same database, as the next start finds it
	if got := h4.m.CutOff(); !slices.Equal(got, []string{idle.ID}) {
		t.Errorf("after a crash, cut off = %v", got)
	}
}
