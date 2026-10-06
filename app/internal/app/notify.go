package app

import (
	"strings"
	"sync"
	"testing"

	"uncli/internal/core"
	"uncli/internal/notify"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Desktop notifications: when a session the user isn't looking at (another
// session is shown, or UNCLI's window isn't focused) finishes a turn, stops
// with an error, or starts waiting on an approval card. Requests answered
// without the user never get that far, and a turn the user stopped says
// nothing. The Notifications preference picks all of them, only approvals,
// or none.

// Notification settings.
const (
	NotifyAll       = "all"
	NotifyApprovals = "approvals"
	NotifyOff       = "off"
)

func validNotify(v string) bool { return v == NotifyAll || v == NotifyApprovals || v == NotifyOff }

// EvtNotifyOpen asks the UI to show a session: the user clicked its notification.
const EvtNotifyOpen = "notify:open" // string: the session id ("" for the app's tray icon)

// attention watches what sessions do on their way to the UI and decides
// when that deserves a notification. It is the session Sink, passing
// everything on to the next one.
type attention struct {
	next session.Sink
	// focused says whether the user is looking at a session; setting is
	// the Notifications preference; show displays a note.
	focused func(id string) bool
	setting func() string
	show    func(notify.Note)

	mu    sync.Mutex
	state map[string]session.State
	page  map[string]store.Page // each session's latest page, as it was last saved
}

func newAttention(next session.Sink, focused func(string) bool, setting func() string, show func(notify.Note)) *attention {
	return &attention{next: next, focused: focused, setting: setting, show: show,
		state: map[string]session.State{}, page: map[string]store.Page{}}
}

func (a *attention) SessionEvent(id string, ev core.Event) { a.next.SessionEvent(id, ev) }

func (a *attention) PageChanged(p store.Page) {
	a.mu.Lock()
	if cur, ok := a.page[p.SessionID]; !ok || p.Seq >= cur.Seq {
		a.page[p.SessionID] = p
	}
	a.mu.Unlock()
	a.next.PageChanged(p)
}

func (a *attention) SessionChanged(v session.View) {
	a.mu.Lock()
	prev, seen := a.state[v.ID]
	a.state[v.ID] = v.State
	page := a.page[v.ID]
	a.mu.Unlock()
	a.next.SessionChanged(v)
	if !seen || prev == v.State {
		return
	}
	if n, ok := noteFor(prev, v, page, a.setting()); ok && !a.focused(v.ID) {
		a.show(n)
	}
}

// busy states are those of a turn in flight.
func busy(s session.State) bool {
	switch s {
	case session.Starting, session.Thinking, session.Writing, session.RunningTools, session.NeedsApproval:
		return true
	}
	return false
}

// noteFor says what, if anything, a session's change of state from prev
// deserves under the setting. page is its latest page.
func noteFor(prev session.State, v session.View, page store.Page, setting string) (notify.Note, bool) {
	if setting == NotifyOff || !validNotify(setting) {
		return notify.Note{}, false
	}
	n := notify.Note{Key: v.ID, Title: v.Title}
	if n.Title == "" {
		n.Title = "Untitled session"
	}
	switch {
	case v.State == session.NeedsApproval:
		n.Body = "Needs your approval"
		if len(v.Approvals) > 0 {
			ap := v.Approvals[len(v.Approvals)-1]
			what := ap.Description
			if what == "" {
				what = ap.Tool
			}
			if what = oneLine(what); what != "" {
				n.Body += ": " + what
			}
		}
		return n, true
	case setting != NotifyAll || !busy(prev):
		return notify.Note{}, false
	case v.State == session.Unread && page.SessionID == v.ID && page.Status == "done":
		n.Body = "Finished"
		if a := oneLine(plainStart(page.AnswerMD)); a != "" {
			n.Body += ": " + a
		}
		return n, true
	case v.State == session.Errored:
		n.Body = "Stopped with an error"
		if e := oneLine(v.Error); e != "" {
			n.Body += ": " + e
		}
		return n, true
	}
	return notify.Note{}, false
}

// plainStart is the start of an answer without the markdown that would
// read as clutter in a notification: heading and list marks, emphasis,
// code fences.
func plainStart(md string) string {
	var out []string
	fenced := false
	for _, line := range strings.Split(md, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
			fenced = !fenced
			continue
		}
		if l == "" || fenced {
			continue
		}
		l = strings.TrimLeft(l, "#>-*+ ")
		l = strings.NewReplacer("**", "", "__", "", "`", "").Replace(l)
		if l != "" {
			out = append(out, l)
		}
		if len(out) == 3 {
			break
		}
	}
	return strings.Join(out, " ")
}

func oneLine(s string) string { return clip(strings.Join(strings.Fields(s), " "), 200) }

// newNotifier is the platform's notifier, except in test binaries, where a
// finished fake session mustn't pop up on the developer's desktop.
func newNotifier(onClick func(string)) notify.Notifier {
	if testing.Testing() {
		return notify.None{}
	}
	return notify.New("UNCLI", onClick)
}

// notifyClicked shows the session a notification was about.
func (s *Service) notifyClicked(sessionID string) {
	if s.ShowWindow != nil {
		s.ShowWindow()
	}
	s.emit.Emit(EvtNotifyOpen, sessionID)
}

func (s *Service) showNote(n notify.Note) {
	if s.notifier != nil {
		_ = s.notifier.Show(n) // a notification that can't be shown isn't worth an error
	}
}

// TestNotification shows a sample notification, so the user can see what
// they look like and that their system lets them through.
func (s *Service) TestNotification() error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.Show(notify.Note{Title: "UNCLI", Body: "Notifications are on. You'll see one like this when a session needs you."})
}
