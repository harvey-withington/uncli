package wsl

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"uncli/internal/core"
)

// fake is wsl.exe: it records each call and answers from a table keyed by
// the call's first words.
type fake struct {
	calls  [][]string
	stdins []string
	answer func(args []string) (string, error)
}

func (f *fake) run(_ context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	in := ""
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		in = string(b)
	}
	f.stdins = append(f.stdins, in)
	if f.answer == nil {
		return nil, nil
	}
	out, err := f.answer(args)
	return []byte(out), err
}

func (f *fake) has(words ...string) bool {
	for _, c := range f.calls {
		if len(c) >= len(words) && slices.Equal(c[:len(words)], words) {
			return true
		}
	}
	return false
}

func TestCommandUsesExecAndOnlyListedEnv(t *testing.T) {
	r := &Runtime{Distro: "uncli-dev"}
	cmd := r.command(core.Command{Path: `C:\cache\claude.exe`, Args: []string{"-p", "--append-system-prompt", "say $HOME; id"},
		Env: map[string]string{"FOO": "1"}, EnvDrop: []string{"CLAUDE"}}, "/workspace/x-1234", "tok")
	if cmd.Path != Exe {
		t.Fatalf("path = %s", cmd.Path)
	}
	want := []string{"--distribution", "uncli-dev", "--user", "uncli", "--cd", "/workspace/x-1234", "--exec", CLIPath, "-p", "--append-system-prompt", "say $HOME; id"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q", cmd.Args)
	}
	if slices.Contains(cmd.Args, "--") {
		t.Error("a plain -- hands the line to a shell")
	}
	if cmd.Env["WSLENV"] != "CLAUDE_CODE_OAUTH_TOKEN:FOO" || cmd.Env[TokenEnv] != "tok" || cmd.Env["WSL_UTF8"] != "1" {
		t.Errorf("env = %v", cmd.Env)
	}
}

func TestStartNeedsAToken(t *testing.T) {
	r := &Runtime{Distro: "uncli-dev", Token: func() (string, error) { return "", nil }}
	if _, err := r.Start(context.Background(), core.Command{}, ""); !errors.Is(err, ErrNoToken) {
		t.Errorf("err = %v", err)
	}
}

func TestMountPointsAndPathsBack(t *testing.T) {
	a := MountPoint(`S:\Local\Code\My Project`)
	b := MountPoint(`D:\Other\My Project`)
	if !strings.HasPrefix(a, "/workspace/my-project-") || a == b {
		t.Errorf("mount points %s, %s", a, b)
	}
	if MountPoint(`s:\local\code\my project\`) != a {
		t.Error("the same folder gets one place whatever its case or trailing slash")
	}
	if p, ok := HostPath(`S:\Local\Code\My Project`, a+"/src/main.go"); !ok || p != filepath.Join(`S:\Local\Code\My Project`, "src", "main.go") {
		t.Errorf("host path = %q %v", p, ok)
	}
	if _, ok := HostPath(`S:\Local\Code\My Project`, "/etc/passwd"); ok {
		t.Error("a path outside the folder has no Windows path")
	}
	if !strings.HasPrefix(MountPoint(`C:\`), "/workspace/folder-") {
		t.Errorf("drive root = %s", MountPoint(`C:\`))
	}
}

func TestSetupScript(t *testing.T) {
	s, err := setupScript([]string{"go", "nodejs"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"apk add --no-cache libgcc libstdc++ bash git util-linux-misc ca-certificates go nodejs", "adduser -D -h /home/uncli -s /bin/bash", "[automount]\nenabled = false", "[interop]\nenabled = false", `command = "echo 0 > /proc/sys/fs/binfmt_misc/WSLInterop"`} {
		if !strings.Contains(s, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	if _, err := setupScript([]string{"go; rm -rf /"}); err == nil {
		t.Error("a package name can't carry a command")
	}
}

func TestLockedDown(t *testing.T) {
	if err := lockedDown("uncli\ndisabled\n1\n", 1); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"root\ndisabled\n1", "uncli\nenabled\n1", "uncli\ndisabled\n3", "uncli"} {
		if lockedDown(bad, 1) == nil {
			t.Errorf("%q passed", bad)
		}
	}
}

func TestStatus(t *testing.T) {
	f := &fake{answer: func(a []string) (string, error) {
		switch a[0] {
		case "--version":
			return "WSL version: 3.0.1.0\r\nKernel version: 6.18.40.1-1\r\n", nil
		case "--list":
			return "Ubuntu\r\nuncli-dev\r\nuncli-go\r\n", nil
		}
		return "", nil
	}}
	s, err := checkStatus(context.Background(), f.run)
	if err != nil || !s.Installed || s.Version != "3.0.1.0" || s.Kernel != "6.18.40.1-1" || !slices.Equal(s.Distros, []string{"uncli-dev", "uncli-go"}) {
		t.Errorf("status = %+v, %v", s, err)
	}
	none := &fake{answer: func([]string) (string, error) { return "", errors.New("not installed") }}
	if s, _ := checkStatus(context.Background(), none.run); s.Installed {
		t.Error("a failing wsl.exe means not installed")
	}
}

func spec(t *testing.T) Spec {
	dir := t.TempDir()
	cli := filepath.Join(dir, "claude")
	if err := os.WriteFile(cli, []byte("ELF"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root.tar.gz")
	writeRootFS(t, root, map[string]string{"./etc/hostname": "localhost\n", "./etc/wsl.conf": "[automount]\nenabled = true\n"})
	return Spec{Name: "uncli-dev", Dir: filepath.Join(dir, "disk"), RootFS: root, CLI: cli, Packages: []string{"go"}}
}

func writeRootFS(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = io.WriteString(tw, body)
	}
	tw.Close()
	zw.Close()
	f.Close()
}

func readRootFS(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(tr)
		out[h.Name] = string(b)
	}
	return out
}

// The lockdown is in the image before the first start: UNCLI's wsl.conf
// replaces any the image had, and everything else is kept.
func TestWithWSLConf(t *testing.T) {
	sp := spec(t)
	dst := filepath.Join(t.TempDir(), "locked.tar.gz")
	if err := withWSLConf(sp.RootFS, dst); err != nil {
		t.Fatal(err)
	}
	got := readRootFS(t, dst)
	if got["./etc/hostname"] != "localhost\n" || got["./etc/wsl.conf"] != wslConf || len(got) != 2 {
		t.Errorf("image = %v", got)
	}
}

func TestBuild(t *testing.T) {
	f := &fake{answer: func(a []string) (string, error) {
		if slices.Contains(a, "-s") && a[3] != "root" {
			return "uncli\ndisabled\n1\n", nil // the lockdown check
		}
		if slices.Contains(a, "--version") {
			return "2.1.285 (Claude Code)\n", nil
		}
		return "", nil
	}}
	var steps []string
	if err := build(context.Background(), spec(t), func(s string) { steps = append(steps, s) }, f.run, func() string { return "Ubuntu" }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(steps, []string{"import", "packages", "cli", "lockdown", "check"}) {
		t.Errorf("steps = %v", steps)
	}
	// Locked down from the first start: no stop and start again, which
	// would disturb other tools sharing WSL for nothing.
	if f.has("--terminate") {
		t.Error("a good build terminated its distro")
	}
	if !f.has("--import", "uncli-dev") || !f.has("--set-default", "Ubuntu") {
		t.Errorf("calls = %q", f.calls)
	}
	if f.has("--unregister") {
		t.Error("a good build was removed")
	}
	if f.stdins[2] != "ELF" {
		t.Errorf("the CLI went in as %q", f.stdins[2])
	}
	for _, c := range f.calls {
		if slices.Contains(c, "--") {
			t.Errorf("%q uses a plain --", c)
		}
	}
}

func TestFailedBuildIsRemoved(t *testing.T) {
	f := &fake{answer: func(a []string) (string, error) {
		if slices.Contains(a, "-s") {
			return "", errors.New("apk: network down")
		}
		return "", nil
	}}
	err := build(context.Background(), spec(t), nil, f.run, func() string { return "" })
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("err = %v", err)
	}
	if !f.has("--terminate", "uncli-dev") || !f.has("--unregister", "uncli-dev") {
		t.Error("the half-built distro was left behind, or removed without stopping it first")
	}
	if f.has("--set-default") {
		t.Error("there was no default to put back")
	}
}

func TestBuildRefusesOtherNames(t *testing.T) {
	s := spec(t)
	for _, n := range []string{"Ubuntu", "uncli-", "uncli-A B"} {
		s.Name = n
		f := &fake{}
		if build(context.Background(), s, nil, f.run, func() string { return "" }) == nil || len(f.calls) != 0 {
			t.Errorf("%q was accepted", n)
		}
	}
}

// A wedged WSL doesn't hang UNCLI: wsl.exe that never answers is reported
// as unresponsive once the time is up.
func TestStatusWhenWSLHangs(t *testing.T) {
	askTimeout = 50 * time.Millisecond
	t.Cleanup(func() { askTimeout = 15 * time.Second })
	hang := func(ctx context.Context, _ io.Reader, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	start := time.Now()
	s, err := checkStatus(context.Background(), hang)
	if err != nil || !s.Unresponsive || s.Installed {
		t.Errorf("status = %+v, %v", s, err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("took %v", time.Since(start))
	}
}

// A runtime's own environment crosses into WSL with the command's.
func TestRuntimeEnv(t *testing.T) {
	r := &Runtime{Distro: "uncli-dev", Env: map[string]string{"MCP_CONNECTION_NONBLOCKING": "0"}}
	cmd := r.command(core.Command{Env: map[string]string{"FOO": "1"}}, "/w", "tok")
	if cmd.Env["MCP_CONNECTION_NONBLOCKING"] != "0" || cmd.Env["WSLENV"] != "CLAUDE_CODE_OAUTH_TOKEN:FOO:MCP_CONNECTION_NONBLOCKING" {
		t.Errorf("env = %v", cmd.Env)
	}
}

// Signed in to the account in the container, the CLI gets no token: the
// token would take priority over the account.
func TestAccountModeNeedsNoToken(t *testing.T) {
	r := &Runtime{Distro: "uncli-dev", Account: func() bool { return true }}
	cmd := r.command(core.Command{}, "/w", "")
	if _, ok := cmd.Env[TokenEnv]; ok || strings.Contains(cmd.Env["WSLENV"], TokenEnv) {
		t.Errorf("env = %v", cmd.Env)
	}
}

func TestHas(t *testing.T) {
	f := &fake{answer: func(a []string) (string, error) {
		if a[len(a)-1] == "bash" {
			return "", errors.New("exit status 1")
		}
		return "", nil
	}}
	r := &Runtime{Distro: "uncli-dev", run: f.run}
	if r.Has(context.Background(), "bash") || !r.Has(context.Background(), "git") {
		t.Error("wrong answer")
	}
	if !slices.Contains(f.calls[0], "--exec") || slices.Contains(f.calls[0], "--") {
		t.Errorf("call = %q", f.calls[0])
	}
}

// Setup steps run as root after the packages, in order; a failing one
// fails the build and says which.
func TestBuildSetupSteps(t *testing.T) {
	sp := spec(t)
	sp.Setup = []string{"npm i -g pnpm", "echo two"}
	// A distro whose relock prints sum and whose user is who.
	distro := func(sum, who string) *fake {
		f := &fake{}
		f.answer = func(a []string) (string, error) {
			switch f.stdins[len(f.stdins)-1] {
			case relock:
				return sum + "  /etc/wsl.conf\n", nil
			case becomesRoot:
				return who + "\n", nil
			case checkLockdown:
				return "uncli\ndisabled\n1\n", nil
			}
			if slices.Contains(a, "--version") {
				return "2.1.285 (Claude Code)\n", nil
			}
			return "", nil
		}
		return f
	}
	f := distro(wslConfSum(), "user")
	var steps []string
	if err := build(context.Background(), sp, func(s string) { steps = append(steps, s) }, f.run, func() string { return "" }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(steps, []string{"import", "packages", "setup", "cli", "lockdown", "check"}) {
		t.Errorf("steps = %v", steps)
	}
	if !strings.Contains(f.stdins[2], "npm i -g pnpm") || !strings.Contains(f.stdins[3], "echo two") || f.calls[2][3] != "root" {
		t.Errorf("setup calls = %q / %q", f.calls[2], f.stdins[2:4])
	}
	// wsl.conf is written again after the steps, and who's asked as the CLI's user.
	if f.stdins[4] != relock || !strings.Contains(relock, wslConf) || f.stdins[5] != becomesRoot || f.calls[5][3] != User {
		t.Errorf("after the steps: %q / %q", f.calls[4:6], f.stdins[4:6])
	}

	// A step that loosens the lockdown fails the build.
	if err := build(context.Background(), sp, nil, distro("0000", "user").run, func() string { return "" }); err == nil || !strings.Contains(err.Error(), "lockdown settings couldn't be restored") {
		t.Errorf("tampered wsl.conf: err = %v", err)
	}
	if err := build(context.Background(), sp, nil, distro(wslConfSum(), "root").run, func() string { return "" }); err == nil || !strings.Contains(err.Error(), "become root") {
		t.Errorf("sudo: err = %v", err)
	}

	calls := 0
	bad := &fake{answer: func(a []string) (string, error) {
		if slices.Contains(a, "-s") && a[3] == "root" {
			calls++
			if calls == 3 { // packages, step 1, then step 2 fails
				return "", errors.New("exit status 127")
			}
		}
		return "", nil
	}}
	err := build(context.Background(), sp, nil, bad.run, func() string { return "" })
	if err == nil || !strings.Contains(err.Error(), "setup step 2 (echo two) failed") {
		t.Errorf("err = %v", err)
	}
}
