package update

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	out, err := runScript(script, installer)
	if err == nil {
		return nil
	}
	if strings.TrimSpace(out) == "" {
		return err
	}
	// The whole output goes to the log; the user is told its message.
	logOutput(filepath.Join(dir, "update.log"), out)
	if msg := scriptMessage(out); msg != "" {
		return fmt.Errorf("the install script didn't start: %s (details in %s)", msg, filepath.Join(dir, "update.log"))
	}
	return fmt.Errorf("the install script didn't start: %v", err)
}

// scriptMessage is what a failed script says, without PowerShell's
// position and category lines ("At C:\…:167 char:1", "+ Register-…",
// "+ ~~~~", "+ CategoryInfo …"), joined onto one line.
func scriptMessage(out string) string {
	var parts []string
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "At ") || strings.HasPrefix(line, "+") {
			// A position block ends this error's message.
			if len(parts) > 0 {
				break
			}
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, " ")
}

func logOutput(path, out string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  The install script didn't start:\r\n%s\r\n", time.Now().Format("2006-01-02 15:04:05"), strings.TrimSpace(out))
}
