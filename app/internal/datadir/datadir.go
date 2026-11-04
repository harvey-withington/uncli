// Package datadir guards a UNCLI data folder (decision 0027): one UNCLI at
// a time uses it, and it stays with the UNCLI it was made for, the desktop
// app or one host. Two UNCLIs on one folder would corrupt its database
// and could resume one CLI conversation twice; a host given the desktop
// app's folder by mistake would drive the user's own sessions.
//
// These guard against mistakes, not malice: a program running as the user
// can read the folder, or the CLIs' sign-in, without UNCLI.
package datadir

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Owner is who a data folder belongs to.
type Owner struct {
	Kind string `json:"kind"`           // KindDesktop or KindHost
	Name string `json:"name,omitempty"` // a host's name
}

const (
	KindDesktop = "desktop"
	KindHost    = "host"
)

// The files in the folder: the lock, held while a UNCLI runs on it, and
// the owner, written the first time.
const (
	lockFile  = "uncli.lock"
	ownerFile = "uncli-owner.json"
)

// ErrInUse: another UNCLI is running on the folder.
var ErrInUse = errors.New("another UNCLI is using this folder")

// ErrNotYours: the folder belongs to another UNCLI.
var ErrNotYours = errors.New("this folder belongs to another UNCLI")

func (o Owner) String() string {
	if o.Kind == KindHost {
		return fmt.Sprintf("the app %q", o.Name)
	}
	return "UNCLI's desktop app"
}

// Claim takes the folder for o while it runs: it refuses one marked for
// another owner, or that another UNCLI is running on, and marks a new one
// as o's. A folder that already holds a UNCLI database but no mark is the
// desktop app's (made before marks); release frees it.
func Claim(dir string, o Owner) (release func(), err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	unlock, err := lock(filepath.Join(dir, lockFile))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, dir)
	}
	if err := own(dir, o); err != nil {
		unlock()
		return nil, err
	}
	return unlock, nil
}

func own(dir string, o Owner) error {
	path := filepath.Join(dir, ownerFile)
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		var was Owner
		if err := json.Unmarshal(b, &was); err != nil {
			return fmt.Errorf("%s isn't readable: %w", path, err)
		}
		if was.Kind != o.Kind || (o.Kind == KindHost && !strings.EqualFold(was.Name, o.Name)) {
			return fmt.Errorf("%w: %s is %s's, not %s's. Give this one a folder of its own (or, if it was renamed, delete %s)", ErrNotYours, dir, was, o, ownerFile)
		}
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	// No mark yet. A database without one is the desktop app's, from
	// before marks.
	if _, err := os.Stat(filepath.Join(dir, "uncli.db")); err == nil && o.Kind != KindDesktop {
		return fmt.Errorf("%w: %s holds a UNCLI that isn't marked as %s's. Give this one a folder of its own", ErrNotYours, dir, o)
	}
	b, _ = json.Marshal(o)
	return os.WriteFile(path, b, 0o644)
}

// Inside reports whether path is dir or within it.
func Inside(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
