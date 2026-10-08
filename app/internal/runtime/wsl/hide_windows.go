//go:build windows

package wsl

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// hide keeps wsl.exe's console window from showing.
func hide(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

const lxss = `Software\Microsoft\Windows\CurrentVersion\Lxss`

// defaultDistro is the user's default WSL distro, from the registry (wsl.exe
// only says so in localised text).
func defaultDistro() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, lxss, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	id, _, err := k.GetStringValue("DefaultDistribution")
	k.Close()
	if err != nil || id == "" {
		return ""
	}
	d, err := registry.OpenKey(registry.CURRENT_USER, lxss+`\`+id, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer d.Close()
	name, _, _ := d.GetStringValue("DistributionName")
	return name
}
