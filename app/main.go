package main

import (
	"embed"
	"io/fs"
	"log"
	"os"

	"uncli/internal/bridge"
	"uncli/internal/hook"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Hook mode: a CLI asking UNCLI about a tool call (decision 0012).
	if len(os.Args) == 3 && os.Args[1] == "hook" {
		os.Exit(hook.Run(os.Args[2], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
	}
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	if err := bridge.Run(dist); err != nil {
		log.Fatal(err)
	}
}
