//go:build !windows

package app

import "errors"

// StartupSupported says whether this platform can start UNCLI at sign-in.
const StartupSupported = false

func setStartupEntry(on bool, _ string) error {
	if on {
		return errors.New("starting with the system is only on Windows for now")
	}
	return nil
}
