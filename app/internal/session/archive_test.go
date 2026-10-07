package session

import (
	"context"
	"strings"
	"testing"
)

func TestPinsAcrossSessions(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	p := h.send(v.ID, "hello")[0]
	got, err := h.m.SetPinned(v.ID, p.ID, true)
	if err != nil || !got.Pinned {
		t.Fatalf("pin = %+v %v", got, err)
	}
	pins, err := h.m.Pinned()
	if err != nil || len(pins) != 1 || pins[0].PageID != p.ID || pins[0].Question != "hello" || pins[0].ProfileID != "chat" {
		t.Fatalf("pinned = %+v %v", pins, err)
	}
	if _, err := h.m.SetPinned(v.ID, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if pins, _ := h.m.Pinned(); len(pins) != 0 {
		t.Errorf("still pinned: %+v", pins)
	}
}

func TestArchiveStopsAndRestores(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	p := h.send(v.ID, "hello")[0]
	_, _ = h.m.SetPinned(v.ID, p.ID, true)
	if !mustView(t, h, v.ID).Running {
		t.Fatal("the CLI should be running after a turn")
	}
	a, err := h.m.Archive(v.ID, true)
	if err != nil || !a.Archived || a.Running {
		t.Fatalf("archive = %+v %v", a, err)
	}
	if err := h.m.Send(context.Background(), v.ID, "more"); err == nil || !strings.Contains(err.Error(), "archived") {
		t.Errorf("send while archived: %v", err)
	}
	if pins, _ := h.m.Pinned(); len(pins) != 1 || !pins[0].Archived {
		t.Errorf("pinned = %+v", pins)
	}

	// After a restart the archived session is still there, still archived.
	h2 := openHarness(t, h.dir, "single-turn")
	var found bool
	for _, s := range h2.m.List() {
		if s.ID == v.ID {
			found = s.Archived
		}
	}
	if !found {
		t.Fatal("the archived session wasn't loaded")
	}
	r, err := h2.m.Archive(v.ID, false)
	if err != nil || r.Archived {
		t.Fatalf("restore = %+v %v", r, err)
	}
}

func mustView(t *testing.T, h *harness, id string) View {
	t.Helper()
	v, err := h.m.View(id)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
