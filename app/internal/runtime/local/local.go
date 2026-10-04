// Package local runs CLI processes on this machine.
package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"uncli/internal/core"
)

type Runtime struct{}

func New() *Runtime { return &Runtime{} }

func (r *Runtime) ID() string { return "local" }

// Env composes the child's environment: the inherited one minus dropped
// prefixes, plus the command's additions.
func Env(base []string, cmd core.Command) []string {
	out := make([]string, 0, len(base)+len(cmd.Env))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, override := cmd.Env[k]; override {
			continue
		}
		drop := false
		for _, p := range cmd.EnvDrop {
			if strings.HasPrefix(strings.ToUpper(k), strings.ToUpper(p)) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	for k, v := range cmd.Env {
		out = append(out, k+"="+v)
	}
	return out
}

func (r *Runtime) Start(ctx context.Context, cmd core.Command, workdir string) (core.Proc, error) {
	c := exec.Command(cmd.Path, cmd.Args...)
	c.Dir = workdir
	c.Env = Env(os.Environ(), cmd)
	configure(c)
	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	p := &proc{cmd: c, stdin: stdin, stdout: stdout, stderr: stderr}
	p.guard = attach(c)
	go func() {
		<-ctx.Done()
		_ = p.Kill()
	}()
	return p, nil
}

type proc struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
	guard  func() // kills the process tree; platform-specific

	once sync.Once
}

func (p *proc) Stdin() io.Writer  { return p.stdin }
func (p *proc) Stdout() io.Reader { return p.stdout }
func (p *proc) Stderr() io.Reader { return p.stderr }
func (p *proc) Wait() error       { return p.cmd.Wait() }

func (p *proc) Kill() error {
	var err error
	p.once.Do(func() {
		_ = p.stdin.Close()
		if p.guard != nil {
			p.guard()
		}
		if p.cmd.Process != nil {
			err = p.cmd.Process.Kill()
		}
	})
	return err
}

// Run runs a short command to completion (auth status and similar) and
// returns its stdout.
func Run(ctx context.Context, cmd core.Command) ([]byte, error) {
	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	c.Env = Env(os.Environ(), cmd)
	configure(c)
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timed out after waiting for %s", filepath.Base(cmd.Path))
		}
		return out, &RunError{Err: err, Stderr: stderr.String()}
	}
	return out, nil
}

// RunInput is Run with stdin.
func RunInput(ctx context.Context, cmd core.Command, stdin string) ([]byte, error) {
	return RunInputIn(ctx, "", cmd, stdin)
}

// RunInputIn is RunInput in a given working folder (empty: this process's).
func RunInputIn(ctx context.Context, dir string, cmd core.Command, stdin string) ([]byte, error) {
	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	c.Dir = dir
	c.Env = Env(os.Environ(), cmd)
	configure(c)
	c.Stdin = strings.NewReader(stdin)
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("timed out after waiting for %s", filepath.Base(cmd.Path))
		}
		return out, &RunError{Err: err, Stderr: stderr.String()}
	}
	return out, nil
}

// RunError is a failed short command, with the stderr that explains it.
type RunError struct {
	Err    error
	Stderr string
}

func (e *RunError) Error() string {
	if s := strings.TrimSpace(e.Stderr); s != "" {
		return e.Err.Error() + ": " + lastLine(s)
	}
	return e.Err.Error()
}

func (e *RunError) Unwrap() error { return e.Err }

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Interactive is a short-lived command UNCLI talks to over stdin (sign-in).
type Interactive struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	mu    sync.Mutex
	out   []byte
	done  chan struct{}
	err   error
}

// StartInteractive starts a command with a stdin pipe, collecting stdout
// and stderr together.
func StartInteractive(cmd core.Command) (*Interactive, error) {
	c := exec.Command(cmd.Path, cmd.Args...)
	c.Env = Env(os.Environ(), cmd)
	configure(c)
	in, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	p := &Interactive{cmd: c, stdin: in, done: make(chan struct{})}
	c.Stdout, c.Stderr = p, p
	if err := c.Start(); err != nil {
		return nil, err
	}
	go func() { p.err = c.Wait(); close(p.done) }()
	return p, nil
}

func (p *Interactive) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.out = append(p.out, b...)
	p.mu.Unlock()
	return len(b), nil
}

// Output is everything printed so far.
func (p *Interactive) Output() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.out...)
}

// Send writes a line to the command's stdin.
func (p *Interactive) Send(line string) error {
	_, err := io.WriteString(p.stdin, line+"\n")
	return err
}

// Done is closed when the command exits; Err is its result then.
func (p *Interactive) Done() <-chan struct{} { return p.done }
func (p *Interactive) Err() error            { return p.err }

func (p *Interactive) Kill() {
	_ = p.stdin.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}
