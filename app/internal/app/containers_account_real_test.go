package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uncli/internal/adapter/claude"
	"uncli/internal/runtime/wsl"
	"uncli/internal/store"
)

// TestRealContainerAccount signs a container in to the user's whole account
// through UNCLI's own flow and checks a session there has the claude.ai
// connectors. A person approves the link: it's written to the file named by
// UNCLI_WSL_ACCOUNT, and the code is read from the same name with .code.
// The sign-in lives in the test's temporary config folder only.
func TestRealContainerAccount(t *testing.T) {
	exchange := os.Getenv("UNCLI_WSL_ACCOUNT")
	if exchange == "" {
		t.Skip("set UNCLI_WSL_ACCOUNT to a file to exchange the link and code through")
	}
	dir := t.TempDir()
	cache, _ := os.UserCacheDir()
	cfg := filepath.Join(dir, "c")
	_ = os.MkdirAll(cfg, 0o755)
	_ = os.WriteFile(filepath.Join(cfg, "containers.yaml"), []byte("containers:\n  - id: it-acct\n    label: Test\n    base: alpine-3.24\n    brain: shared\n    mcp: shared\n    connectors: shared\n    packages: []\n"), 0o644)
	svc, err := New(Paths{Config: cfg, Cache: filepath.Join(cache, "uncli"), Scratch: filepath.Join(dir, "s")}, &recEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		svc.Close()
		_ = wsl.Remove(context.Background(), wsl.DistroName("it-acct"))
	})
	if err := svc.BuildContainer("it-acct"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		svc.containers.mu.Lock()
		step, failed := svc.containers.building["it-acct"], svc.containers.failed["it-acct"]
		svc.containers.mu.Unlock()
		if failed != "" {
			t.Fatal(failed)
		}
		if step == "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	link, err := svc.StartContainerAccountSignIn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(exchange + ".code")
	if err := os.WriteFile(exchange, []byte(link), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("sign-in link written; waiting for the code")
	var code []byte
	for wait := time.Now().Add(55 * time.Minute); time.Now().Before(wait); time.Sleep(time.Second) {
		if code, err = os.ReadFile(exchange + ".code"); err == nil {
			break
		}
	}
	if len(code) == 0 {
		t.Fatal("no code came")
	}
	_ = os.Remove(exchange + ".code")
	if err := svc.FinishContainerAccountSignIn(string(code)); err != nil {
		t.Fatal(err)
	}
	if !svc.Containers(context.Background()).AccountSignedIn {
		t.Fatal("not signed in after the code")
	}

	folder := filepath.Join(dir, "work")
	_ = os.MkdirAll(folder, 0o755)
	// A shared brain creates the folder's memory folder in the user's own
	// ~/.claude/projects: remove it with the test.
	if home, err := os.UserHomeDir(); err == nil {
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(home, ".claude", "projects", claude.ProjectDir(folder))) })
	}
	v, _, err := svc.CreateSessionIn("cowork", folder, "haiku", "it-acct")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(context.Background(), v.ID, "List the names of your MCP servers (not tools), comma separated, or say NONE.", nil); err != nil {
		t.Fatal(err)
	}
	var page store.Page
	for end := time.Now().Add(3 * time.Minute); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
		ps, _ := svc.Sessions.Pages(v.ID)
		if len(ps) == 1 && ps[0].Status != "open" {
			page = ps[0]
			break
		}
	}
	t.Logf("answer: %q", page.AnswerMD)
	if page.Status != "done" || !strings.Contains(page.AnswerMD, "claude.ai") {
		vw, _ := svc.Sessions.View(v.ID)
		t.Errorf("no connectors: %s %q (session error %q)", page.Status, page.AnswerMD, vw.Error)
	}
	if err := svc.SignOutContainerAccount(); err != nil {
		t.Error(err)
	}
}
