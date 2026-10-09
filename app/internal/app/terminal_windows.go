package app

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// newConsole runs the command in a console window of its own, for the user
// to interact with (a CLI's own sign-in).
func newConsole(c *exec.Cmd) error {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	return c.Start()
}
