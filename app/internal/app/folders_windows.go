//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// hiddenFolder: dot folders and those Windows marks hidden or system.
func hiddenFolder(dir string, e os.DirEntry) bool {
	if strings.HasPrefix(e.Name(), ".") {
		return true
	}
	p, err := windows.UTF16PtrFromString(filepath.Join(dir, e.Name()))
	if err != nil {
		return true
	}
	attrs, err := windows.GetFileAttributes(p)
	return err == nil && attrs&(windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM) != 0
}
