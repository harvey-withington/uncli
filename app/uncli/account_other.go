//go:build !windows

package uncli

import "os"

// serviceAccount is "root" when the process runs as root, "" otherwise.
func serviceAccount() string {
	if os.Geteuid() == 0 {
		return "root"
	}
	return ""
}
