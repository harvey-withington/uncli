// Package notify shows desktop notifications. Wails v2 has no notification
// API, so each platform uses its own: a tray icon's balloons on Windows
// (shown as toasts under the app's name), osascript on macOS and
// notify-send on Linux. It knows nothing about Wails or sessions: the app
// decides when to notify and what clicking one does.
package notify

import "unicode/utf16"

// Note is one notification. Key says what it is about (a session id), so
// a click can go there; a newer note replaces an older one.
type Note struct {
	Key   string
	Title string
	Body  string
}

// Notifier shows notes. Show never blocks for long and is safe to call
// from any goroutine.
type Notifier interface {
	Show(n Note) error
	Close() // removes anything left behind (the tray icon)
}

// New returns the platform's notifier. app is the name shown with the
// icon; onClick runs (on its own goroutine) when the user clicks a note,
// with its Key, or with "" when they click the app's tray icon.
func New(app string, onClick func(key string)) Notifier { return newPlatform(app, onClick) }

// None shows nothing (tests, and platforms without a way to notify).
type None struct{}

func (None) Show(Note) error { return nil }
func (None) Close()          {}

// clip shortens s to at most n UTF-16 units (Windows' fixed buffers count
// in those), ending with an ellipsis when cut.
func clip(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	r := []rune(s)
	for len(utf16.Encode(r)) > n-1 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
