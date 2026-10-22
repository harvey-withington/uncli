package session

import (
	"context"
	"strings"
	"testing"

	"uncli/internal/core"
)

func TestSlashName(t *testing.T) {
	for in, want := range map[string]string{
		"/compact":                "compact",
		"/compact focus on tests": "compact",
		"/my-plugin:do-it now":    "my-plugin:do-it",
		"/usr/bin is a folder":    "",
		"hello /compact":          "",
		"/":                       "",
	} {
		if got := slashName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// A command that could stop the CLI asking UNCLI is never sent or offered;
// a CLI that takes no commands gets none.
func TestCheckSlash(t *testing.T) {
	takes := core.Capabilities{SlashPassthrough: true}
	if err := checkSlash("/compact", "Grok", takes); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"/always-approve on", "/yolo", "/bypass-permissions", "/auto-mode-setup"} {
		if err := checkSlash(bad, "Grok", takes); err == nil || !strings.Contains(err.Error(), "UNCLI doesn't send") {
			t.Errorf("%s: %v", bad, err)
		}
	}
	if err := checkSlash("/compact", "Antigravity CLI", core.Capabilities{}); err == nil || !strings.Contains(err.Error(), "doesn't take slash commands") {
		t.Errorf("no commands: %v", err)
	}
	if err := checkSlash("explain /usr/bin", "Antigravity CLI", core.Capabilities{}); err != nil {
		t.Errorf("not a command: %v", err)
	}
	got := usable([]core.CommandInfo{{Name: "compact"}, {Name: "always-approve"}, {Name: ""}})
	if len(got) != 1 || got[0].Name != "compact" {
		t.Errorf("usable = %+v", got)
	}
}

// A session offers what its CLI reported; one that hasn't started its CLI
// offers what that CLI reported last; an unsafe command is refused.
func TestSessionCommands(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	if got, _ := h.m.Commands(v.ID); len(got) != 0 {
		t.Errorf("nothing known yet: %+v", got)
	}
	h.send(v.ID, "hi")
	s, _ := h.m.get(v.ID)
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	s.handle(gen, core.NewEvent(core.EvCommands, core.Commands{List: []core.CommandInfo{
		{Name: "compact", Description: "Compress the conversation"}, {Name: "always-approve", Hint: "on|off"},
	}}, nil))
	if got, _ := h.m.Commands(v.ID); len(got) != 1 || got[0].Name != "compact" {
		t.Errorf("session's = %+v", got)
	}
	fresh, _ := h.m.Create("chat", "", "haiku")
	if got, _ := h.m.Commands(fresh.ID); len(got) != 1 || got[0].Description != "Compress the conversation" {
		t.Errorf("a new session gets the CLI's last: %+v", got)
	}
	if err := h.m.Send(context.Background(), v.ID, "/always-approve on"); err == nil {
		t.Error("/always-approve must never be sent")
	}
}

// Asking for a session's commands before its CLI has run starts the CLI,
// with no turn: it reports its commands as it starts.
func TestCommandsStartTheCLI(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	if _, err := h.m.Commands(v.ID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the CLI", func() bool {
		h.rt.mu.Lock()
		defer h.rt.mu.Unlock()
		return len(h.rt.starts) == 1
	})
	if pages, _ := h.m.Pages(v.ID); len(pages) != 0 {
		t.Errorf("no turn should be sent: %d pages", len(pages))
	}
	if vw, _ := h.m.View(v.ID); vw.Busy || vw.State != Starting {
		t.Errorf("warming isn't a turn: busy %v, state %s", vw.Busy, vw.State)
	}
	// The handshake's answer means it's ready: no turn will say so.
	s, _ := h.m.get(v.ID)
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	s.handle(gen, core.NewEvent(core.EvAccount, core.Account{}, nil))
	if vw, _ := h.m.View(v.ID); vw.State != Idle {
		t.Errorf("after the handshake: %s", vw.State)
	}
	// Asking again while it runs starts nothing more.
	_, _ = h.m.Commands(v.ID)
	h.rt.mu.Lock()
	defer h.rt.mu.Unlock()
	if len(h.rt.starts) != 1 {
		t.Errorf("starts = %d", len(h.rt.starts))
	}
}
