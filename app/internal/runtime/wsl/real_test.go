package wsl

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
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

// TestLocalPathsInRealDistro mounts a project in an existing distro
// (UNCLI_WSL_DISTRO) with local paths, and checks that the container's
// files there and Windows' don't see each other, that taking the project
// out takes the local paths out first, and that they come back, with what
// was in them, when the project is mounted again. It uses no CLI.
func TestLocalPathsInRealDistro(t *testing.T) {
	name := os.Getenv("UNCLI_WSL_DISTRO")
	if name == "" {
		t.Skip("set UNCLI_WSL_DISTRO to an existing distro to try it there")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := &Runtime{Distro: name, run: execRunner, Local: func() []string { return []string{"node_modules", "apps/web/node_modules"} }}
	folder := filepath.Join(t.TempDir(), "release-manager")
	if err := os.MkdirAll(filepath.Join(folder, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(folder, "node_modules", "esbuild.exe"), []byte("windows\n"), 0o644)
	_ = os.WriteFile(filepath.Join(folder, "package.json"), []byte("{}\n"), 0o644)
	point, err := r.Mount(ctx, folder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Unmount(context.Background(), point)
		_, _ = r.run(context.Background(), nil, "--distribution", name, "--user", "root", "--exec", "rm", "-rf", LocalSource(point, ""))
		r.Close()
	})
	sh := func(user, script string) (string, error) {
		out, err := r.run(ctx, nil, "--distribution", name, "--user", user, "--exec", "sh", "-c", script, "sh", point)
		return strings.TrimSpace(string(out)), err
	}
	install := `echo linux > "$1/node_modules/esbuild" && echo linux > "$1/apps/web/node_modules/esbuild" && ls "$1/node_modules"`

	// The container's user writes into its own copies, and doesn't see
	// Windows' file; the rest of the project is Windows' as before.
	out, err := sh(User, install+` && cat "$1/package.json"`)
	if err != nil || strings.Join(strings.Fields(out), " ") != "esbuild {}" {
		t.Fatalf("in the container: %q, %v", out, err)
	}
	// Windows has its own file and not the container's; the missing path
	// was made, empty.
	if b, err := os.ReadFile(filepath.Join(folder, "node_modules", "esbuild.exe")); err != nil || string(b) != "windows\n" {
		t.Errorf("Windows' file: %q, %v", b, err)
	}
	for _, p := range []string{filepath.Join(folder, "node_modules", "esbuild"), filepath.Join(folder, "apps", "web", "node_modules", "esbuild")} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("the container's %s reached Windows", p)
		}
	}
	if st, err := os.Stat(filepath.Join(folder, "apps", "web", "node_modules")); err != nil || !st.IsDir() {
		t.Errorf("apps/web/node_modules wasn't made in the project: %v", err)
	}

	// The manifest lists them as local, from the distro's own disk.
	if err := r.WriteManifest(ctx); err != nil {
		t.Fatal(err)
	}
	if out, _ := sh(User, `cat `+ManifestPath); !strings.Contains(out, `"kind": "local"`) || !strings.Contains(out, LocalSource(point, "apps/web/node_modules")) {
		t.Errorf("manifest: %s", out)
	}

	// Taking the project out takes the local paths out first: each comes
	// off cleanly, none lazily left behind.
	trace, err := exec.Command(Exe, "--distribution", name, "--user", "root", "--exec", "sh", "-xc", unmountScript, "sh", point).CombinedOutput()
	if err != nil {
		t.Fatalf("unmount: %v %s", err, trace)
	}
	// The traced umount lines, in order, by their target.
	var order []string
	for _, l := range strings.Split(strings.ReplaceAll(string(trace), "\r", ""), "\n") {
		if f := strings.Fields(l); len(f) >= 3 && f[0] == "+" && f[1] == "umount" {
			order = append(order, strings.TrimPrefix(strings.Join(f[2:], " "), "-l "))
		}
	}
	if !slices.Equal(order, []string{point + "/node_modules", point + "/apps/web/node_modules", point}) && !slices.Equal(order, []string{point + "/apps/web/node_modules", point + "/node_modules", point}) ||
		strings.Contains(string(trace), "umount -l") {
		t.Errorf("unmount order %q:\n%s", order, trace)
	}
	if out, _ := sh("root", `grep -c " $1" /proc/self/mountinfo || true`); out != "0" {
		t.Errorf("still mounted: %s", out)
	}
	_ = r.Unmount(ctx, point) // forgets them

	// Mounted again, the container's copies are back as they were.
	if _, err := r.Mount(ctx, folder); err != nil {
		t.Fatal(err)
	}
	if out, err := sh(User, `cat "$1/node_modules/esbuild" "$1/apps/web/node_modules/esbuild"; ls "$1/node_modules"`); err != nil || strings.Join(strings.Fields(out), " ") != "linux linux esbuild" {
		t.Errorf("after remounting: %q, %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "node_modules", "esbuild")); err == nil {
		t.Error("the container's file reached Windows after remounting")
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
