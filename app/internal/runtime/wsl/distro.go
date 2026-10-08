package wsl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Status is WSL on this machine.
type Status struct {
	Installed bool     `json:"installed"`
	Version   string   `json:"version,omitempty"` // WSL's own version, e.g. 3.0.1.0
	Kernel    string   `json:"kernel,omitempty"`
	Distros   []string `json:"distros"` // UNCLI's distros (uncli-*)
	// Unresponsive: wsl.exe didn't answer in time. WSL's service can wedge
	// (an interrupted unregister, say); restarting
	// Windows brings it back. UNCLI never runs wsl --shutdown: it would
	// stop every other tool's distros and containers too.
	Unresponsive bool `json:"unresponsive,omitempty"`
}

// How long wsl.exe gets before UNCLI says WSL isn't responding (tests
// shorten them).
var (
	askTimeout    = 15 * time.Second
	removeTimeout = 2 * time.Minute
)

// ErrUnresponsive means wsl.exe didn't answer in time.
var ErrUnresponsive = errors.New("WSL isn't responding: restart Windows to bring it back")

var (
	wslVersionRE = regexp.MustCompile(`(?m)^WSL version:\s*(\S+)`)
	kernelRE     = regexp.MustCompile(`(?m)^Kernel version:\s*(\S+)`)
)

// CheckStatus asks wsl.exe whether WSL is installed and lists UNCLI's
// distros. Before the restart that finishes an install, wsl.exe still says
// it isn't installed.
func CheckStatus(ctx context.Context) (Status, error) { return checkStatus(ctx, execRunner) }

func checkStatus(ctx context.Context, run runner) (Status, error) {
	s := Status{Distros: []string{}}
	ask, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	out, err := run(ask, nil, "--version")
	if ask.Err() == context.DeadlineExceeded {
		s.Unresponsive = true
		return s, nil
	}
	if err != nil {
		return s, nil // not installed (or not working): say so, not an error
	}
	text := strings.ReplaceAll(string(out), "\x00", "")
	if m := wslVersionRE.FindStringSubmatch(text); m != nil {
		s.Version = m[1]
	}
	if m := kernelRE.FindStringSubmatch(text); m != nil {
		s.Kernel = m[1]
	}
	s.Installed = s.Version != ""
	if !s.Installed {
		return s, nil
	}
	names, err := listDistros(ask, run)
	if ask.Err() == context.DeadlineExceeded {
		s.Unresponsive = true
		return s, nil
	}
	if err != nil {
		return s, err
	}
	for _, n := range names {
		if strings.HasPrefix(n, "uncli-") {
			s.Distros = append(s.Distros, n)
		}
	}
	return s, nil
}

// listDistros lists every distro's name. With none, wsl.exe exits non-zero
// and explains how to install one, which is an empty list here.
func listDistros(ctx context.Context, run runner) ([]string, error) {
	out, err := run(ctx, nil, "--list", "--quiet")
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, l := range strings.Split(strings.ReplaceAll(string(out), "\x00", ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	return names, nil
}

// InstallWSL turns WSL on: wsl.exe --install --no-distribution, run as
// administrator, which asks the user once. Windows has to restart before
// WSL works.
func InstallWSL(ctx context.Context) error {
	ps := `$p = Start-Process -FilePath wsl.exe -ArgumentList '--install','--no-distribution' -Verb RunAs -WindowStyle Hidden -Wait -PassThru; exit $p.ExitCode`
	c := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	hide(c)
	if out, err := c.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "canceled by the user") {
			return errors.New("WSL wasn't turned on: the administrator prompt was declined")
		}
		return fmt.Errorf("turning WSL on failed: %w", err)
	}
	return nil
}

// Spec is a distro to build.
type Spec struct {
	Name     string   // uncli-<profile>
	Dir      string   // where WSL keeps its disk
	RootFS   string   // a verified root filesystem tarball
	CLI      string   // the verified Linux build of the CLI
	Packages []string // apk packages beyond the base ones
}

// Build creates a locked-down distro from spec. On failure the half-built
// distro is removed. The user's default distro is put back: WSL makes the
// first distro imported the default.
func Build(ctx context.Context, spec Spec, step func(string)) error {
	return build(ctx, spec, step, execRunner, defaultDistro)
}

func build(ctx context.Context, spec Spec, step func(string), run runner, def func() string) (err error) {
	if !strings.HasPrefix(spec.Name, "uncli-") || !ValidProfile(strings.TrimPrefix(spec.Name, "uncli-")) {
		return fmt.Errorf("%q isn't a name UNCLI gives its distros", spec.Name)
	}
	script, err := setupScript(spec.Packages)
	if err != nil {
		return err
	}
	if step == nil {
		step = func(string) {}
	}
	before := def()
	if err := os.MkdirAll(spec.Dir, 0o755); err != nil {
		return err
	}
	step("import")
	// The lockdown goes in before the first start, so the distro is never
	// started unlocked and needn't be stopped and started again to apply it.
	locked := filepath.Join(spec.Dir, "rootfs-locked.tar.gz")
	if err := withWSLConf(spec.RootFS, locked); err != nil {
		return fmt.Errorf("preparing %s: %w", spec.Name, err)
	}
	_, err = run(ctx, nil, "--import", spec.Name, spec.Dir, locked, "--version", "2")
	_ = os.Remove(locked)
	if err != nil {
		return fmt.Errorf("importing %s: %w", spec.Name, err)
	}
	defer func() {
		if before != "" && before != spec.Name {
			_, _ = run(context.Background(), nil, "--set-default", before)
		}
		if err != nil {
			_, _ = run(context.Background(), nil, "--terminate", spec.Name)
			_, _ = run(context.Background(), nil, "--unregister", spec.Name)
		}
	}()
	root := []string{"--distribution", spec.Name, "--user", "root", "--exec", "sh", "-s"}
	step("packages")
	if _, err := run(ctx, strings.NewReader(script), root...); err != nil {
		return fmt.Errorf("setting up %s: %w", spec.Name, err)
	}
	step("cli")
	f, err := os.Open(spec.CLI)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := run(ctx, f, "--distribution", spec.Name, "--user", "root", "--exec", "sh", "-c", installCLI); err != nil {
		return fmt.Errorf("installing the CLI in %s: %w", spec.Name, err)
	}
	step("lockdown")
	out, err := run(ctx, strings.NewReader(checkLockdown), "--distribution", spec.Name, "--exec", "sh", "-s")
	if err != nil {
		return fmt.Errorf("checking %s: %w", spec.Name, err)
	}
	if err := lockedDown(string(out), 1); err != nil {
		return fmt.Errorf("%s isn't locked down: %w", spec.Name, err)
	}
	step("check")
	v, err := run(ctx, nil, "--distribution", spec.Name, "--user", User, "--exec", CLIPath, "--version")
	if err != nil {
		return fmt.Errorf("the CLI doesn't run in %s: %w", spec.Name, err)
	}
	if !bytes.Contains(v, []byte("Claude Code")) {
		return fmt.Errorf("the CLI in %s says %q", spec.Name, strings.TrimSpace(string(v)))
	}
	return nil
}

// Terminate stops one of UNCLI's distros through WSL's own shutdown of
// it; other distros, and WSL itself, are left alone.
func Terminate(ctx context.Context, name string) error {
	if !strings.HasPrefix(name, "uncli-") {
		return fmt.Errorf("%q isn't one of UNCLI's distros", name)
	}
	ctx, cancel := context.WithTimeout(ctx, removeTimeout)
	defer cancel()
	_, err := execRunner(ctx, nil, "--terminate", name)
	if ctx.Err() == context.DeadlineExceeded {
		return ErrUnresponsive
	}
	return err
}

// Remove deletes one of UNCLI's distros and everything in it: stopped
// first (Terminate), then unregistered. It never touches another distro.
func Remove(ctx context.Context, name string) error {
	if !strings.HasPrefix(name, "uncli-") {
		return fmt.Errorf("%q isn't one of UNCLI's distros", name)
	}
	if err := Terminate(ctx, name); errors.Is(err, ErrUnresponsive) {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, removeTimeout)
	defer cancel()
	_, err := execRunner(ctx, nil, "--unregister", name)
	if ctx.Err() == context.DeadlineExceeded {
		return ErrUnresponsive
	}
	return err
}
