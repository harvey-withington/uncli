// Package artifacts keeps the versions of what sessions put in their
// artifacts folder. Each version's content is stored once, under its
// SHA-256, in a folder of UNCLI's own; a page records which versions its
// turn produced, so paging back shows each artifact as it was then. No git
// is needed and nothing is added to the user's folder (decision record 0008).
package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Folder is the artifacts folder's name inside a session folder.
const Folder = "artifacts"

// MaxVersioned is the biggest file kept as a version; a bigger one is
// recorded without content and shown only from the folder.
const MaxVersioned = 25 << 20

// MaxListed is the most files List reports.
const MaxListed = 500

// Store holds artifact contents by hash.
type Store struct{ dir string }

func NewStore(dir string) *Store { return &Store{dir: dir} }

var hashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (s *Store) path(hash string) string { return filepath.Join(s.dir, hash[:2], hash) }

// Put stores a file's content (once per content) and returns its hash. A
// file over MaxVersioned is hashed but not stored: hash is empty.
func (s *Store) Put(file string) (hash string, size int64, err error) {
	f, err := os.Open(file)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	if st.Size() > MaxVersioned {
		return "", st.Size(), nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", 0, err
	}
	tmp, err := os.CreateTemp(s.dir, "put-*")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(tmp.Name()) // gone already once renamed
	h := sha256.New()
	size, err = io.Copy(io.MultiWriter(tmp, h), f)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", 0, err
	}
	hash = hex.EncodeToString(h.Sum(nil))
	dst := s.path(hash)
	if _, err := os.Stat(dst); err == nil {
		return hash, size, nil // the same content is already kept
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return "", 0, err
	}
	return hash, size, nil
}

// ErrTooBig means a file is bigger than the caller will show.
var ErrTooBig = errors.New("too big to show here")

// Read returns a stored version's content.
func (s *Store) Read(hash string, max int64) ([]byte, error) {
	if !hashRE.MatchString(hash) {
		return nil, errors.New("not an artifact version")
	}
	return readCapped(s.path(hash), max)
}

// File is an artifact in the folder now.
type File struct {
	Path string `json:"path"` // relative, with slashes
	Size int64  `json:"size"`
}

// List reports the files under root, sorted, at most MaxListed. A missing
// folder has none.
func List(root string) ([]File, error) {
	out := []File{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		out = append(out, File{Path: filepath.ToSlash(rel), Size: info.Size()})
		if len(out) >= MaxListed {
			return fs.SkipAll
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// Resolve turns a path relative to root into a file inside it, refusing
// anything that leads out of it (.., an absolute path, or a link).
func Resolve(root, rel string) (string, error) {
	rel = filepath.FromSlash(rel)
	if rel == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", errors.New("not an artifact")
	}
	p := filepath.Join(root, rel)
	if r, err := filepath.Rel(root, p); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", errors.New("not an artifact")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realP, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", errors.New("that artifact no longer exists")
	}
	if r, err := filepath.Rel(realRoot, realP); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", errors.New("not an artifact")
	}
	return realP, nil
}

// ReadLive reads an artifact as it is in the folder now.
func ReadLive(root, rel string, max int64) ([]byte, error) {
	p, err := Resolve(root, rel)
	if err != nil {
		return nil, err
	}
	return readCapped(p, max)
}

func readCapped(p string, max int64) ([]byte, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, errors.New("that artifact no longer exists")
	}
	if st.IsDir() {
		return nil, errors.New("that's a folder")
	}
	if st.Size() > max {
		return nil, fmt.Errorf("%w (%d MB)", ErrTooBig, st.Size()>>20)
	}
	return os.ReadFile(p)
}
