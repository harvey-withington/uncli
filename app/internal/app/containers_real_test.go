package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uncli/internal/adapter/claude"
	"uncli/internal/runtime/wsl"
	"uncli/internal/secrets"
	"uncli/internal/session"
	"uncli/internal/store"
)

// TestRealContainerSession goes the whole way with real WSL: a container
// profile from containers.yaml is built (root filesystem and Linux CLI
// downloaded into UNCLI's own cache), a Co-work session is created in it
// and one turn reads a file from the session's folder. It needs WSL and a
// container token in the credential store, uses a little of the account's
// usage, and runs only with UNCLI_WSL_REAL=1.
func TestRealContainerSession(t *testing.T) {
	if os.Getenv("UNCLI_WSL_REAL") != "1" {
		t.Skip("set UNCLI_WSL_REAL=1 to build a real container")
	}
	keyring = osSecrets{} // the real token
	t.Cleanup(func() { keyring = &memSecrets{} })
	if tok, _ := secrets.Get(secretContainerToken); tok == "" {
		t.Skip("no container token in the credential store")
	}
	dir := t.TempDir()
	cache, _ := os.UserCacheDir()
	cfg := filepath.Join(dir, "c")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	// A folder of its own, read-only (decision 0017).
	extra := filepath.Join(dir, "reference")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(extra, "guide.md"), []byte("read me\n"), 0o644)
	yml := "containers:\n  - id: it-app\n    label: Test\n    base: alpine-3.24\n    brain: shared\n    mcp: shared\n    packages: []\n" +
		"    folders:\n      - {source: '" + extra + "', name: reference}\n"
	if err := os.WriteFile(filepath.Join(cfg, "containers.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := New(Paths{Config: cfg, Cache: filepath.Join(cache, "uncli"), Scratch: filepath.Join(dir, "s")}, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		svc.Close()
		_ = wsl.Remove(context.Background(), wsl.DistroName("it-app"))
	})
	if err := svc.BuildContainer("it-app"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Minute)
	var info ContainerInfo
	for time.Now().Before(deadline) {
		for _, c := range svc.Containers(context.Background()).Containers {
			if c.ID == "it-app" {
				info = c
			}
		}
		if info.Step == "" && (info.Built || info.Error != "") {
			break
		}
		time.Sleep(time.Second)
	}
	if !info.Built {
		t.Fatalf("build: %+v", info)
	}

	folder := filepath.Join(dir, "notes")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "colour.txt"), []byte("The colour is VERMILION.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, _, err := svc.CreateSessionIn("cowork", folder, "haiku", "it-app")
	if err != nil {
		t.Fatal(err)
	}
	if v.Runtime != "wsl" || v.RuntimeRef != "it-app" {
		t.Fatalf("session = %+v", v.Session)
	}
	if err := svc.Send(context.Background(), v.ID, "Read colour.txt and reply with only the colour.", nil); err != nil {
		t.Fatal(err)
	}
	var last store.Page
	for time.Now().Before(deadline) {
		ps, _ := svc.Sessions.Pages(v.ID)
		if len(ps) > 0 && ps[len(ps)-1].Status != "open" {
			last = ps[len(ps)-1]
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if last.Status != "done" || !strings.Contains(strings.ToUpper(last.AnswerMD), "VERMILION") {
		vw, _ := svc.Sessions.View(v.ID)
		t.Fatalf("page = %s %q, session error %q", last.Status, last.AnswerMD, vw.Error)
	}

	// The model was told where it runs, and the session's mounts are listed
	// where it can read them but not change them; the folder of its own is
	// there, read-only.
	if !strings.Contains(last.Directives, "Linux container") || !strings.Contains(last.Directives, wsl.ManifestPath) {
		t.Errorf("not told about the container: %q", last.Directives)
	}
	sh := func(script string) string {
		out, _ := exec.Command(wsl.Exe, "--distribution", wsl.DistroName("it-app"), "--user", "uncli", "--exec", "sh", "-c", script).CombinedOutput()
		return string(out)
	}
	if out := sh(`cat /etc/uncli/mounts.json; (echo x >> /etc/uncli/mounts.json) 2>/dev/null && echo MANIFEST-WRITABLE`); !strings.Contains(out, `"name": "reference"`) ||
		!strings.Contains(out, `"access": "ro"`) || !strings.Contains(out, `"name": "project"`) || strings.Contains(out, "MANIFEST-WRITABLE") {
		t.Errorf("manifest: %s", out)
	}
	if out := sh(`cat ~/reference/guide.md; (echo x > ~/reference/new) 2>/dev/null && echo WRITE-OK || echo refused`); !strings.Contains(out, "read me") || !strings.Contains(out, "refused") {
		t.Errorf("reference folder: %s", out)
	}

	// The shell tool works (it needs bash), so git does.
	if err := svc.Send(context.Background(), v.ID, "Use the Bash tool to run: git --version. Reply with only what it printed.", nil); err != nil {
		t.Fatal(err)
	}
	last = store.Page{}
	for time.Now().Before(deadline) {
		ps, _ := svc.Sessions.Pages(v.ID)
		if len(ps) == 2 && ps[1].Status != "open" {
			last = ps[1]
			break
		}
		time.Sleep(500 * time.Millisecond)
		for _, a := range func() []session.Approval { vw, _ := svc.Sessions.View(v.ID); return vw.Approvals }() {
			_ = svc.Sessions.Answer(v.ID, a.RequestID, session.Allow, "")
		}
	}
	if !strings.Contains(last.AnswerMD, "git version") {
		t.Errorf("git: %s %q", last.Status, last.AnswerMD)
	}
	// Co-work keeps artifacts: the CLI is told where, at its own path.
	if err := svc.Send(context.Background(), v.ID, "Use the Bash tool to run: echo $UNCLI_ARTIFACTS_DIR. Reply with only what it printed.", nil); err != nil {
		t.Fatal(err)
	}
	last = store.Page{}
	for time.Now().Before(deadline) {
		ps, _ := svc.Sessions.Pages(v.ID)
		if len(ps) == 3 && ps[2].Status != "open" {
			last = ps[2]
			break
		}
		time.Sleep(500 * time.Millisecond)
		for _, a := range func() []session.Approval { vw, _ := svc.Sessions.View(v.ID); return vw.Approvals }() {
			_ = svc.Sessions.Answer(v.ID, a.RequestID, session.Allow, "")
		}
	}
	if want := wsl.MountPoint(folder) + "/artifacts"; !strings.Contains(last.AnswerMD, want) {
		t.Errorf("artifacts folder: %q, want %s", last.AnswerMD, want)
	}

	// The shared brain: the project's memory folder is the user's, mounted
	// at the name the container's path gives it. Created for this test, so
	// removed after.
	home, _ := os.UserHomeDir()
	memory := filepath.Join(home, ".claude", "projects", claude.ProjectDir(folder), "memory")
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(memory)) })
	if st, err := os.Stat(memory); err != nil || !st.IsDir() {
		t.Errorf("no memory folder on Windows: %v", err)
	}
	inside := "/home/uncli/.claude/projects/" + claude.ProjectDir(wsl.MountPoint(folder)) + "/memory"
	if out, err := exec.Command(wsl.Exe, "--distribution", wsl.DistroName("it-app"), "--user", "root", "--exec", "mountpoint", "-q", inside).CombinedOutput(); err != nil {
		t.Errorf("memory not mounted at %s: %v %s", inside, err, out)
	}

	// Shared MCP servers: the names only, so no URL or token is printed.
	b, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	want, _ := claude.ContainerMCP(b, folder)
	got, _ := exec.Command(wsl.Exe, "--distribution", wsl.DistroName("it-app"), "--user", "uncli", "--exec", "cat", "/home/uncli/.claude.json").Output()
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	_ = json.Unmarshal(got, &doc)
	for name := range want {
		if _, ok := doc.MCPServers[name]; !ok {
			t.Errorf("MCP server %q wasn't shared", name)
		}
	}
	t.Logf("shared MCP servers: %d", len(doc.MCPServers))

	// Their tools are there in a new session's first message: the CLI waits
	// for the servers to connect instead of starting without them.
	if len(want) > 0 {
		v2, _, err := svc.CreateSessionIn("cowork", folder, "haiku", "it-app")
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.Send(context.Background(), v2.ID, "List the names of the MCP tools you have, comma separated, or say NONE.", nil); err != nil {
			t.Fatal(err)
		}
		var first store.Page
		for time.Now().Before(deadline) {
			ps, _ := svc.Sessions.Pages(v2.ID)
			if len(ps) == 1 && ps[0].Status != "open" {
				first = ps[0]
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if first.Status != "done" || strings.Contains(strings.ToUpper(first.AnswerMD), "NONE") {
			t.Errorf("first message had no MCP tools: %s %q", first.Status, first.AnswerMD)
		}
	}
}
