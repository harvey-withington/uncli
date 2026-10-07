package session

import (
	"context"
	"strings"
	"testing"

	"uncli/internal/core"
)

// otherRuntime is the fake runtime under another id (a container).
type otherRuntime struct{ *fakeRuntime }

func (otherRuntime) ID() string { return "wsl" }

// placeIn moves a new session to another runtime, as creating it in a
// container profile will.
func placeIn(t *testing.T, h *harness, id, runtime, ref string) {
	t.Helper()
	s, err := h.m.get(id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.rec.Runtime, s.rec.RuntimeRef = runtime, ref
	s.mu.Unlock()
}

// A session starts in its own runtime, found by its id and ref.
func TestSessionStartsInItsRuntime(t *testing.T) {
	h := newHarness(t, "single-turn")
	var asked []string
	h.m.d.Runtimes = func(id, ref string) (core.Runtime, error) {
		asked = append(asked, id+"/"+ref)
		return otherRuntime{h.rt}, nil
	}
	v, _ := h.m.Create("chat", "", "haiku")
	placeIn(t, h, v.ID, "wsl", "go-dev")
	pages := h.send(v.ID, "hi")
	if len(asked) != 1 || asked[0] != "wsl/go-dev" || len(pages) != 1 {
		t.Errorf("asked %v, pages %d", asked, len(pages))
	}
}

// A session in a runtime this UNCLI doesn't have says so instead of
// starting locally.
func TestUnknownRuntimeIsRefused(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	placeIn(t, h, v.ID, "wsl", "go-dev")
	err := h.m.Send(context.Background(), v.ID, "hi")
	if err == nil || !strings.Contains(err.Error(), "wsl") {
		t.Errorf("err = %v", err)
	}
	h.rt.mu.Lock()
	defer h.rt.mu.Unlock()
	if len(h.rt.starts) != 0 {
		t.Error("it started locally")
	}
}
