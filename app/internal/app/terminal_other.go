//go:build !windows

package app

import (
	"errors"
	"os/exec"
	"strings"
)

// newConsole: UNCLI can't open a terminal window here yet; the error says
// what to run.
func newConsole(c *exec.Cmd) error {
	return errors.New("open a terminal and run: " + strings.Join(c.Args, " "))
}
