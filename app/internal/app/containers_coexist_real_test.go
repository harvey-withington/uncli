package app

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"uncli/internal/adapter/claude"
	"uncli/internal/core"
	"uncli/internal/runtime/local"
	"uncli/internal/runtime/wsl"
	"uncli/internal/secrets"
)

// TestRealCoexist checks UNCLI leaves other WSL users alone. WSL distros
// share one VM and one connection to Windows' drives, so another tool's
// drive mounts (Docker, Podman, dev containers) must stay alive through
// UNCLI's whole container life cycle. Another distro (UNCLI_WSL_PROBE, not
// one of UNCLI's profiles) mounts a Windows folder (UNCLI_WSL_PROBE_DIR) and
// lists it every half second while UNCLI builds, uses, rebuilds, stops and
// removes a container three times. Any failed listing fails the test.
func TestRealCoexist(t *testing.T) {
	probe, folder := os.Getenv("UNCLI_WSL_PROBE"), os.Getenv("UNCLI_WSL_PROBE_DIR")
	if probe == "" || folder == "" {
		t.Skip("set UNCLI_WSL_PROBE (another distro) and UNCLI_WSL_PROBE_DIR (a Windows folder)")
	}
	keyring = osSecrets{}
	t.Cleanup(func() { keyring = &memSecrets{} })
	if tok, _ := secrets.Get(secretContainerToken); tok == "" {
		t.Skip("no container token")
	}

	// The other tool's mount, probed until the end.
	p, err := local.New().Start(context.Background(), core.Command{Path: wsl.Exe,
		Args: []string{"--distribution", probe, "--user", "root", "--exec", "sh", "-c",
			`mkdir -p /probe && (mountpoint -q /probe || mount -t drvfs "$1" /probe) && echo READY &&
			 while :; do ls /probe >/dev/null 2>&1 || echo "FAIL $(date +%T) $(ls /probe 2>&1 | head -1)"; sleep 0.5; done`, "sh", folder},
		Env: map[string]string{"WSL_UTF8": "1"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var fails []string
	ready := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(p.Stdout())
		for sc.Scan() {
			line := sc.Text()
			if line == "READY" {
				close(ready)
				continue
			}
			if strings.HasPrefix(line, "FAIL") {
				mu.Lock()
				fails = append(fails, line)
				mu.Unlock()
				t.Log("probe:", line)
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(30 * time.Second):
		t.Fatal("the probe never mounted its folder")
	}
	defer func() { _ = p.Kill() }()

	dir := t.TempDir()
	cache, _ := os.UserCacheDir()
	cfg := filepath.Join(dir, "c")
	_ = os.MkdirAll(cfg, 0o755)
	_ = os.WriteFile(filepath.Join(cfg, "containers.yaml"), []byte("containers:\n  - id: it-coexist\n    label: Test\n    base: alpine-3.24\n    brain: shared\n    mcp: shared\n    packages: []\n"), 0o644)
	svc, err := New(Paths{Config: cfg, Cache: filepath.Join(cache, "uncli"), Scratch: filepath.Join(dir, "s")}, &recEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		svc.Close()
		_ = wsl.Remove(context.Background(), wsl.DistroName("it-coexist"))
	})
	build := func(what string) {
		if err := svc.BuildContainer("it-coexist"); err != nil {
			t.Fatal(err)
		}
		for end := time.Now().Add(10 * time.Minute); time.Now().Before(end); time.Sleep(300 * time.Millisecond) {
			svc.containers.mu.Lock()
			step, failed := svc.containers.building["it-coexist"], svc.containers.failed["it-coexist"]
			svc.containers.mu.Unlock()
			if failed != "" {
				t.Fatalf("%s: %s", what, failed)
			}
			if step == "" {
				return
			}
		}
		t.Fatalf("%s never finished", what)
	}
	work := filepath.Join(dir, "work")
	_ = os.MkdirAll(work, 0o755)
	// A shared brain creates the folder's memory folder in the user's own
	// ~/.claude/projects: remove it with the test.
	if home, err := os.UserHomeDir(); err == nil {
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(home, ".claude", "projects", claude.ProjectDir(work))) })
	}
	ctx := context.Background()
	for cycle := 1; cycle <= 3; cycle++ {
		t.Logf("cycle %d: build", cycle)
		build("build")
		v, _, err := svc.CreateSessionIn("cowork", work, "haiku", "it-coexist")
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.Send(ctx, v.ID, "Reply with exactly: OK", nil); err != nil {
			t.Fatal(err)
		}
		for end := time.Now().Add(2 * time.Minute); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
			if vw, _ := svc.Sessions.View(v.ID); !vw.Busy {
				break
			}
		}
		t.Logf("cycle %d: rebuild with the session there", cycle)
		build("rebuild")
		t.Logf("cycle %d: stop UNCLI's containers, then remove", cycle)
		if err := svc.StopContainers(ctx); err != nil {
			t.Error(err)
		}
		if err := svc.RemoveContainer(ctx, "it-coexist"); err != nil {
			t.Error(err)
		}
	}
	time.Sleep(3 * time.Second) // a late failure still shows

	mu.Lock()
	defer mu.Unlock()
	if len(fails) > 0 {
		t.Errorf("the other distro's mount failed %d times, first: %s", len(fails), fails[0])
	}
	out, err := local.Run(ctx, core.Command{Path: wsl.Exe, Args: []string{"--distribution", probe, "--user", "root", "--exec", "ls", "/probe"}, Env: map[string]string{"WSL_UTF8": "1"}})
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		t.Errorf("the other distro's mount is dead after the cycles: %v %s", err, out)
	}
}
