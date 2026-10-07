package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"uncli/internal/adapter/claude"
	"uncli/internal/core"
)

// A fixture transcript copied into a temp config folder, its recorded
// folder rewritten to one that exists here.
func importFixture(t *testing.T, s *Service, workdir string) string {
	t.Helper()
	const id = "939053cc-a279-4983-bad0-d37f5fab3c0d"
	src := filepath.Join("..", "..", "testdata", "transcripts", "claude", claude.PinnedVersion, "projects", "C--uncli-spike-w-import", id+".jsonl")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if workdir != "" {
		// The recorded folder, as JSON writes it (backslashes doubled).
		b = bytes.ReplaceAll(b, []byte(`C:\\uncli-spike\\w-import`), []byte(filepath.ToSlash(workdir)))
	}
	cfg := t.TempDir()
	dir := filepath.Join(cfg, "projects", "x")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	s.transcripts = &core.TranscriptLocation{ConfigDir: cfg}
	return id
}

func TestTranscriptsListAndImport(t *testing.T) {
	s := newTestService(t)
	// A deleted UNCLI chat: its folder is a scratch folder that still exists.
	chat := filepath.Join(s.Paths.Scratch, "old-chat")
	_ = os.MkdirAll(chat, 0o755)
	id := importFixture(t, s, chat)

	list, err := s.Transcripts()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v %v", list, err)
	}
	e := list[0]
	if !e.FromChat || e.Profile != "chat" || e.FolderGone || e.SessionID != "" || e.Turns != 4 {
		t.Errorf("entry = %+v", e)
	}

	got, err := s.ImportTranscript(id, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.View.ProfileID != "chat" || got.View.Model != "haiku" || got.PagesLoaded != 4 {
		t.Errorf("imported = %+v", got)
	}
	if list, _ := s.Transcripts(); list[0].SessionID != got.View.ID {
		t.Errorf("not marked as imported: %+v", list[0])
	}
}
