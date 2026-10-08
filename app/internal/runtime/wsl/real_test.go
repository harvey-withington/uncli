package wsl

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"uncli/internal/core"
	"uncli/internal/secrets"
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
	if err := Build(ctx, Spec{Name: name, Dir: disk, RootFS: rootfs, CLI: cli}, func(s string) { t.Log("step", s) }); err != nil {
		t.Fatal(err)
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
