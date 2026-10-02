//go:build !windows

package clipfiles

// Not yet on macOS and Linux: pasting copied files there inserts their
// names, as the web view does.
func paths() ([]string, error) { return nil, nil }
