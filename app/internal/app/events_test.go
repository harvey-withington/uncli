package app

import (
	"sync"
	"testing"
	"time"

	"uncli/internal/core"
)

type recorder struct {
	mu  sync.Mutex
	got []SessionEventMsg
}

func (r *recorder) Emit(name string, data any) {
	if m, ok := data.(SessionEventMsg); ok {
		r.mu.Lock()
		r.got = append(r.got, m)
		r.mu.Unlock()
	}
}

func (r *recorder) all() []SessionEventMsg {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]SessionEventMsg(nil), r.got...)
}

func delta(seq, idx int, text string) core.Event {
	ev := core.NewEvent(core.EvTextDelta, core.TextDelta{Index: idx, Text: text}, nil)
	ev.TurnSeq = seq
	return ev
}

func TestCoalescerMergesDeltasAndKeepsOrder(t *testing.T) {
	r := &recorder{}
	c := newCoalescer(r, time.Hour) // only explicit flushes
	c.SessionEvent("s", delta(1, 1, "Hel"))
	c.SessionEvent("s", delta(1, 1, "lo"))
	c.SessionEvent("s", delta(1, 2, " again")) // new block: flush the first
	c.SessionEvent("s", core.NewEvent(core.EvTextBlock, core.TextBlock{Text: "Hello"}, nil))
	got := r.all()
	if len(got) != 3 {
		t.Fatalf("emitted %d events", len(got))
	}
	d0, _ := core.Decode[core.TextDelta](got[0].Event)
	d1, _ := core.Decode[core.TextDelta](got[1].Event)
	if d0.Text != "Hello" || d1.Text != " again" || got[2].Event.Kind != core.EvTextBlock {
		t.Errorf("got %q, %q, %s", d0.Text, d1.Text, got[2].Event.Kind)
	}
}

func TestCoalescerFlushesOnTimer(t *testing.T) {
	r := &recorder{}
	c := newCoalescer(r, 10*time.Millisecond)
	c.SessionEvent("a", delta(1, 0, "x"))
	c.SessionEvent("b", delta(1, 0, "y"))
	time.Sleep(60 * time.Millisecond)
	if n := len(r.all()); n != 2 {
		t.Errorf("emitted %d, want one per session", n)
	}
}
