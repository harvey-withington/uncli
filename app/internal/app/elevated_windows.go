package app

import "golang.org/x/sys/windows"

// elevated reports whether UNCLI runs with administrator rights ("Run as
// administrator", or started by an installer that was), which it never
// needs: everything it starts inherits them.
func elevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
