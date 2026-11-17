package update

import (
	"context"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// Blocked is why Update now can't work on this PC (see ready.go), or "".
func Blocked(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", powershellProbe)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := c.Output()
	if err != nil {
		return "Windows PowerShell, which installs updates, didn't run: " + err.Error()
	}
	return probeProblem(string(out))
}
