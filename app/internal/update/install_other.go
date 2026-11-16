//go:build !windows

package update

import "errors"

func runScript(string, string) (string, error) {
	return "", errors.New("updating from UNCLI is only on Windows so far")
}
