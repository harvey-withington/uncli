package update

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func runScript(script, installer string) error {
	c := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, "-Installer", installer, "-Delay", "3")
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("the install script didn't start: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
