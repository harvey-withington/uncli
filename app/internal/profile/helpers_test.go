package profile

import (
	"io/fs"

	"uncli/config"
)

func fsSub() (fs.FS, error) { return fs.Sub(config.Defaults, "defaults") }
