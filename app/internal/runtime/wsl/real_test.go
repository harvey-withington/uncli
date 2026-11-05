package wsl

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/secrets"
)

// TestRealDistro builds a distro with real WSL, mounts a folder and runs
// one turn in it. It needs WSL, a verified root filesystem and Linux CLI,
// and a container token in the credential store, so it runs only when
// asked: UNCLI_WSL_ROOTFS and UNCLI_WSL_CLI name the files. It uses a
// little of the account's usage.
func TestRealDistro(t *testing.T) {
	rootfs, cli := os.Getenv("UNCLI_WSL_ROOTFS"), os.Getenv("UNCLI_WSL_CLI")
	if rootfs == "" || cli == "" {
		t.Skip("set UNCLI_WSL_ROOTFS and UNCLI_WSL_CLI to build a real distro")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	name := "uncli-it"
	_ = Remove(ctx, name)
	// The disk is WSL's until the distro is removed, so it goes after.
	disk, err := os.MkdirTemp("", "uncli-it-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = Remove(context.Background(), name)
		_ = os.RemoveAll(disk)
	})
	before := defaultDistro()
	// A package, and setup steps that use it and run as root.
	spec := Spec{Name: name, Dir: disk, RootFS: rootfs, CLI: cli, Packages: []string{"jq"},
		Setup: []string{"jq -n '{step: 1}' > /etc/uncli-step", "id -un >> /etc/uncli-step"}}
	if err := Build(ctx, spec, func(s string) { t.Log("step", s) }); err != nil {
		t.Fatal(err)
	}
	// A step that lets the CLI's user become root is refused, the
	// half-built distro removed.
	loose := Spec{Name: name + "-loose", Dir: filepath.Join(disk, "loose"), RootFS: rootfs, CLI: cli, Packages: []string{"sudo"},
		Setup: []string{"echo 'uncli ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/uncli"}}
	if err := os.MkdirAll(loose.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Build(ctx, loose, nil); err == nil || !strings.Contains(err.Error(), "become root") {
		t.Errorf("sudo step: err = %v", err)
	}
	if st, _ := CheckStatus(ctx); slices.Contains(st.Distros, loose.Name) {
		t.Errorf("%s was left behind", loose.Name)
		_ = Remove(ctx, loose.Name)
	}
	if got := defaultDistro(); before != "" && got != before {
		t.Errorf("the default distro is now %q, was %q", got, before)
	}
	st, err := CheckStatus(ctx)
	if err != nil || !st.Installed {
		t.Fatalf("status = %+v, %v", st, err)
	}

	folder := filepath.Join(t.TempDir(), "My Folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "note.txt"), []byte("The password is TANGERINE.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New("it", func() (string, error) { return secrets.Get("claude-oauth-token") })
	defer r.Close()
	if b, err := r.ReadFile(ctx, "/etc/uncli-step"); err != nil || strings.Join(strings.Fields(string(b)), " ") != `{ "step": 1 } root` {
		t.Errorf("setup steps left %q, %v", b, err)
	}
	p, err := r.Start(ctx, core.Command{Args: []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
		"--model", "haiku", "--tools", "Read", "--allowedTools", "Read", "--strict-mcp-config"}}, folder)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, p.Stderr()) }()
	turn := `{"type":"user","message":{"role":"user","content":"Read note.txt in the current folder and reply with only the password."}}` + "\n"
	if _, err := io.WriteString(p.Stdin(), turn); err != nil {
		t.Fatal(err)
	}
	var cwd, result string
	sc := bufio.NewScanner(p.Stdout())
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		var l struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
			Cwd     string `json:"cwd"`
			Result  string `json:"result"`
		}
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			t.Fatalf("not JSON: %q", sc.Text())
		}
		if l.Type == "system" && l.Subtype == "init" {
			cwd = l.Cwd
		}
		if l.Type == "result" {
			result = l.Result
			_ = p.Kill()
			break
		}
	}
	if cwd != MountPoint(folder) {
		t.Errorf("cwd = %q, want %q", cwd, MountPoint(folder))
	}
	if !strings.Contains(result, "TANGERINE") {
		t.Errorf("result = %q", result)
	}
}

// TestLockedFileInRealDistro puts a locked copy in an existing distro
// (UNCLI_WSL_DISTRO) and checks the CLI's user can't change it however
// they try, that a new copy replaces it, and that nil removes it.
func TestLockedFileInRealDistro(t *testing.T) {
	name := os.Getenv("UNCLI_WSL_DISTRO")
	if name == "" {
		t.Skip("set UNCLI_WSL_DISTRO to an existing distro to try it there")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	r := &Runtime{Distro: name, run: execRunner}
	at := "/tmp/uncli-locked-test/CLAUDE.md"
	asUser := func(script string) error {
		_, err := r.run(ctx, nil, "--distribution", name, "--user", User, "--exec", "sh", "-c", script, "sh", at)
		return err
	}
	t.Cleanup(func() { _ = r.WriteLockedFile(context.Background(), at, nil) })
	if err := r.WriteLockedFile(ctx, at, []byte("Be brief.\n")); err != nil {
		t.Fatal(err)
	}
	for what, script := range map[string]string{
		"writing in place":   `echo changed > "$1"`,
		"saving by renaming": `echo changed > "$1.new" && mv -f "$1.new" "$1"`,
		"deleting":           `rm -f "$1" && [ ! -e "$1" ]`,
		"making it writable": `chmod u+w "$1"`,
	} {
		if err := asUser(script); err == nil {
			t.Errorf("%s worked", what)
		}
	}
	if b, _ := r.ReadFile(ctx, at); string(b) != "Be brief.\n" {
		t.Errorf("after the attempts: %q", b)
	}
	// The next start replaces it with what's on Windows now.
	if err := r.WriteLockedFile(ctx, at, []byte("Be thorough.\n")); err != nil {
		t.Fatal(err)
	}
	if b, _ := r.ReadFile(ctx, at); string(b) != "Be thorough.\n" {
		t.Errorf("replaced: %q", b)
	}
	if err := r.WriteLockedFile(ctx, at, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := r.ReadFile(ctx, at); len(b) != 0 {
		t.Errorf("removed, yet: %q", b)
	}
}
