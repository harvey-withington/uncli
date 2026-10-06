package app

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/ide"
)

func TestEditorPreference(t *testing.T) {
	s := newTestService(t)
	if got := s.Preferences().Editor; got != ide.Auto {
		t.Errorf("default editor = %q", got)
	}
	p := s.Preferences()
	p.Editor = "cursor"
	if saved, err := s.SetPreferences(p); err != nil || saved.Editor != "cursor" {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	p.Editor, p.EditorCommand = ide.Custom, ""
	if _, err := s.SetPreferences(p); err == nil {
		t.Error("custom without a command was accepted")
	}
	p.EditorCommand = "subl {file}:{line}"
	if saved, err := s.SetPreferences(p); err != nil || saved.EditorCommand != "subl {file}:{line}" {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	p.Editor = "notepad-plus-plus"
	if _, err := s.SetPreferences(p); err == nil {
		t.Error("an unknown editor was accepted")
	}
	if len(s.Bootstrap().Editors) != len(ide.Presets) {
		t.Error("bootstrap doesn't list the editors")
	}
}

// A chat session never opens a program a model wrote, only shows it.
func TestOpenFileRefusesProgramsOutsideTheEditor(t *testing.T) {
	s := newTestService(t)
	v, _, err := s.CreateSession("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(v.Workdir, "artifacts", "setup.exe")
	if err := os.WriteFile(exe, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(v.ID, exe, 0); err == nil || !strings.Contains(err.Error(), "Show in folder") {
		t.Errorf("err = %v", err)
	}
}

func TestReadArtifact(t *testing.T) {
	s := newTestService(t)
	v, _, err := s.CreateSession("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(v.Workdir, "artifacts", "page.html")
	if err := os.WriteFile(f, []byte("<p>hi</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, _ := s.ArtifactFiles(v.ID)
	if len(files) != 1 || files[0].Path != "page.html" {
		t.Fatalf("files = %+v", files)
	}
	live, err := s.ReadArtifact(v.ID, ArtifactRef{Path: "page.html"})
	if err != nil || live.Data != base64.StdEncoding.EncodeToString([]byte("<p>hi</p>")) {
		t.Fatalf("live = %+v %v", live, err)
	}
	hash, _, err := s.artifactStore.Put(f)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(f, []byte("changed"), 0o644)
	old, err := s.ReadArtifact(v.ID, ArtifactRef{Hash: hash})
	if err != nil || old.Size != 9 {
		t.Errorf("version = %+v %v", old, err)
	}
	if _, err := s.ReadArtifact(v.ID, ArtifactRef{Path: "../../uncli.db"}); err == nil {
		t.Error("read outside the artifacts folder")
	}
	code, _, _ := s.CreateSession("code", t.TempDir(), "haiku")
	if files, _ := s.ArtifactFiles(code.ID); len(files) != 0 {
		t.Error("a code session listed artifacts")
	}
}
