//go:build !windows

package update

import "context"

// Blocked: updating from UNCLI is only on Windows so far.
func Blocked(context.Context) string { return "updating from UNCLI is only on Windows so far" }
