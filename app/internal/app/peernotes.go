package app

import (
	"encoding/json"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/notify"
	"github.com/harvey-withington/uncli/app/internal/session"
	"github.com/harvey-withington/uncli/app/internal/store"
)

// PeerNotes gives desktop notifications for a paired server's sessions
// (decision 0031), under the same rules and the same Notifications
// preference as this PC's own. The server's events name its sessions by
// its own ids; the notes, and the focus they're checked against, use the
// window's key for them, "<server>~<id>" (lib/api/identity.ts), so a click
// opens the right session.
type PeerNotes struct {
	server string
	watch  *attention
}

// PeerSep joins a server and a session id in the window's session keys.
const PeerSep = "~"

func (s *Service) NewPeerNotes(server string) *PeerNotes {
	return newPeerNotes(server,
		func(id string) bool { return s.Sessions != nil && s.Sessions.Focused(id) },
		func() string { return s.Preferences().Notifications },
		func(n notify.Note) { go s.showNote(n) })
}

func newPeerNotes(server string, focused func(string) bool, setting func() string, show func(notify.Note)) *PeerNotes {
	return &PeerNotes{server: server, watch: newAttention(discard{}, focused, setting, show)}
}

// Event takes one event from the server's stream.
func (p *PeerNotes) Event(name string, data json.RawMessage) {
	switch name {
	case "session:changed":
		var v session.View
		if json.Unmarshal(data, &v) == nil && v.ID != "" {
			v.ID = p.server + PeerSep + v.ID
			p.watch.SessionChanged(v)
		}
	case "page:changed":
		var pg store.Page
		if json.Unmarshal(data, &pg) == nil && pg.SessionID != "" {
			pg.SessionID = p.server + PeerSep + pg.SessionID
			p.watch.PageChanged(pg)
		}
	}
}

// discard is the end of a peer's notes: its events already went to the
// window.
type discard struct{}

func (discard) SessionEvent(string, core.Event) {}
func (discard) SessionChanged(session.View)     {}
func (discard) PageChanged(store.Page)          {}
