package main

import (
	"embed"
	"io/fs"
	"log"

	"uncli/internal/bridge"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	if err := bridge.Run(dist); err != nil {
		log.Fatal(err)
	}
}
