package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/artifacts"
	"uncli/internal/store"
)

func TestMergeTouched(t *testing.T) {
	var l []store.TouchedFile
	l = mergeTouched(l, store.TouchedFile{Path: "/w/a.go", How: HowEdit, Line: 10, Added: 3, Removed: 1})
	l = mergeTouched(l, store.TouchedFile{Path: "/w/a.go", How: HowEdit, Line: 42, Added: 2}) // a later edit's line; counts add up
	l = mergeTouched(l, store.TouchedFile{Path: "/w/a.go", How: HowCommand})                  // the snapshot saw it too
	l = mergeTouched(l, store.TouchedFile{Path: "/w/new.md", How: HowWrite})
	l = mergeTouched(l, store.TouchedFile{Path: "/w/new.md", How: HowEdit, Line: 3}) // created, then edited: still created
	l = mergeTouched(l, store.TouchedFile{Path: "/w/gone.txt", How: HowEdit, Line: 1, Added: 4})
	l = mergeTouched(l, store.TouchedFile{Path: "/w/gone.txt", How: HowDeleted})
	l = mergeTouched(l, store.TouchedFile{Path: "/w/built.js", How: HowCommand})
	want := []store.TouchedFile{
		{Path: "/w/a.go", How: HowEdit, Line: 42, Added: 5, Removed: 1},
		{Path: "/w/new.md", How: HowWrite, Line: 3},
		{Path: "/w/gone.txt", How: HowDeleted},
		{Path: "/w/built.js", How: HowCommand},
	}
	if len(l) != len(want) {
		t.Fatalf("list = %+v", l)
	}
	for i := range want {
		if l[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, l[i], want[i])
		}
	}
}

// An Edit that succeeded is on its page, once, at the line it changed.
func TestPageListsEditedFile(t *testing.T) {
	h := newHarness(t, "edit-tool-result")
	v, err := h.m.Create("code", h.dir, "haiku")
	if err != nil {
		t.Fatal(err)
	}
	pages := h.send(v.ID, "change six to SIX")
	got := pages[0].TouchedFiles
	if len(got) != 1 || got[0].How != HowEdit || got[0].Line != 6 || !strings.HasSuffix(got[0].Path, "notes.txt") {
		t.Errorf("touched = %+v", got)
	}
}

// What a turn changed without a tool saying so is found by comparing the
// session folder with its state when the turn started.
func TestFinishFilesFindsCommandChanges(t *testing.T) {
	h := newHarness(t, "single-turn")
	work := filepath.Join(h.dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "old.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := h.m.Create("code", work, "haiku")
	if err != nil {
		t.Fatal(err)
	}
	pages := h.send(v.ID, "hello")
	p := pages[0]
	p.Bookmarked = false
	if _, err := h.m.SetBookmark(v.ID, p.ID, true); err != nil { // set meanwhile: must survive
		t.Fatal(err)
	}

	turn := startTurnFiles(work, "")
	<-turn.done
	turn.tools = true
	if err := os.WriteFile(filepath.Join(work, "made.txt"), []byte("by a command"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(work, "old.txt"))
	s, _ := h.m.get(v.ID)
	s.finishFiles(turn, &p)

	saved, err := h.db.Page(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{}
	for _, f := range saved.TouchedFiles {
		byName[filepath.Base(f.Path)] = f.How
	}
	if byName["made.txt"] != HowCommand || byName["old.txt"] != HowDeleted || len(byName) != 2 {
		t.Errorf("touched = %+v", saved.TouchedFiles)
	}
	if !saved.Bookmarked {
		t.Error("the bookmark set meanwhile was lost")
	}
}

// A chat session keeps each new, changed or deleted artifact as a version
// on the page whose turn made it.
func TestFinishFilesKeepsArtifactVersions(t *testing.T) {
	h := newHarness(t, "single-turn")
	st := artifacts.NewStore(filepath.Join(h.dir, "store"))
	h.m.d.Artifacts = st
	v, err := h.m.Create("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(v.Workdir, artifacts.Folder)
	_ = os.WriteFile(filepath.Join(art, "old.md"), []byte("old"), 0o644)
	_ = os.WriteFile(filepath.Join(art, "keep.md"), []byte("same"), 0o644)
	p := h.send(v.ID, "hello")[0]

	turn := startTurnFiles(v.Workdir, art)
	<-turn.done
	turn.tools = true
	_ = os.MkdirAll(filepath.Join(art, "charts"), 0o755)
	_ = os.WriteFile(filepath.Join(art, "charts", "sales.svg"), []byte("<svg/>"), 0o644)
	_ = os.Remove(filepath.Join(art, "old.md"))
	s, _ := h.m.get(v.ID)
	s.finishFiles(turn, &p)

	saved, _ := h.db.Page(p.ID)
	got := map[string]store.ArtifactVersion{}
	for _, a := range saved.Artifacts {
		got[a.Path] = a
	}
	if len(got) != 2 || !got["old.md"].Deleted || got["charts/sales.svg"].Size != 6 {
		t.Fatalf("artifacts = %+v", saved.Artifacts)
	}
	if b, err := st.Read(got["charts/sales.svg"].Hash, 100); err != nil || string(b) != "<svg/>" {
		t.Errorf("stored version = %q %v", b, err)
	}
}
