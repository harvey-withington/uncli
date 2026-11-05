//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harvey-withington/uncli/app/internal/app"
)

// A message queued while a real answer runs goes when that answer ends,
// as a turn of its own, and the real CLI answers it.
func TestQueuedMessageGoesAfterTheAnswer(t *testing.T) {
	config := t.TempDir()
	os.WriteFile(filepath.Join(config, "profiles.yaml"), []byte(`profiles:
  - id: plain
    label: Plain
    folder: pick
    model: haiku
    tools: []
    isolated: true
    permission_mode: default
`), 0o644)
	svc, _ := open(t, config)
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("plain", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Sessions.Send(ctx, v.ID, "Write the numbers from one to sixty in words, one per line, and nothing else."); err != nil {
		t.Fatal(err)
	}
	waitTil(t, "the answer under way", 60*time.Second, func() bool { return view(svc, v.ID).Busy })
	if err := svc.QueueWith(ctx, v.ID, "Reply with only the word QUEUED.", nil, app.SendOptions{}); err != nil {
		t.Fatal(err)
	}
	if q := view(svc, v.ID).Queued; q == nil || q.Text != "Reply with only the word QUEUED." {
		t.Fatalf("queued = %+v", q)
	}
	waitTil(t, "the queued message sent", 3*time.Minute, func() bool { pages, _ := svc.Sessions.Pages(v.ID); return len(pages) == 2 })
	waitIdle(t, svc, v.ID)
	pages, _ := svc.Sessions.Pages(v.ID)
	if pages[0].Status != "done" || !strings.Contains(strings.ToLower(pages[0].AnswerMD), "sixty") {
		t.Errorf("the first answer: %s, %.200q", pages[0].Status, pages[0].AnswerMD)
	}
	if p := pages[1]; p.Question != "Reply with only the word QUEUED." || !strings.Contains(p.AnswerMD, "QUEUED") {
		t.Errorf("the queued turn: %q -> %q", p.Question, p.AnswerMD)
	}
	if view(svc, v.ID).Queued != nil {
		t.Error("still queued")
	}
}
