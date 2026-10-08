// Package wsl runs CLI processes in WSL distros that UNCLI creates and owns
// (decision 0011): one per container profile, named uncli-<profile>,
// locked down so it sees only the folders UNCLI mounts and can't start
// Windows programs. The CLI talks over stdin and stdout exactly as it does
// locally, through wsl.exe.
package wsl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"uncli/internal/core"
	"uncli/internal/runtime/local"
)

// Exe is wsl.exe.
const Exe = "wsl.exe"

// TokenEnv carries the CLI's OAuth token into the distro.
const TokenEnv = "CLAUDE_CODE_OAUTH_TOKEN"

// ErrNoToken means no one has signed in for containers yet.
var ErrNoToken = errors.New("sign in for containers first")

// DistroName is the distro of a container profile.
func DistroName(profile string) string { return "uncli-" + profile }

var profileRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

// ValidProfile reports whether an id can name a distro.
func ValidProfile(id string) bool { return profileRE.MatchString(id) }

// Runtime starts the CLI in one distro. It implements core.Runtime.
type Runtime struct {
	Distro string
	// Token returns the CLI's OAuth token, kept in the OS credential store.
	Token func() (string, error)
	// Prepare runs before each CLI start, once the session folder (host)
	// is mounted at cli: what the container shares of the user's setup
	// (memories, MCP servers). Optional.
	Prepare func(ctx context.Context, r *Runtime, host, cli string) error
	// Env is added to every CLI the runtime starts (through WSLENV).
	Env map[string]string
	// Account reports whether the CLI signs in with an account of its own
	// in the container (mounted by Prepare) instead of the token, which
	// would take priority over it. Optional.
	Account func() bool

	run runner

	mu     sync.Mutex
	keeper core.Proc
	live   map[*gentle]bool // the CLIs it started that are still running
}

// New is the runtime for a container profile's distro.
func New(profile string, token func() (string, error)) *Runtime {
	return &Runtime{Distro: DistroName(profile), Token: token, run: execRunner}
}

func (r *Runtime) ID() string { return "wsl" }

// Start runs the CLI's Linux build in the distro, in the session folder
// mounted at its place under /workspace. cmd.Path (the Windows binary) is
// replaced by the distro's own CLI; the command's environment and the
// token cross into WSL through WSLENV, and nothing else from Windows does.
func (r *Runtime) Start(ctx context.Context, cmd core.Command, workdir string) (core.Proc, error) {
	token := ""
	if r.Account == nil || !r.Account() {
		t, err := r.Token()
		if err != nil {
			return nil, err
		}
		if t == "" {
			return nil, ErrNoToken
		}
		token = t
	}
	var err error
	dir := "/home/" + User
	if workdir != "" {
		if dir, err = r.Mount(ctx, workdir); err != nil {
			return nil, err
		}
		if r.Prepare != nil {
			if err := r.Prepare(ctx, r, workdir, dir); err != nil {
				return nil, err
			}
		}
	}
	p, err := local.New().Start(ctx, r.command(cmd, dir, token), "")
	if err != nil {
		return nil, err
	}
	g := gently(p, stopGrace)
	r.mu.Lock()
	if r.live == nil {
		r.live = map[*gentle]bool{}
	}
	r.live[g] = true
	r.mu.Unlock()
	go func() {
		<-g.done
		r.mu.Lock()
		delete(r.live, g)
		r.mu.Unlock()
	}()
	return g, nil
}

// command is the wsl.exe command that runs cmd in dir. --exec passes each
// argument as it is: after a plain --, wsl.exe hands the line to a Linux
// shell, which would expand $ and run anything after a ;.
func (r *Runtime) command(cmd core.Command, dir, token string) core.Command {
	args := append([]string{"--distribution", r.Distro, "--user", User, "--cd", dir, "--exec", CLIPath}, cmd.Args...)
	env := map[string]string{}
	names := []string{}
	for k, v := range r.Env {
		env[k] = v
		names = append(names, k)
	}
	for k, v := range cmd.Env {
		env[k] = v
		names = append(names, k)
	}
	if token != "" {
		env[TokenEnv] = token
		names = append(names, TokenEnv)
	}
	sort.Strings(names)
	env["WSLENV"] = strings.Join(dedupe(names), ":")
	env["WSL_UTF8"] = "1"
	return core.Command{Path: Exe, Args: args, Env: env, EnvDrop: cmd.EnvDrop}
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// MountPoint is where a Windows folder appears in the distro: under
// /workspace, by its name and a hash of its full path, so two folders with
// the same name don't meet.
func MountPoint(host string) string {
	clean := strings.ToLower(filepath.Clean(host))
	sum := sha256.Sum256([]byte(clean))
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, strings.ToLower(filepath.Base(filepath.Clean(host))))
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "folder"
	}
	if len(name) > 40 {
		name = name[:40]
	}
	return WorkspaceRoot + "/" + name + "-" + hex.EncodeToString(sum[:4])
}

// HostPath maps a path the CLI sees back to Windows, for a session whose
// folder is host; ok is false for paths outside it.
func HostPath(host, p string) (string, bool) {
	point := MountPoint(host)
	switch {
	case p == point:
		return filepath.Clean(host), true
	case strings.HasPrefix(p, point+"/"):
		return filepath.Join(filepath.Clean(host), filepath.FromSlash(strings.TrimPrefix(p, point+"/"))), true
	}
	return "", false
}

// HostPath maps a path the CLI reported for a session in host back to it.
func (r *Runtime) HostPath(host, p string) (string, bool) { return HostPath(host, p) }

// mountScript mounts $2 (a Windows folder) at $1 unless something already
// is; drvfs with metadata keeps Linux permissions, owned by the CLI's user.
// Folders it creates in the user's home are the user's.
const mountScript = `mountpoint -q "$1" && exit 0
mkdir -p "$1"
d="$1"
while d=$(dirname "$d"); [ "$d" != / ] && [ "$d" != /home ]; do
  case "$d" in /home/uncli|/home/uncli/*) chown 1000:1000 "$d" ;; esac
done
mount -t drvfs "$2" "$1" -o metadata,uid=1000,gid=1000`

// Mount makes a Windows folder visible in the distro and returns where.
// Mounts last while the distro runs, so the runtime keeps it running.
func (r *Runtime) Mount(ctx context.Context, host string) (string, error) {
	if !filepath.IsAbs(host) {
		return "", fmt.Errorf("%q is not a full path", host)
	}
	if st, err := os.Stat(host); err != nil || !st.IsDir() {
		return "", fmt.Errorf("the folder %s doesn't exist", host)
	}
	point := MountPoint(host)
	return point, r.MountAt(ctx, host, point)
}

// MountAt mounts a Windows folder at a path in the distro (kept running so
// it stays).
func (r *Runtime) MountAt(ctx context.Context, host, at string) error {
	if err := r.keepRunning(ctx); err != nil {
		return err
	}
	if _, err := r.run(ctx, nil, "--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c", mountScript, "sh", at, filepath.Clean(host)); err != nil {
		return fmt.Errorf("could not mount %s in %s: %w", host, r.Distro, err)
	}
	return nil
}

// Unmount unmounts a path in the distro if something is mounted there.
func (r *Runtime) Unmount(ctx context.Context, at string) error {
	_, err := r.run(ctx, nil, "--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c", `if mountpoint -q "$1"; then umount "$1" || umount -l "$1"; fi`, "sh", at)
	return err
}

// Mounted reports whether something is mounted at a path in the distro.
func (r *Runtime) Mounted(ctx context.Context, at string) bool {
	_, err := r.run(ctx, nil, "--distribution", r.Distro, "--user", "root", "--exec", "mountpoint", "-q", at)
	return err == nil
}

// Has reports whether a program is installed in the distro.
func (r *Runtime) Has(ctx context.Context, program string) bool {
	_, err := r.run(ctx, nil, "--distribution", r.Distro, "--user", User, "--exec", "sh", "-c", `command -v "$1" >/dev/null`, "sh", program)
	return err == nil
}

// WriteFile writes a file as the CLI's user, creating its folder.
func (r *Runtime) WriteFile(ctx context.Context, at string, data []byte) error {
	_, err := r.run(ctx, bytes.NewReader(data), "--distribution", r.Distro, "--user", User, "--exec", "sh", "-c", `mkdir -p "$(dirname "$1")" && cat > "$1"`, "sh", at)
	return err
}

// ReadFile reads a file as the CLI's user; a missing one is empty.
func (r *Runtime) ReadFile(ctx context.Context, at string) ([]byte, error) {
	return r.run(ctx, nil, "--distribution", r.Distro, "--user", User, "--exec", "sh", "-c", `[ -f "$1" ] && cat "$1" || true`, "sh", at)
}

// keepRunning starts a process that holds the distro up, so its mounts
// stay; WSL stops a distro soon after its last process exits.
func (r *Runtime) keepRunning(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.keeper != nil {
		return nil
	}
	// It waits on its input: closing that ends it, with nothing killed.
	lp, err := local.New().Start(context.Background(), core.Command{Path: Exe,
		Args: []string{"--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c", "read _ || true"},
		Env:  map[string]string{"WSL_UTF8": "1"}}, "")
	if err != nil {
		return fmt.Errorf("could not start %s: %w", r.Distro, err)
	}
	p := gently(lp, stopGrace)
	go func() { _, _ = io.Copy(io.Discard, p.Stdout()) }()
	go func() { _, _ = io.Copy(io.Discard, p.Stderr()) }()
	go func() {
		_ = p.Wait()
		r.mu.Lock()
		if r.keeper == p {
			r.keeper = nil
		}
		r.mu.Unlock()
	}()
	r.keeper = p
	return nil
}

// Close lets the distro stop.
func (r *Runtime) Close() {
	r.mu.Lock()
	p := r.keeper
	r.keeper = nil
	live := make([]*gentle, 0, len(r.live))
	for g := range r.live {
		live = append(live, g)
	}
	r.mu.Unlock()
	// The CLIs first, asked to end and given the grace period, so nothing
	// is still running in the distro when the process holding it goes.
	for _, g := range live {
		_ = g.Kill()
	}
	deadline := time.Now().Add(stopGrace + time.Second)
	for _, g := range live {
		g.wait(time.Until(deadline))
	}
	if p != nil {
		_ = p.Kill()
		if g, ok := p.(*gentle); ok {
			g.wait(stopGrace + time.Second)
		}
	}
}

// stopGrace is how long a process UNCLI started in a distro gets to end by
// itself once its input is closed, before it's killed. WSL is shared with
// other tools (Docker, Podman, dev containers), so UNCLI stops what it
// started gently and by PID, never by name or with wsl --shutdown.
var stopGrace = 5 * time.Second

// gentle is a wsl.exe process UNCLI started. Kill closes its input (the
// CLI and the keeper end on that) and kills it, by its own PID, only if
// it's still running after the grace period; it doesn't block.
type gentle struct {
	core.Proc
	grace time.Duration
	done  chan struct{}
	once  sync.Once
	stop  sync.Once
}

func gently(p core.Proc, grace time.Duration) *gentle {
	return &gentle{Proc: p, grace: grace, done: make(chan struct{})}
}

func (g *gentle) Wait() error {
	err := g.Proc.Wait()
	g.once.Do(func() { close(g.done) })
	return err
}

func (g *gentle) Kill() error {
	g.stop.Do(func() {
		if c, ok := g.Proc.Stdin().(io.Closer); ok {
			_ = c.Close()
		}
		go func() {
			select {
			case <-g.done:
			case <-time.After(g.grace):
				_ = g.Proc.Kill()
			}
		}()
	})
	return nil
}

// wait waits up to d for the process to end.
func (g *gentle) wait(d time.Duration) {
	select {
	case <-g.done:
	case <-time.After(d):
	}
}

// runner runs wsl.exe to completion with optional stdin.
type runner func(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error)

// execRunner runs the real wsl.exe. WSL_UTF8 makes its own messages UTF-8
// (they're UTF-16 otherwise).
func execRunner(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, Exe, args...)
	c.Env = append(os.Environ(), "WSL_UTF8=1")
	hide(c)
	c.Stdin = stdin
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(out))
		}
		if msg != "" {
			return out, fmt.Errorf("%w: %s", err, lastLine(msg))
		}
	}
	return out, err
}

func lastLine(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}
