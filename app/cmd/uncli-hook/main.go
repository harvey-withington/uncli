// Command uncli-hook is UNCLI's hook mode on its own, for tests that run a
// real CLI without building the app (app.HookCommandEnv). The app itself
// runs as the hook ("uncli hook <event>", main.go).
package main

import (
	"os"

	"uncli/internal/hook"
)

func main() {
	if len(os.Args) != 2 {
		os.Stderr.WriteString("usage: uncli-hook <event>\n")
		os.Exit(2)
	}
	os.Exit(hook.Run(os.Args[1], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}
