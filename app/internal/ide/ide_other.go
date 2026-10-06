//go:build !windows

package ide

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func hide(*exec.Cmd) {}

func openDefault(path string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return start(exec.Command(name, path))
}

func reveal(path string, isDir bool) error {
	if runtime.GOOS == "darwin" {
		if isDir {
			return start(exec.Command("open", path))
		}
		return start(exec.Command("open", "-R", path))
	}
	if !isDir {
		path = filepath.Dir(path)
	}
	return start(exec.Command("xdg-open", path))
}

func start(c *exec.Cmd) error {
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }()
	return nil
}
