package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"uncli/internal/runtime/wsl"
)

type recEmitter struct {
	mu   sync.Mutex
	sent []ContainersInfo
}

func (e *recEmitter) Emit(name string, data any) {
	if name != EvtContainers {
		return
	}
	e.mu.Lock()
	e.sent = append(e.sent, data.(ContainersInfo))
	e.mu.Unlock()
}

// Build progress goes out straight away and in order, with WSL as last
// asked: asking wsl.exe again would wait behind the build's own WSL work.
func TestContainerProgressDoesNotWaitForWSL(t *testing.T) {
	asked := 0
	checkWSL = func(context.Context) (wsl.Status, error) {
		asked++
		return wsl.Status{Installed: true, Version: "3.0.1.0", Distros: []string{"uncli-sandbox"}}, nil
	}
	t.Cleanup(func() { checkWSL = wsl.CheckStatus })
	dir := t.TempDir()
	rec := &recEmitter{}
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, rec)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if info := svc.Containers(context.Background()); asked != 1 || !info.Containers[0].Built {
		t.Fatalf("asked %d, info %+v", asked, info)
	}
	for _, step := range []string{"download", "import", "packages", "cli", "lockdown", "check"} {
		svc.containers.mu.Lock()
		svc.containers.building["sandbox"] = step
		svc.containers.mu.Unlock()
		svc.emitContainers()
	}
	if asked != 1 {
		t.Errorf("progress asked wsl.exe %d more times", asked-1)
	}
	var steps []string
	for _, info := range rec.sent {
		steps = append(steps, info.Containers[0].Step)
		if !info.WSL.Installed || info.WSL.Version != "3.0.1.0" {
			t.Errorf("progress lost WSL's status: %+v", info.WSL)
		}
	}
	if len(steps) != 6 || steps[0] != "download" || steps[5] != "check" {
		t.Errorf("steps = %v", steps)
	}
}

// A built container says what changed since it was built, and an edit to
// containers.yaml shows without restarting UNCLI.
func TestContainerChangesSinceBuilt(t *testing.T) {
	checkWSL = func(context.Context) (wsl.Status, error) {
		return wsl.Status{Installed: true, Version: "3.0.1.0", Distros: []string{"uncli-sandbox"}}, nil
	}
	t.Cleanup(func() { checkWSL = wsl.CheckStatus })
	dir := t.TempDir()
	cfg := filepath.Join(dir, "c")
	svc, err := New(Paths{Config: cfg, Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, &recEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	sandbox := func() ContainerInfo {
		for _, c := range svc.Containers(context.Background()).Containers {
			if c.ID == "sandbox" {
				return c
			}
		}
		t.Fatal("no sandbox")
		return ContainerInfo{}
	}
	if c := sandbox(); !slices.Equal(c.Changes, []string{ChangedUnknown}) {
		t.Errorf("no record: %v", c.Changes)
	}
	p, b, _ := svc.profileAndBase("sandbox")
	if err := svc.saveBuildRecord("sandbox", recordFor(p, b, svc.cliVersion())); err != nil {
		t.Fatal(err)
	}
	if c := sandbox(); len(c.Changes) != 0 {
		t.Errorf("as built: %v", c.Changes)
	}
	user := "containers:\n  - id: sandbox\n    label: Sandbox\n    base: alpine-3.24\n    brain: shared\n    packages: [go]\n"
	if err := os.WriteFile(filepath.Join(cfg, "containers.yaml"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := sandbox(); !slices.Equal(c.Changes, []string{ChangedPackages}) {
		t.Errorf("packages edited: %v", c.Changes)
	}
	// Another CLI version, too.
	_ = svc.Store.SetSetting(SettingCLIVersion, "9.9.9")
	if c := sandbox(); !slices.Equal(c.Changes, []string{ChangedPackages, ChangedCLI}) {
		t.Errorf("and the CLI: %v", c.Changes)
	}
}

// The full account sign-in is kept per provider in UNCLI's folder; signing
// out removes only its credentials.
func TestContainerAccount(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, &recEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if svc.containerAccount() != "" {
		t.Fatal("signed in from the start")
	}
	acct := svc.accountDir()
	if !strings.HasSuffix(acct, filepath.Join("wsl-data", "accounts", "claude")) {
		t.Errorf("account folder = %s", acct)
	}
	_ = os.MkdirAll(filepath.Join(acct, "cache"), 0o700)
	_ = os.WriteFile(filepath.Join(acct, ".credentials.json"), []byte("{}"), 0o600)
	if svc.containerAccount() != acct {
		t.Error("the sign-in wasn't seen")
	}
	if err := svc.SignOutContainerAccount(); err != nil || svc.containerAccount() != "" {
		t.Errorf("sign out: %v", err)
	}
	if _, err := os.Stat(filepath.Join(acct, "cache")); err != nil {
		t.Error("signing out removed more than the sign-in")
	}
}
