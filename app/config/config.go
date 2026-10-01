// Package config embeds UNCLI's built-in profiles, modifiers and toolbar.
package config

import "embed"

//go:embed defaults/*.yaml
var Defaults embed.FS
