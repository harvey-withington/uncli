//go:build windows

package ide

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hide keeps a console window from flashing up (code.cmd runs in cmd.exe).
func hide(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// openDefault asks the shell to open a document with its default app.
func openDefault(path string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// reveal opens Explorer on a folder, or on a file's folder with the file
// selected. Explorer reads its own command line, so it is written by hand.
func reveal(path string, isDir bool) error {
	cmd := exec.Command("explorer.exe")
	if isDir {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe "` + path + `"`}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // Explorer exits 1 even when it worked
	return nil
}
