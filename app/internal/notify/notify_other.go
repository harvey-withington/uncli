//go:build !windows

package notify

import (
	"os/exec"
	"runtime"
	"strings"
)

// macOS: osascript's "display notification" (attributed to Script Editor;
// a click does nothing yet). Linux: notify-send, when it's installed.
// macOS is deferred until there's a Mac to test on.

type execNotifier struct{ app string }

func newPlatform(app string, _ func(string)) Notifier {
	switch runtime.GOOS {
	case "darwin":
		return execNotifier{app: app}
	case "linux":
		if _, err := exec.LookPath("notify-send"); err == nil {
			return execNotifier{app: app}
		}
	}
	return None{}
}

func (e execNotifier) Show(n Note) error {
	title, body := clip(n.Title, 120), clip(n.Body, 400)
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("osascript", "-e",
			"display notification "+appleString(body)+" with title "+appleString(title)+" subtitle "+appleString(e.app))
	} else {
		cmd = exec.Command("notify-send", "--app-name", e.app, title, body)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (execNotifier) Close() {}

// appleString quotes s as an AppleScript string literal.
func appleString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", " ", "\n", " ")
	return `"` + r.Replace(s) + `"`
}
