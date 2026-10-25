package remote

import (
	"strconv"
	"testing"
)

func seqs(evs []Event) []uint64 {
	out := []uint64{}
	for _, e := range evs {
		out = append(out, e.Seq)
	}
	return out
}

func TestHubResumes(t *testing.T) {
	h := NewHub()
	first, c := h.Subscribe("d1", "")
	if !c.Reload {
		t.Error("a new client should load state first")
	}
	h.Unsubscribe(first)
	h.Emit("session:changed", map[string]string{"id": "a"})
	h.Emit("page:changed", map[string]string{"id": "p"})
	h.Emit("notify:open", "a") // the window's own: never sent
	h.Emit("session:event", map[string]int{"n": 3})

	// Caught up after the first event: gets the next two, in order.
	sub, c := h.Subscribe("d1", c.Cursor[:len(c.Cursor)-1]+"1")
	if c.Reload || len(c.Missed) != 2 || c.Missed[0].Name != "page:changed" || c.Missed[1].Seq != 3 {
		t.Fatalf("catch-up = %+v", c)
	}
	// The cursor stays the client's own until it has read the missed events
	// (each carries its own), so a drop partway through them loses none.
	if c.Cursor != h.epoch+":1" {
		t.Errorf("cursor = %q", c.Cursor)
	}
	h.Emit("cli:status", map[string]bool{"ok": true})
	if ev := <-sub.C; ev.Seq != 4 || ev.Name != "cli:status" || string(ev.Data) != `{"ok":true}` {
		t.Errorf("live event = %+v", ev)
	}
	h.Unsubscribe(sub)

	// Up to date: nothing missed, no reload.
	if _, c := h.Subscribe("d1", h.epoch+":4"); c.Reload || len(c.Missed) != 0 || c.Cursor != h.epoch+":4" {
		t.Errorf("up to date = %+v", c)
	}
	// Reloading: it resumes from the latest, its state fetched afresh.
	if _, c := h.Subscribe("d1", ""); !c.Reload || c.Cursor != h.epoch+":4" {
		t.Errorf("reload = %+v", c)
	}
	// Another run of UNCLI, or a number from the future: reload.
	for _, cur := range []string{"other:2", h.epoch + ":99", "garbage"} {
		if _, c := h.Subscribe("d1", cur); !c.Reload {
			t.Errorf("%q didn't reload", cur)
		}
	}
}

func TestHubReloadsWhenEventsAreGone(t *testing.T) {
	h := NewHub()
	for i := 0; i < keepEvents+10; i++ {
		h.Emit("session:event", i)
	}
	if _, c := h.Subscribe("d", h.epoch+":5"); !c.Reload {
		t.Error("events no longer held should mean a reload")
	}
	last := strconv.Itoa(keepEvents + 5)
	if _, c := h.Subscribe("d", h.epoch+":"+last); c.Reload || len(c.Missed) != 5 {
		t.Errorf("recent catch-up = reload %v, %v", c.Reload, seqs(c.Missed))
	}
}

func TestHubDropsSlowAndRevokedClients(t *testing.T) {
	h := NewHub()
	slow, _ := h.Subscribe("slow", "")
	other, _ := h.Subscribe("other", "")
	for i := 0; i < subBuffer+1; i++ {
		h.Emit("session:event", i)
		if i < subBuffer {
			<-other.C
		}
	}
	select {
	case <-slow.Done():
	default:
		t.Error("a client that fell behind wasn't dropped")
	}
	select {
	case <-other.Done():
		t.Error("a client keeping up was dropped")
	default:
	}
	h.CloseDevice("other")
	select {
	case <-other.Done():
	default:
		t.Error("revoking didn't end the device's stream")
	}
}
