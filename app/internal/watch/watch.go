// Package watch finds and reads the text files of a session's folder, for
// the side panel's File tab: a live view of one file (a TODO the agent
// keeps, a log, a config). Only files inside the folder are listed or
// read, and nothing goes to the model.
package watch

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is a file in the folder, by its path relative to it (slashes).
type Entry struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"` // Unix milliseconds
}

// File is a file as read: its text, or why there is none.
type File struct {
	Path     string `json:"path"`
	Text     string `json:"text"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Missing  bool   `json:"missing,omitempty"` // not there (deleted, or not yet written)
	Binary   bool   `json:"binary,omitempty"`  // not text
	TooBig   bool   `json:"tooBig,omitempty"`  // over the cap
}

// MaxSize is the most of a file the panel shows.
const MaxSize = 1 << 20

// maxWalk stops a listing of a huge folder.
const maxWalk = 50000

// skipDirs are folders whose files nobody watches: version control,
// dependencies, build output, caches.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, "bower_components": true, "vendor": true,
	"dist": true, "build": true, "out": true, "bin": true, "obj": true, "target": true, ".next": true, ".svelte-kit": true,
	".venv": true, "venv": true, "__pycache__": true, ".mypy_cache": true, ".pytest_cache": true, ".cache": true,
	"coverage": true, ".idea": true, ".vs": true, ".gradle": true,
}

// binaryExt are kinds of file that aren't text.
var binaryExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".bmp": true, ".tif": true, ".tiff": true,
	".pdf": true, ".zip": true, ".gz": true, ".tgz": true, ".tar": true, ".7z": true, ".rar": true, ".xz": true, ".bz2": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true, ".o": true, ".a": true, ".lib": true, ".obj": true,
	".class": true, ".jar": true, ".pyc": true, ".wasm": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".mp3": true, ".mp4": true, ".mov": true, ".avi": true, ".wav": true, ".flac": true, ".ogg": true, ".webm": true,
	".db": true, ".sqlite": true, ".db-wal": true, ".db-shm": true, ".psd": true, ".blend": true,
}

// List finds the folder's text files whose path contains query (any case;
// all of them when it's empty), most recently changed first, at most limit.
func List(root, query string, limit int) ([]Entry, error) {
	q := strings.ToLower(strings.TrimSpace(filepath.ToSlash(query)))
	out := []Entry{}
	seen := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil // an unreadable folder is skipped
		}
		if seen++; seen > maxWalk {
			return fs.SkipAll
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || binaryExt[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if q != "" && !strings.Contains(strings.ToLower(rel), q) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, Entry{Path: rel, Size: info.Size(), Modified: info.ModTime().UnixMilli()})
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Modified != out[j].Modified {
			return out[i].Modified > out[j].Modified
		}
		return out[i].Path < out[j].Path
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, err
}

// ErrOutside is a path that leads out of the folder.
var ErrOutside = errors.New("only files in the session's folder can be watched")

// resolve turns a path relative to root into a file inside it, refusing
// anything that leads out of it (.., an absolute path, or a link).
func resolve(root, rel string) (string, error) {
	rel = filepath.FromSlash(rel)
	if rel == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", ErrOutside
	}
	p := filepath.Join(root, rel)
	if r, err := filepath.Rel(root, p); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", ErrOutside
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realP, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p, nil // not there (yet): Read says so
	}
	if r, err := filepath.Rel(realRoot, realP); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", ErrOutside
	}
	return realP, nil
}

// Read reads a file in the folder, unless it's unchanged since modified
// (then ok is false and File is empty): the panel asks every second or so.
func Read(root, rel string, since int64) (f File, ok bool, err error) {
	p, err := resolve(root, rel)
	if err != nil {
		return File{}, false, err
	}
	f = File{Path: filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))}
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() {
		f.Missing = true
		return f, since != -1, nil
	}
	f.Size, f.Modified = st.Size(), st.ModTime().UnixMilli()
	if since == f.Modified {
		return File{}, false, nil
	}
	if f.Size > MaxSize {
		f.TooBig = true
		return f, true, nil
	}
	h, err := os.Open(p)
	if err != nil {
		return File{}, false, err
	}
	defer h.Close()
	b, err := io.ReadAll(io.LimitReader(h, MaxSize))
	if err != nil {
		return File{}, false, err
	}
	head := b
	if len(head) > 8000 {
		head = head[:8000]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		f.Binary = true
		return f, true, nil
	}
	f.Text = string(b)
	return f, true, nil
}
