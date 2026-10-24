package app

import (
	"os"
	"testing"
)

// Start with Windows: UNCLI can start when the user signs in, minimised to
// the tray, so a phone can reach its sessions after a reboot (decision
// 0021). It's an entry in the user's own Run key, never a service or a
// machine-wide setting. The preference is the truth; the entry follows it
// whenever it's saved.

// MinimisedArg is the command-line flag the Run entry starts UNCLI with:
// straight to the tray, no window.
const MinimisedArg = "--minimised"

// startupEntry writes or removes the Run entry. Test binaries get one that
// touches nothing, and tests that check it replace it.
var startupEntry = func() func(on bool, command string) error {
	if testing.Testing() {
		return func(bool, string) error { return nil }
	}
	return setStartupEntry
}()

// applyStartup makes the Run entry match the preference.
func applyStartup(on bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startupEntry(on, `"`+exe+`" `+MinimisedArg)
}
