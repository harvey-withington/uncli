//go:build !windows

package wsl

import "os/exec"

func hide(*exec.Cmd) {}

func defaultDistro() string { return "" }
