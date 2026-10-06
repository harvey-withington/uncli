package app

import (
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/core"
	"uncli/internal/notify"
	"uncli/internal/session"
	"uncli/internal/store"
)

type nullSink struct{}

func (nullSink) SessionEvent(string, core.Event) {}
func (nullSink) SessionChanged(session.View)     {}
func (nullSink) PageChanged(store.Page)          {}

// watch builds an attention sink with the given focus and setting, and
// returns the notes it shows.
func watch(focused bool, setting string) (*attention, *[]notify.Note) {
	var shown []notify.Note
	a := newAttention(nullSink{}, func(string) bool { return focused }, func() string { return setting },
		func(n notify.Note) { shown = append(shown, n) })
	return a, &shown
}

func view(state session.State) session.View {
	v := session.View{State: state}
	v.ID, v.Title = "s1", "Fix the build"
	return v
}

func TestNotifiesWhenABackgroundTurnFinishes(t *testing.T) {
	a, shown := watch(false, NotifyAll)
	a.SessionChanged(view(session.Idle))
	a.SessionChanged(view(session.Thinking))
	a.PageChanged(store.Page{SessionID: "s1", Seq: 1, Status: "done", AnswerMD: "## Done\n\nThe **build** passes now.\n\n```go\nx\n```"})
	a.SessionChanged(view(session.Unread))
	if len(*shown) != 1 {
		t.Fatalf("shown = %v", *shown)
	}
	n := (*shown)[0]
	if n.Key != "s1" || n.Title != "Fix the build" || n.Body != "Finished: Done The build passes now." {
		t.Errorf("note = %+v", n)
	}
}

func TestNotifiesWhenAnApprovalWaits(t *testing.T) {
	a, shown := watch(false, NotifyApprovals)
	a.SessionChanged(view(session.RunningTools))
	v := view(session.NeedsApproval)
	v.Approvals = []session.Approval{{Tool: "Bash", Description: "Push the branch to origin"}}
	a.SessionChanged(v)
	a.SessionChanged(v) // the same wait again: nothing new
	if len(*shown) != 1 || (*shown)[0].Body != "Needs your approval: Push the branch to origin" {
		t.Fatalf("shown = %v", *shown)
	}
	// Only approvals: a finished turn says nothing.
	a.PageChanged(store.Page{SessionID: "s1", Seq: 1, Status: "done", AnswerMD: "ok"})
	a.SessionChanged(view(session.Unread))
	if len(*shown) != 1 {
		t.Errorf("finished turn notified under approvals only: %v", *shown)
	}
}

func TestQuietWhenWatchingStoppedOrOff(t *testing.T) {
	// The user is looking at the session.
	a, shown := watch(true, NotifyAll)
	a.SessionChanged(view(session.Thinking))
	a.SessionChanged(view(session.NeedsApproval))
	if len(*shown) != 0 {
		t.Errorf("focused session notified: %v", *shown)
	}
	// A turn the user stopped.
	a, shown = watch(false, NotifyAll)
	a.SessionChanged(view(session.Writing))
	a.PageChanged(store.Page{SessionID: "s1", Seq: 2, Status: "interrupted"})
	a.SessionChanged(view(session.Unread))
	if len(*shown) != 0 {
		t.Errorf("stopped turn notified: %v", *shown)
	}
	// Off.
	a, shown = watch(false, NotifyOff)
	a.SessionChanged(view(session.Thinking))
	a.SessionChanged(view(session.NeedsApproval))
	if len(*shown) != 0 {
		t.Errorf("notified while off: %v", *shown)
	}
	// The first view of a session (at startup) is never a change.
	a, shown = watch(false, NotifyAll)
	a.SessionChanged(view(session.Unread))
	if len(*shown) != 0 {
		t.Errorf("startup notified: %v", *shown)
	}
}

func TestNotifiesAnError(t *testing.T) {
	a, shown := watch(false, NotifyAll)
	a.SessionChanged(view(session.Thinking))
	v := view(session.Errored)
	v.Error = "The CLI stopped unexpectedly:\n  out of memory"
	a.SessionChanged(v)
	if len(*shown) != 1 || !strings.HasPrefix((*shown)[0].Body, "Stopped with an error: The CLI stopped unexpectedly: out of memory") {
		t.Fatalf("shown = %v", *shown)
	}
}

func TestNotificationsPreference(t *testing.T) {
	s := newTestService(t)
	if got := s.Preferences().Notifications; got != NotifyAll {
		t.Errorf("default = %q", got)
	}
	p := s.Preferences()
	p.Notifications = NotifyApprovals
	if saved, err := s.SetPreferences(p); err != nil || saved.Notifications != NotifyApprovals {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	p.Notifications = "loud"
	if _, err := s.SetPreferences(p); err == nil {
		t.Error("an unknown setting was accepted")
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	return svc
}
