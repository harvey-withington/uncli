package session

import (
	"strings"
	"testing"

	"uncli/internal/adapter/claude"
	"uncli/internal/core"
)

func fixtureTranscript(t *testing.T) core.Transcript {
	t.Helper()
	loc := core.TranscriptLocation{ConfigDir: "../../testdata/transcripts/claude/" + claude.PinnedVersion}
	tr, err := claude.New(nil).ReadTranscript(loc, "939053cc-a279-4983-bad0-d37f5fab3c0d")
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// The recorded folder is gone on this computer: the session is imported
// archived, with a page per turn.
func TestImportMakesPagesAndArchivesWhenTheFolderIsGone(t *testing.T) {
	h := newHarness(t, "single-turn")
	tr := fixtureTranscript(t)
	got, err := h.m.Import(tr, "code", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if !got.FolderGone || !got.View.Archived || got.PagesLoaded != 4 || got.View.ProviderSID != tr.Info.ID {
		t.Fatalf("imported = %+v", got)
	}
	if !strings.HasPrefix(got.View.Title, "Use the Write tool") {
		t.Errorf("title = %q", got.View.Title)
	}
	pages, _ := h.m.Pages(got.View.ID)
	if len(pages) != 4 || pages[0].Trace[0].Summary != "Write hello.txt" || pages[0].TouchedFiles[0].Added != 1 ||
		pages[2].Question != "/compact" || pages[3].AnswerMD != "AFTER" || pages[0].OutputTokens == 0 {
		t.Errorf("pages = %+v", pages)
	}
	// Twice is refused, pointing at the session that has it.
	again, err := h.m.Import(tr, "code", "haiku")
	if err == nil || again.View.ID != got.View.ID {
		t.Errorf("second import = %+v, %v", again, err)
	}
}

// With its folder there, an imported session continues the CLI's own
// conversation: the next turn resumes it.
func TestImportedSessionResumes(t *testing.T) {
	h := newHarness(t, "single-turn")
	tr := fixtureTranscript(t)
	tr.Info.Workdir = h.dir
	got, err := h.m.Import(tr, "code", "haiku")
	if err != nil || got.FolderGone || got.View.Archived {
		t.Fatalf("imported = %+v, %v", got, err)
	}
	pages := h.send(got.View.ID, "and then?")
	if len(pages) != 5 || pages[4].Seq != 5 {
		t.Errorf("pages after a turn = %d", len(pages))
	}
	args := strings.Join(h.rt.starts[0].Args, " ")
	if !strings.Contains(args, "--resume "+tr.Info.ID) {
		t.Errorf("the CLI wasn't resumed: %s", args)
	}
}
