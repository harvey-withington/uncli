//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uncli/internal/app"
	"uncli/internal/session"
)

// Grok Build as a provider plugin speaking ACP (decisions 0013, 0014),
// through the app against the real CLI: the pinned download, a device
// sign-in's link and code, then a session with approvals, a stop and a
// resume. Signing in takes a person, so the session runs on a sign-in made
// beforehand: UNCLI_GROK_AUTH names a Grok auth.json to copy into the
// test's own Grok folder.

func grokService(t *testing.T, config string) *app.Service {
	t.Helper()
	dir := filepath.Join(config, "providers", "grok")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../testdata/providers/grok/provider.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "provider.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, _ := open(t, config)
	return svc
}

func TestGrokAsAPlugin(t *testing.T) {
	auth := os.Getenv("UNCLI_GROK_AUTH")
	if auth == "" {
		t.Skip("set UNCLI_GROK_AUTH to a signed-in Grok auth.json")
	}
	config := t.TempDir()
	svc := grokService(t, config)
	var hash string
	for _, p := range svc.Providers() {
		if p.ID == "grok" && p.Plugin != nil {
			if p.Plugin.Error != "" {
				t.Fatalf("plugin: %s", p.Plugin.Error)
			}
			hash = p.Plugin.Hash
		}
	}
	if err := svc.EnablePlugin("grok", hash); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, err := svc.InstallProvider(ctx, "grok")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !st.Installed || st.Version != "1.0.50" || st.LoggedIn {
		t.Fatalf("after install: %+v", st)
	}

	// A device sign-in shows its link and code (left unapproved).
	d, err := svc.StartDeviceSignIn(ctx, "grok")
	if err != nil || !strings.HasPrefix(d.URL, "https://accounts.x.ai/") || len(d.Code) != 9 {
		t.Fatalf("device sign-in = %+v, %v", d, err)
	}
	svc.CancelDeviceSignIn("grok")

	// Signed in with the sign-in made beforehand.
	b, err := os.ReadFile(auth)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "provider-state", "grok", "auth.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if st = svc.ProviderStatus(ctx, "grok", true); !st.LoggedIn || len(st.Models) == 0 {
		t.Fatalf("signed in: %+v", st)
	}
	t.Logf("Grok Build %s signed in, models %v", st.Version, st.Models[0].Value)

	work := t.TempDir()
	v, _, err := svc.CreateSessionWith(session.NewSession{Profile: "code", Workdir: work, Provider: "grok"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sessions.SetMode(v.ID, session.ModeAlways); err != nil {
		t.Fatal(err)
	}
	card := func(what string) session.Approval {
		return waitFor(t, svc, v.ID, func(v session.View) bool { return len(v.Approvals) > 0 }, what).Approvals[0]
	}

	// A write waits on a card; allowed, it happens, with its usage.
	go func() {
		_ = svc.Sessions.Send(ctx, v.ID, "Create a file hello.txt in the current folder containing exactly: hi from grok. Then reply DONE.")
	}()
	c := card("a card for the write")
	if !strings.HasSuffix(c.Action.Path, "hello.txt") {
		t.Errorf("card = %+v", c)
	}
	if err := svc.Sessions.Answer(v.ID, c.RequestID, session.Allow, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if b, err := os.ReadFile(filepath.Join(work, "hello.txt")); err != nil || !strings.Contains(string(b), "hi from grok") {
		t.Fatalf("hello.txt = %q, %v", b, err)
	}
	p := last(t, svc, v.ID)
	if p.Status != "done" || p.InputTokens == 0 || p.CostUSD <= 0 || len(p.TouchedFiles) == 0 {
		t.Errorf("page: status %s, tokens %d, cost %v, files %+v", p.Status, p.InputTokens, p.CostUSD, p.TouchedFiles)
	}

	// A delete waits on a card; denied, it doesn't happen.
	go func() {
		_ = svc.Sessions.Send(ctx, v.ID, "Run this shell command: Remove-Item hello.txt . Then say what happened.")
	}()
	c = card("a card for the delete")
	if c.Action.Kind != "shell" || !strings.Contains(c.Action.Command, "Remove-Item") {
		t.Errorf("card = %+v", c)
	}
	if err := svc.Sessions.Answer(v.ID, c.RequestID, session.Deny, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if _, err := os.Stat(filepath.Join(work, "hello.txt")); err != nil {
		t.Errorf("hello.txt was deleted after a denial: %v", err)
	}

	// Stop mid-answer.
	go func() {
		_ = svc.Sessions.Send(ctx, v.ID, "Write a 500-word story about a lighthouse keeper. Use no tools.")
	}()
	waitFor(t, svc, v.ID, func(v session.View) bool { return v.Busy }, "the story to start")
	time.Sleep(4 * time.Second)
	if err := svc.Sessions.Interrupt(v.ID); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if p := last(t, svc, v.ID); p.Status != "interrupted" {
		t.Errorf("stopped turn = %s", p.Status)
	}

	// After a restart, the conversation resumes.
	svc.Close()
	svc, _ = open(t, config)
	defer svc.Close()
	p = send(t, svc, v.ID, "What is the name of the file you created earlier? Reply with just the file name.")
	if !strings.Contains(strings.ToLower(p.AnswerMD), "hello.txt") {
		t.Errorf("after a restart: %q", p.AnswerMD)
	}
}
