// Package gitinfo says which git branch a folder is on, by reading the
// repository's HEAD file: no git binary, no command run. The session
// header shows it, so which branch an agent is working on is plain without
// opening an editor (Harvey, 2026-11-09).
package gitinfo

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Head is where a repository's HEAD points: a branch, or a commit when
// it's detached. Its JSON is the window's GitHead, as store.GitHead's is.
type Head struct {
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit"` // a detached HEAD's commit, short; "" on a branch
}

// headFile finds the HEAD file of the repository dir is in: .git/HEAD up
// the tree, or, for a worktree or submodule, the one its .git file points
// to. "" when dir isn't in a repository.
func headFile(dir string) string {
	for d := filepath.Clean(dir); ; {
		dotgit := filepath.Join(d, ".git")
		if fi, err := os.Stat(dotgit); err == nil {
			if fi.IsDir() {
				return filepath.Join(dotgit, "HEAD")
			}
			if b, err := os.ReadFile(dotgit); err == nil {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:"); ok {
					gd := strings.TrimSpace(rest)
					if !filepath.IsAbs(gd) {
						gd = filepath.Join(d, gd)
					}
					return filepath.Join(gd, "HEAD")
				}
			}
			return ""
		}
		up := filepath.Dir(d)
		if up == d {
			return ""
		}
		d = up
	}
}

// parse reads a HEAD file's content.
func parse(content string) (Head, bool) {
	s := strings.TrimSpace(content)
	if ref, ok := strings.CutPrefix(s, "ref:"); ok {
		ref = strings.TrimSpace(ref)
		return Head{Branch: strings.TrimPrefix(ref, "refs/heads/")}, ref != ""
	}
	if len(s) >= 7 && strings.Trim(s, "0123456789abcdef") == "" {
		return Head{Commit: s[:7]}, true
	}
	return Head{}, false
}

// Reader answers for many folders, many times a second (each session's
// view asks): where each folder's HEAD file is is found once (a folder in
// no repository is looked at again after missAge, in case one was made),
// and the file is read again only when it changed.
type Reader struct {
	mu    sync.Mutex
	files map[string]found  // folder → its HEAD file
	seen  map[string]cached // HEAD file → what it said
	now   func() time.Time
}

const missAge = 30 * time.Second

type found struct {
	file string // "" for none
	at   time.Time
}

type cached struct {
	mod  time.Time
	head Head
	ok   bool
}

func NewReader() *Reader {
	return &Reader{files: map[string]found{}, seen: map[string]cached{}, now: time.Now}
}

// Head is where dir's repository's HEAD points; ok is false when dir isn't
// in a repository (or its HEAD can't be read). A nil Reader knows none.
func (r *Reader) Head(dir string) (Head, bool) {
	if r == nil || dir == "" {
		return Head{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f, known := r.files[dir]
	if !known || (f.file == "" && r.now().Sub(f.at) > missAge) {
		f = found{file: headFile(dir), at: r.now()}
		r.files[dir] = f
	}
	file := f.file
	if file == "" {
		return Head{}, false
	}
	fi, err := os.Stat(file)
	if err != nil {
		delete(r.files, dir) // the repository moved or went: look again next time
		return Head{}, false
	}
	if c, ok := r.seen[file]; ok && c.mod.Equal(fi.ModTime()) {
		return c.head, c.ok
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return Head{}, false
	}
	h, ok := parse(string(b))
	r.seen[file] = cached{mod: fi.ModTime(), head: h, ok: ok}
	return h, ok
}
