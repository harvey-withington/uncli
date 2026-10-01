//go:build !windows

package local

import (
	"os/exec"
	"syscall"
)

// configure starts the CLI in its own process group so the whole tree can
// be stopped together.
func configure(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attach(c *exec.Cmd) func() {
	pid := c.Process.Pid
	return func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }
}
