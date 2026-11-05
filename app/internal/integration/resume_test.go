//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// A turn UNCLI's quit cuts off carries on when UNCLI starts again, with
// ResumeCutOff on: the session is told why, in a turn of its own, and the
// real CLI picks the conversation up.
func TestResumeCutOffTurn(t *testing.T) {
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
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	p := svc.Preferences()
	p.ResumeCutOff = true
	if _, err := svc.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("plain", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Sessions.Send(ctx, v.ID, "Write the numbers from one to three hundred in words, one per line, and nothing else."); err != nil {
		t.Fatal(err)
	}
	waitTil(t, "the turn under way", 90*time.Second, func() bool { return view(svc, v.ID).Busy })
	time.Sleep(6 * time.Second) // into the answer, which takes longer than this
	svc.Close() // quit mid-answer

	svc2, _ := open(t, config)
	defer svc2.Close()
	if got := svc2.Sessions.CutOff(); !slices.Contains(got, v.ID) {
		t.Fatalf("cut off = %v", got)
	}
	if st := last(t, svc2, v.ID).Status; st != "interrupted" {
		t.Errorf("the cut-off page is %q", st)
	}
	svc2.ResumeCutOff()
	waitTil(t, "the session carrying on", 30*time.Second, func() bool { pages, _ := svc2.Sessions.Pages(v.ID); return len(pages) == 2 })
	waitIdle(t, svc2, v.ID)
	page := last(t, svc2, v.ID)
	if !strings.Contains(page.Question, "UNCLI restarted") || page.Status != "done" || !strings.Contains(strings.ToLower(page.AnswerMD), "hundred") {
		t.Errorf("carried on: %q, %s, %.300q", page.Question, page.Status, page.AnswerMD)
	}
}
