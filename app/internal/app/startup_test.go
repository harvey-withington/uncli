package app

import (
	"errors"
	"strings"
	"testing"
)

func TestStartWithWindowsPreference(t *testing.T) {
	type call struct {
		on      bool
		command string
	}
	var calls []call
	fail := false
	prev := startupEntry
	startupEntry = func(on bool, command string) error {
		if fail {
			return errors.New("access denied")
		}
		calls = append(calls, call{on, command})
		return nil
	}
	t.Cleanup(func() { startupEntry = prev })

	s := newTestService(t)
	if s.Preferences().StartWithWindows {
		t.Fatal("on by default")
	}
	p := s.Preferences()
	p.StartWithWindows = true
	if !StartupSupported {
		if _, err := s.SetPreferences(p); err == nil {
			t.Error("accepted where it isn't supported")
		}
		return
	}
	if saved, err := s.SetPreferences(p); err != nil || !saved.StartWithWindows {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	if len(calls) != 1 || !calls[0].on || !strings.HasSuffix(calls[0].command, `" `+MinimisedArg) {
		t.Fatalf("calls = %+v", calls)
	}
	// Saving another preference leaves the entry alone.
	p.Notifications = NotifyOff
	if _, err := s.SetPreferences(p); err != nil || len(calls) != 1 {
		t.Fatalf("calls = %+v, %v", calls, err)
	}
	// A failure keeps the preference as it was, with the reason.
	fail = true
	p.StartWithWindows = false
	if saved, err := s.SetPreferences(p); err == nil || !saved.StartWithWindows {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	fail = false
	if saved, err := s.SetPreferences(p); err != nil || saved.StartWithWindows || len(calls) != 2 || calls[1].on {
		t.Fatalf("saved %+v, calls %+v, %v", saved, calls, err)
	}
	// At start the entry is rewritten only while it's on, naming this copy.
	s.refreshStartup()
	if len(calls) != 2 {
		t.Fatalf("refreshed while off: %+v", calls)
	}
	p.StartWithWindows = true
	if _, err := s.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	s.refreshStartup()
	if len(calls) != 4 || !calls[3].on || !strings.HasSuffix(calls[3].command, `" `+MinimisedArg) {
		t.Fatalf("calls = %+v", calls)
	}
}
