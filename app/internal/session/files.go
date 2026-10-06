package session

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"uncli/internal/snapshot"
	"uncli/internal/store"
)

// Files a turn changed. Tools that write report their file once they
// succeed (EvFileTouched); for anything else, such as a command, the
// session takes a snapshot of its folder as the turn starts and compares
// it when a turn that used tools ends (decision record 0007). Changes the
// user makes between turns aren't credited to Claude.

// turnFiles is what a turn needs to find its changes when it ends. For a
// session type with artifacts, its artifacts folder is compared too, and
// each new or changed artifact is kept as a version (decision record 0008).
type turnFiles struct {
	done      chan struct{} // closed once the snapshots are taken
	before    snapshot.Snapshot
	artifacts snapshot.Snapshot // Root empty: no artifacts folder
	tools     bool              // the turn used a tool, so something may have changed
}

func startTurnFiles(root, artifactsRoot string) *turnFiles {
	t := &turnFiles{done: make(chan struct{})}
	go func() {
		t.before = snapshot.Take(context.Background(), root)
		if artifactsRoot != "" {
			t.artifacts = snapshot.TakeFolder(artifactsRoot)
		}
		close(t.done)
	}()
	return t
}

// finishFiles compares the session folder (and artifacts folder) with
// their state when the turn began, adds what changed to the closed page p
// and tells the UI. It runs on its own goroutine after the page closed.
func (s *Session) finishFiles(t *turnFiles, p *store.Page) {
	<-t.done
	changes := snapshot.Diff(t.before, snapshot.Take(context.Background(), t.before.Root))
	versions := s.keepArtifacts(t.artifacts)
	if len(changes) == 0 && len(versions) == 0 {
		return
	}
	s.mu.Lock()
	for _, c := range changes {
		how := HowCommand
		if c.Kind == snapshot.Deleted {
			how = HowDeleted
		}
		p.TouchedFiles = mergeTouched(p.TouchedFiles, store.TouchedFile{Path: c.Path, How: how})
	}
	for _, v := range versions {
		p.Artifacts = mergeArtifact(p.Artifacts, v)
	}
	touched := append([]store.TouchedFile{}, p.TouchedFiles...)
	artifacts := append([]store.ArtifactVersion{}, p.Artifacts...)
	s.mu.Unlock()
	st := s.m.d.Store
	if err := st.SetPageFiles(p.ID, touched, artifacts); err != nil {
		return
	}
	if fresh, err := st.Page(p.ID); err == nil {
		s.m.d.Sink.PageChanged(fresh)
	}
}

// keepArtifacts compares the artifacts folder with before and stores each
// new or changed file as a version. A file the store won't keep (too big,
// or unreadable) is recorded without a hash.
func (s *Session) keepArtifacts(before snapshot.Snapshot) []store.ArtifactVersion {
	if before.Root == "" || s.m.d.Artifacts == nil {
		return nil
	}
	var out []store.ArtifactVersion
	for _, c := range snapshot.Diff(before, snapshot.TakeFolder(before.Root)) {
		rel, err := filepath.Rel(before.Root, c.Path)
		if err != nil {
			continue
		}
		v := store.ArtifactVersion{Path: filepath.ToSlash(rel)}
		if c.Kind == snapshot.Deleted {
			v.Deleted = true
		} else {
			v.Hash, v.Size, _ = s.m.d.Artifacts.Put(c.Path)
		}
		out = append(out, v)
	}
	return out
}

// mergeArtifact puts v in the list, replacing an earlier version of the
// same artifact from the same turn.
func mergeArtifact(list []store.ArtifactVersion, v store.ArtifactVersion) []store.ArtifactVersion {
	for i := range list {
		if list[i].Path == v.Path {
			list[i] = v
			return list
		}
	}
	return append(list, v)
}

// How a file was touched.
const (
	HowWrite   = "write"   // a tool created or replaced it
	HowEdit    = "edit"    // a tool edited it
	HowCommand = "command" // a command changed or created it
	HowDeleted = "deleted" // a command deleted it
)

// mergeTouched adds f to the list, one entry per file. What a tool says
// beats what a snapshot found (it knows the line), creating beats editing,
// a later edit's line replaces an earlier one, the tools' line counts add
// up, and a delete is the end of it.
func mergeTouched(list []store.TouchedFile, f store.TouchedFile) []store.TouchedFile {
	key := fileKey(f.Path)
	for i := range list {
		cur := &list[i]
		if fileKey(cur.Path) != key {
			continue
		}
		switch {
		case f.How == HowDeleted:
			cur.How, cur.Line, cur.Added, cur.Removed = HowDeleted, 0, 0, 0
		case f.How == HowCommand:
			if cur.How == HowDeleted {
				cur.How = HowCommand // made again
			}
		default: // a tool
			if cur.How != HowWrite || f.How == HowWrite {
				cur.How = f.How
			}
			if f.Line > 0 {
				cur.Line = f.Line
			}
			cur.Added += f.Added
			cur.Removed += f.Removed
		}
		return list
	}
	return append(list, f)
}

// fileKey compares paths the way the file system does: Windows ignores
// case and either slash.
func fileKey(p string) string {
	p = filepath.Clean(filepath.FromSlash(p))
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}
