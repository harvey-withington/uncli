package app

import (
	"encoding/json"
	"testing"

	"github.com/harvey-withington/uncli/app/internal/notify"
	"github.com/harvey-withington/uncli/app/internal/session"
	"github.com/harvey-withington/uncli/app/internal/store"
)

// A paired server's session waiting on an approval is a desktop note,
// keyed as the window names the session, unless it's the one on screen.
func TestPeerNotes(t *testing.T) {
	var shown []notify.Note
	focused := ""
	p := newPeerNotes("workshop:9871", func(id string) bool { return id == focused }, func() string { return NotifyAll },
		func(n notify.Note) { shown = append(shown, n) })
	send := func(state session.State) {
		raw, _ := json.Marshal(session.View{Session: store.Session{ID: "s1", Title: "Nightly build"}, State: state})
		p.Event("session:changed", raw)
	}
	send(session.Idle)
	send(session.Thinking)
	send(session.NeedsApproval)
	if len(shown) != 1 || shown[0].Key != "workshop:9871~s1" || shown[0].Title != "Nightly build" {
		t.Fatalf("shown = %+v", shown)
	}
	focused = "workshop:9871~s1"
	send(session.Thinking)
	send(session.NeedsApproval)
	if len(shown) != 1 {
		t.Errorf("a note for the session on screen: %+v", shown)
	}
	p.Event("cli:status", json.RawMessage(`{}`)) // anything else is ignored
}
