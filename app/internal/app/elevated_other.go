//go:build !windows

package app

// elevated: only Windows warns about administrator rights so far.
func elevated() bool { return false }
