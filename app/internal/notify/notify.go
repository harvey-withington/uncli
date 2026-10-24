// Package notify shows desktop notifications and, on Windows, UNCLI's tray
// icon. Wails v2 has neither, so each platform uses its own: a tray icon's
// balloons on Windows (shown as toasts under the app's name), osascript on
// macOS and notify-send on Linux. It knows nothing about Wails or sessions:
// the app decides when to notify and what clicking one does.
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
	// Keep puts the app's icon in the tray now and keeps it there, with
	// its menu, until Close (the app runs on with its window closed).
	// Where there's no tray it does nothing.
	Keep() error
	Close() // removes anything left behind (the tray icon)
}

// Handlers are what the tray icon and notes do. Each runs on its own
// goroutine.
type Handlers struct {
	// Click runs when the user clicks a note, with its Key, or with ""
	// when they click the tray icon or its Open.
	Click func(key string)
	Quit  func() // the tray menu's Quit
}

// Menu is the tray menu's wording, in the user's language.
type Menu struct{ Open, Quit string }

// New returns the platform's notifier. app is the name shown with the icon.
func New(app string, menu Menu, h Handlers) Notifier { return newPlatform(app, menu, h) }

// None shows nothing (tests, and platforms without a way to notify).
type None struct{}

func (None) Show(Note) error { return nil }
func (None) Keep() error     { return nil }
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
