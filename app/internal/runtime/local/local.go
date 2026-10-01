// Package local runs CLI processes on this machine.
package local

import (
	"context"
	"io"
	"os"
	"os/exec"
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
	return c.Output()
}

// StartDetached starts a command that outlives the call (browser sign-in)
// and returns a wait function.
func StartDetached(cmd core.Command) (func() error, error) {
	c := exec.Command(cmd.Path, cmd.Args...)
	c.Env = Env(os.Environ(), cmd)
	configure(c)
	c.Stdin = strings.NewReader("")
	if err := c.Start(); err != nil {
		return nil, err
	}
	return c.Wait, nil
}
