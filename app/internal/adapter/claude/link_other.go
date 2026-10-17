//go:build !windows

package claude

import "os"

// link makes a symbolic link at path pointing to target.
func link(target, path string) error { return os.Symlink(target, path) }
