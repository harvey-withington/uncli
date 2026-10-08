package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uncli/internal/runtime/wsl"
	"uncli/internal/secrets"
	"uncli/internal/store"
)

// timedEmitter logs each containers update with when it came.
type timedEmitter struct {
	t     *testing.T
	start time.Time
	last  chan ContainersInfo
}

func (e *timedEmitter) Emit(name string, data any) {
	if name != EvtContainers {
		return
	}
	info := data.(ContainersInfo)
	for _, c := range info.Containers {
		if c.ID == "it-rebuild" {
			e.t.Logf("%6.1fs step=%q built=%v error=%q", time.Since(e.start).Seconds(), c.Step, c.Built, c.Error)
		}
	}
	select {
	case e.last <- info:
	default:
	}
}

// TestRealRebuild builds a container, runs a session in it (so the distro
// is up, as in use), then rebuilds it, logging every progress update.
// UNCLI_WSL_REAL=1 only.
func TestRealRebuild(t *testing.T) {
	if os.Getenv("UNCLI_WSL_REAL") != "1" {
		t.Skip("set UNCLI_WSL_REAL=1")
	}
	keyring = osSecrets{}
	t.Cleanup(func() { keyring = &memSecrets{} })
	if tok, _ := secrets.Get(secretContainerToken); tok == "" {
		t.Skip("no container token")
	}
	dir := t.TempDir()
	cache, _ := os.UserCacheDir()
	cfg := filepath.Join(dir, "c")
	_ = os.MkdirAll(cfg, 0o755)
	_ = os.WriteFile(filepath.Join(cfg, "containers.yaml"), []byte("containers:\n  - id: it-rebuild\n    label: Test\n    base: alpine-3.24\n    packages: []\n"), 0o644)
	em := &timedEmitter{t: t, start: time.Now(), last: make(chan ContainersInfo, 100)}
	svc, err := New(Paths{Config: cfg, Cache: filepath.Join(cache, "uncli"), Scratch: filepath.Join(dir, "s")}, em)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		svc.Close()
		_ = wsl.Remove(context.Background(), wsl.DistroName("it-rebuild"))
	})
	waitBuilt := func(what string) {
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			svc.containers.mu.Lock()
			step, failed := svc.containers.building["it-rebuild"], svc.containers.failed["it-rebuild"]
			svc.containers.mu.Unlock()
			if step == "" && failed != "" {
				t.Fatalf("%s failed: %s", what, failed)
			}
			if step == "" {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("%s never finished", what)
	}
	t.Log("first build")
	if err := svc.BuildContainer("it-rebuild"); err != nil {
		t.Fatal(err)
	}
	waitBuilt("build")

	folder := filepath.Join(dir, "work")
	_ = os.MkdirAll(folder, 0o755)
	v, _, err := svc.CreateSessionIn("cowork", folder, "haiku", "it-rebuild")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(context.Background(), v.ID, "Reply with exactly: READY", nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(8 * time.Second) // the session's CLI is running in the distro

	t.Log("rebuild while a session runs there")
	em.start = time.Now()
	if err := svc.BuildContainer("it-rebuild"); err != nil {
		t.Fatal(err)
	}
	waitBuilt("rebuild")
	t.Logf("rebuild took %.1fs", time.Since(em.start).Seconds())

	// The conversation outlived the rebuild: the session resumes it.
	waitIdle := func() store.Page {
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			vw, _ := svc.Sessions.View(v.ID)
			ps, _ := svc.Sessions.Pages(v.ID)
			if !vw.Busy && len(ps) > 0 && ps[len(ps)-1].Status != "open" {
				return ps[len(ps)-1]
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Fatal("the session never finished")
		return store.Page{}
	}
	if p := waitIdle(); p.Status != "done" {
		t.Fatalf("first turn: %s %q", p.Status, p.Error)
	}
	if err := svc.Send(context.Background(), v.ID, "What exact word did I ask you to reply with in my previous message? Reply with only that word.", nil); err != nil {
		t.Fatal(err)
	}
	if p := waitIdle(); p.Status != "done" || !strings.Contains(strings.ToUpper(p.AnswerMD), "READY") {
		t.Errorf("after the rebuild: %s %q %q", p.Status, p.AnswerMD, p.Error)
	}
}
