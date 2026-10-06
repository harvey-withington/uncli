// Package snapshot notices which files changed in a folder between two
// moments (the start and end of a turn), without watching it: it records
// each file's size and modification time and compares. In a git repo the
// files are those git would list (tracked, plus untracked files that
// aren't ignored), so .gitignore is honoured; elsewhere a walk that skips
// hidden and generated folders. Decision record 0007.
package snapshot

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"uncli/internal/core"
	"uncli/internal/runtime/local"
)

// Limit is the most files a snapshot records; a bigger folder isn't
// compared at all (Complete is false), since statting it twice a turn
// would cost too much.
const Limit = 20000

type entry struct {
	size int64
	mod  int64 // modification time, ns
}

// Snapshot is a folder's files at one moment.
type Snapshot struct {
	Root     string
	Complete bool // every file was recorded; an incomplete snapshot finds no changes
	files    map[string]entry
}

// Change is a file that was added, changed or deleted.
type Change struct {
	Path string // absolute
	Kind string // added | changed | deleted
}

// Change kinds.
const (
	Added   = "added"
	Changed = "changed"
	Deleted = "deleted"
)

// Take records the files under root.
func Take(ctx context.Context, root string) Snapshot {
	names, ok := gitFiles(ctx, root)
	if !ok {
		names, ok = walk(root, true)
	}
	return record(root, names, ok)
}

// TakeFolder records every file under root, git or not, skipping nothing
// (an artifacts folder: what's in it is the point, ignored or not). A
// missing folder is an empty one.
func TakeFolder(root string) Snapshot {
	if _, err := os.Stat(root); err != nil {
		return Snapshot{Root: root, Complete: true, files: map[string]entry{}}
	}
	names, ok := walk(root, false)
	return record(root, names, ok)
}

func record(root string, names []string, ok bool) Snapshot {
	s := Snapshot{Root: root, files: map[string]entry{}}
	if !ok || len(names) > Limit {
		return s
	}
	for _, n := range names {
		p := filepath.Join(root, filepath.FromSlash(n))
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		s.files[p] = entry{size: st.Size(), mod: st.ModTime().UnixNano()}
	}
	s.Complete = true
	return s
}

// Diff lists what changed from before to after, sorted by path. Either
// snapshot being incomplete, or of another folder, means no changes.
func Diff(before, after Snapshot) []Change {
	if !before.Complete || !after.Complete || before.Root != after.Root {
		return nil
	}
	var out []Change
	for p, a := range after.files {
		b, had := before.files[p]
		switch {
		case !had:
			out = append(out, Change{Path: p, Kind: Added})
		case a != b:
			out = append(out, Change{Path: p, Kind: Changed})
		}
	}
	for p := range before.files {
		if _, still := after.files[p]; !still {
			out = append(out, Change{Path: p, Kind: Deleted})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// gitFiles lists the files git would show for a folder in a repo: tracked,
// and untracked but not ignored. ok is false outside a repo (or without git).
func gitFiles(ctx context.Context, root string) ([]string, bool) {
	git, err := lookGit()
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := local.Run(ctx, core.Command{Path: git, Args: []string{"-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard"}})
	if err != nil {
		return nil, false
	}
	var names []string
	for _, n := range bytes.Split(out, []byte{0}) {
		if len(n) > 0 {
			names = append(names, string(n))
		}
		if len(names) > Limit {
			break
		}
	}
	return names, true
}

// skipped are folders a walk doesn't go into: what builds and package
// managers make, which a change to is noise.
var skipped = map[string]bool{
	"node_modules": true, "dist": true, "build": true, "out": true, "target": true, "coverage": true,
	"__pycache__": true, "venv": true, "vendor": true, "bin": true, "obj": true, "pods": true,
	"_build": true, "deps": true, "tmp": true, "temp": true,
}

// walk lists a folder's files (as slash paths relative to root), skipping
// hidden and generated folders when skip is set. ok is false when there
// are too many.
func walk(root string, skip bool) ([]string, bool) {
	var names []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil // unreadable: leave it out
		}
		if d.IsDir() {
			name := strings.ToLower(d.Name())
			if skip && p != root && (strings.HasPrefix(name, ".") || skipped[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err == nil {
			names = append(names, filepath.ToSlash(rel))
		}
		if len(names) > Limit {
			return fs.SkipAll
		}
		return nil
	})
	return names, err == nil && len(names) <= Limit
}
