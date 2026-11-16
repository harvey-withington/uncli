package update

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// runScript runs the install script and returns what it printed.
func runScript(script, installer string) (string, error) {
	c := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, "-Installer", installer, "-Delay", "3")
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := c.CombinedOutput()
	return string(out), err
}
