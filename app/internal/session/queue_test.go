package session

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A message queued while an answer runs goes when the answer finishes;
// queued again, the two go as one. After Stop it's held until the user
// sends it, and it can be taken back to edit.
func TestQueue(t *testing.T) {
	h := newHarness(t, "single-turn")
	h.m.InterruptGrace = 50 * time.Millisecond // the fake CLI never answers an interrupt
	answer := h.rt.turns[0]
	v, _ := h.m.Create("chat", "", "haiku")
	ctx := context.Background()
	queued := func() *QueuedView { w, _ := h.m.View(v.ID); return w.Queued }
	sent := func(text string) bool { return h.written(`"type":"user"`, text) }

	// An answer under way, which the fake CLI holds until told.
	h.rt.mu.Lock()
	h.rt.next = len(h.rt.turns)
	h.rt.mu.Unlock()
	if err := h.m.Send(ctx, v.ID, "Write a story"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Queue(ctx, v.ID, Queued{Text: "Make it shorter"}); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Queue(ctx, v.ID, Queued{Text: "And funnier"}); err != nil {
		t.Fatal(err)
	}
	if q := queued(); q == nil || q.Text != "Make it shorter\n\nAnd funnier" || q.Held {
		t.Fatalf("queued = %+v", q)
	}
	if sent("Make it shorter") {
		t.Fatal("sent while the answer was running")
	}

	// The answer finishes: the queue goes, as one message.
	h.rt.mu.Lock()
	p := h.rt.procs[len(h.rt.procs)-1]
	h.rt.mu.Unlock()
	go func() { _, _ = p.outW.Write(answer) }()
	waitUntil(t, "the queued message sent", func() bool { return sent(`Make it shorter\n\nAnd funnier`) })
	if q := queued(); q != nil {
		t.Errorf("still queued: %+v", q)
	}

	// Stop: what's queued meanwhile is held, not sent.
	if err := h.m.Queue(ctx, v.ID, Queued{Text: "Now a poem"}); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Interrupt(v.ID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the stop", func() bool { w, _ := h.m.View(v.ID); return !w.Busy })
	time.Sleep(50 * time.Millisecond)
	if q := queued(); q == nil || !q.Held || sent("Now a poem") {
		t.Fatalf("after Stop: queued = %+v, sent = %v", q, sent("Now a poem"))
	}
	if err := h.m.SendQueued(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the held message sent", func() bool { return sent("Now a poem") })

	// Taken back to edit: off the queue, text and all.
	if err := h.m.Queue(ctx, v.ID, Queued{Text: "Rhyming, please"}); err != nil {
		t.Fatal(err)
	}
	q, err := h.m.Unqueue(v.ID)
	if err != nil || q == nil || q.Text != "Rhyming, please" || queued() != nil {
		t.Errorf("unqueued = %+v, %v; left %+v", q, err, queued())
	}
	if strings.Contains(strings.Join(h.rt.lines(), "\n"), "Rhyming") {
		t.Error("a message taken back was sent")
	}
}
