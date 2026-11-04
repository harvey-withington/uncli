package profile

import (
	"io/fs"

	"github.com/harvey-withington/uncli/app/config"
)

func fsSub() (fs.FS, error) { return fs.Sub(config.Defaults, "defaults") }
