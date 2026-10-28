package update

import (
	_ "embed"
	"os"
	"path/filepath"
)

// The install script (decision 0024), the one npm run install-build runs:
// it hands the installer to a scheduled task that installs silently, then
// starts UNCLI again, from outside UNCLI.
//
//go:embed install-build.ps1
var installScript []byte

// Dir is where downloads and the install script go:
// %LocalAppData%\UNCLI\update.
func Dir() (string, error) {
	base, err := os.UserCacheDir() // %LocalAppData% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "UNCLI", "update"), nil
}

// Install hands installer to the install script, which quits UNCLI, installs
// it and starts UNCLI again. It returns once the script has started its
// task, a few seconds before UNCLI quits.
func Install(installer string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	script := filepath.Join(dir, "install-build.ps1")
	if err := os.WriteFile(script, installScript, 0o600); err != nil {
		return err
	}
	return runScript(script, installer)
}
