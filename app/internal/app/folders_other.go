//go:build !windows

package app

import (
	"os"
	"strings"
)

func hiddenFolder(_ string, e os.DirEntry) bool { return strings.HasPrefix(e.Name(), ".") }
