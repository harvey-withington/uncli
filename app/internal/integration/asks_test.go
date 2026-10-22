//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uncli/internal/core"
	"uncli/internal/session"
)

// The model asks a question with its question tool; the user answers on
// the card (AnswerAsk), and the answer reaches the model, which uses it.
func TestQuestionAnswered(t *testing.T) {
	config := t.TempDir()
	os.WriteFile(filepath.Join(config, "profiles.yaml"), []byte(`profiles:
  - id: asking
    label: Asking
    folder: pick
    model: haiku
    tools: [AskUserQuestion]
    isolated: true
    permission_mode: default
`), 0o644)
	svc, _ := open(t, config)
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("asking", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Sessions.Send(ctx, v.ID, "Use the AskUserQuestion tool to ask me one question: which colour I prefer, with the options Red, Green and Blue. "+
		"Then reply with only the colour I chose, in capitals."); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Minute)
	var q session.Approval
	for time.Now().Before(deadline) && q.RequestID == "" {
		for _, a := range view(svc, v.ID).Approvals {
			if a.Ask == session.AskQuestion {
				q = a
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if q.RequestID == "" {
		t.Fatal("no question card")
	}
	question := ""
	if i := strings.Index(string(q.Input), `"question":"`); i >= 0 {
		rest := string(q.Input)[i+len(`"question":"`):]
		question = rest[:strings.Index(rest, `"`)]
	}
	if err := svc.Sessions.AnswerAsk(v.ID, q.RequestID, true, map[string]string{question: "Green"}, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if p := last(t, svc, v.ID); p.Status != "done" || !strings.Contains(p.AnswerMD, "GREEN") {
		t.Errorf("page = %s %q", p.Status, p.AnswerMD)
	}
}

// The CLI's slash commands reach the session from its handshake, without
// the ones that only work in its terminal.
func TestSlashCommands(t *testing.T) {
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Sessions.Send(ctx, v.ID, "Reply with only OK."); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	cs, err := svc.Sessions.Commands(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range cs {
		names[c.Name] = true
	}
	if !names["compact"] || !names["context"] || names["doctor"] {
		t.Errorf("commands = %v", names)
	}
	t.Logf("%d commands", len(cs))
}

// On a fresh install nothing has reported commands yet: asking for them
// starts the session's CLI (no turn), which reports them as it starts.
func TestSlashCommandsBeforeAnyTurn(t *testing.T) {
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if cs, _ := svc.Sessions.Commands(v.ID); len(cs) != 0 {
		t.Fatalf("nothing should be known yet: %d", len(cs))
	}
	deadline := time.Now().Add(time.Minute)
	var cs []core.CommandInfo
	for time.Now().Before(deadline) && len(cs) == 0 {
		time.Sleep(500 * time.Millisecond)
		cs, _ = svc.Sessions.Commands(v.ID)
	}
	if len(cs) == 0 {
		t.Fatal("the CLI never reported its commands")
	}
	if pages, _ := svc.Sessions.Pages(v.ID); len(pages) != 0 {
		t.Errorf("no turn should have been sent: %d pages", len(pages))
	}
	t.Logf("%d commands", len(cs))
}
