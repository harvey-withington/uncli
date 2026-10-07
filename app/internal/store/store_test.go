package store

import (
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "uncli.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestSessionsAndPagesSurviveReopen(t *testing.T) {
	s, path := open(t)
	sess := &Session{ID: "s1", Title: "Hello", Adapter: "claude", Runtime: "local", ProfileID: "chat",
		Workdir: "/w", Model: "sonnet", Modifiers: []string{"efficiency"}}
	if err := s.CreateSession(sess); err != nil {
		t.Fatal(err)
	}
	sess.ProviderSID, sess.CLIVersion = "prov-1", "2.1.285"
	if err := s.UpdateSession(sess); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		seq, _ := s.NextSeq("s1")
		p := &Page{ID: "p" + string(rune('0'+i)), SessionID: "s1", Seq: seq, Question: "q", Model: "sonnet",
			Modifiers: []string{"efficiency"}, Status: "open", StartedAt: 1}
		if err := s.SavePage(p); err != nil {
			t.Fatal(err)
		}
		p.AnswerMD, p.Status, p.CostUSD, p.OutputTokens = "**a**", "done", 0.01, 42
		p.Trace = []TraceItem{{ID: "t1", Name: "Bash", Summary: "Bash ls", Done: true, OK: true}}
		if err := s.SavePage(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetBookmark("p2", true); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	list, _ := s2.ListSessions()
	if len(list) != 1 || list[0].ProviderSID != "prov-1" || !slices.Equal(list[0].Modifiers, []string{"efficiency"}) {
		t.Fatalf("sessions = %+v", list)
	}
	pages, _ := s2.ListPages("s1")
	if len(pages) != 3 || pages[1].Seq != 2 || !pages[1].Bookmarked || pages[0].Bookmarked {
		t.Fatalf("pages = %+v", pages)
	}
	if p := pages[2]; p.AnswerMD != "**a**" || p.Status != "done" || p.OutputTokens != 42 || len(p.Trace) != 1 || !p.Trace[0].OK {
		t.Errorf("page = %+v", p)
	}
}

func TestMarkOpenPagesAndErrors(t *testing.T) {
	s, _ := open(t)
	s.CreateSession(&Session{ID: "s", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "/", Model: "m"})
	s.SavePage(&Page{ID: "p", SessionID: "s", Seq: 1, Question: "q", Model: "m", Status: "open"})
	if err := s.MarkOpenPages("interrupted"); err != nil {
		t.Fatal(err)
	}
	pages, _ := s.ListPages("s")
	if pages[0].Status != "interrupted" || pages[0].FinishedAt == 0 {
		t.Errorf("page = %+v", pages[0])
	}
	pages[0].Error = "Not logged in"
	s.SavePage(&pages[0])
	pages, _ = s.ListPages("s")
	if pages[0].Error != "Not logged in" {
		t.Errorf("error lost: %+v", pages[0])
	}
	if err := s.SetBookmark("missing", true); err == nil {
		t.Error("bookmarking a missing page should fail")
	}
}

func TestEventCapPrunesOldestPages(t *testing.T) {
	s, _ := open(t)
	s.EventCap = 10
	for page := 1; page <= 3; page++ {
		for n := 0; n < 6; n++ {
			s.AppendEvent("s", page, n, "text_delta", time.Now(), []byte(`{}`), []byte(`{}`))
		}
		if err := s.PruneEvents("s"); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := s.CountEvents("s"); n != 6 {
		t.Errorf("events = %d, want only the newest page's 6", n)
	}
}

func TestSettingsAndDelete(t *testing.T) {
	s, _ := open(t)
	if v, err := s.Setting("x"); err != nil || v != "" {
		t.Fatal(v, err)
	}
	s.SetSetting("x", "1")
	s.SetSetting("x", "2")
	if v, _ := s.Setting("x"); v != "2" {
		t.Errorf("setting = %q", v)
	}
	s.CreateSession(&Session{ID: "s", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "/", Model: "m"})
	s.SavePage(&Page{ID: "p", SessionID: "s", Seq: 1, Question: "q", Model: "m", Status: "done"})
	if err := s.DeleteSession("s"); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.ListSessions(); len(l) != 0 {
		t.Error("session not deleted")
	}
}

func TestPageOutline(t *testing.T) {
	s, path := open(t)
	s.CreateSession(&Session{ID: "s", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "/", Model: "m"})
	s.SavePage(&Page{ID: "p", SessionID: "s", Seq: 1, Question: "q", Model: "m", Status: "done"})
	o := &PageOutline{Provider: "claude", Model: "haiku", Sections: []OutlineSection{{Block: 0, Title: "Intro"}, {Block: 3, Title: "Details", Kind: "analysis"}}, CostUSD: 0.004}
	if err := s.SetPageOutline("p", o); err != nil {
		t.Fatal(err)
	}
	// A later save of the page (bookmark, status) must keep the outline.
	s.SavePage(&Page{ID: "p", SessionID: "s", Seq: 1, Question: "q", Model: "m", Status: "done", AnswerMD: "x"})
	s.Close()
	s2, _ := Open(path)
	defer s2.Close()
	pages, _ := s2.ListPages("s")
	if pages[0].Outline == nil || len(pages[0].Outline.Sections) != 2 || pages[0].Outline.Sections[1].Title != "Details" {
		t.Fatalf("outline = %+v", pages[0].Outline)
	}
	s2.SetPageOutline("p", nil)
	if pages, _ := s2.ListPages("s"); pages[0].Outline != nil {
		t.Error("outline not cleared")
	}
}

// A database made before the outline column existed gets it on open.
func TestMigrationAddsOutline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, _ := sql.Open("sqlite", "file:"+path)
	db.Exec(schema) // the original schema, without the outline column
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CreateSession(&Session{ID: "s", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "/", Model: "m"})
	s.SavePage(&Page{ID: "p", SessionID: "s", Seq: 1, Question: "q", Model: "m", Status: "done"})
	if err := s.SetPageOutline("p", &PageOutline{Model: "haiku"}); err != nil {
		t.Fatal(err)
	}
}

// A bookmark or pin set while the answer streams survives the page's next
// saves, which carry the session's own (older) copy.
func TestSavePageKeepsMarks(t *testing.T) {
	s, _ := open(t)
	_ = s.CreateSession(&Session{ID: "s1", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "/w", Model: "m"})
	p := &Page{ID: "p1", SessionID: "s1", Seq: 1, Question: "q", Model: "m", Status: "open"}
	if err := s.SavePage(p); err != nil {
		t.Fatal(err)
	}
	_ = s.SetBookmark("p1", true)
	_ = s.SetPinned("p1", true)
	p.AnswerMD, p.Status = "a", "done" // Bookmarked and Pinned still false here
	if err := s.SavePage(p); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Page("p1")
	if !got.Bookmarked || !got.Pinned || got.AnswerMD != "a" {
		t.Errorf("page = %+v", got)
	}
	if err := s.SetPinned("missing", true); err == nil {
		t.Error("pinned a missing page")
	}
}
