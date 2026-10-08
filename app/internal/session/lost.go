package session

// A conversation the CLI was asked to resume can be gone: its files were
// deleted, or kept in a container that was rebuilt. The CLI then fails at
// once, and would on every message after. Instead the session forgets the
// conversation, says so on the page, and the next message starts a new one
// here; the pages before stay.

// lostConversation is an adapter that can tell from its CLI's stderr that
// the conversation to resume no longer exists.
type lostConversation interface {
	LostConversation(stderr string) bool
}

// LostConversationNote is what the failed page says.
const LostConversationNote = "The CLI no longer has this conversation: its files were removed (for example when a container is rebuilt). " +
	"Send your message again to carry on in a new conversation; the pages above stay here."

// lostConversationLocked handles a CLI that exited because the conversation
// it was resuming is gone; it reports whether it did. Caller holds s.mu.
func (s *Session) lostConversationLocked(stderr string) bool {
	lc, ok := s.ad().(lostConversation)
	if !ok || s.rec.ProviderSID == "" || !lc.LostConversation(stderr) {
		return false
	}
	s.rec.ProviderSID = "" // the next message starts a new conversation
	_ = s.m.d.Store.UpdateSession(&s.rec)
	if s.page != nil {
		s.closePage("error", LostConversationNote)
	} else if p := s.closed; p != nil && p.Status == "error" {
		p.Error = LostConversationNote
		_ = s.m.d.Store.SavePage(p)
		s.m.d.Sink.PageChanged(*p)
	}
	s.err, s.state = "", Idle
	s.changed()
	return true
}
