package store

import "testing"

// A page's explanation is kept apart from what the turn saves, so saving
// the page again doesn't lose it; a session's changes go with it.
func TestPageExplanationAndChanges(t *testing.T) {
	s, _ := open(t)
	s.CreateSession(&Session{ID: "s", Adapter: "claude", Runtime: "local", ProfileID: "code", Workdir: "/", Model: "m"})
	p := Page{ID: "p", SessionID: "s", Seq: 1, Question: "q", Model: "m", Status: "done"}
	if err := s.SavePage(&p); err != nil {
		t.Fatal(err)
	}
	if err := s.AddPageChange("s", "p", FileChange{Path: "/a.go", Hunks: []Hunk{{OldStart: 1, NewStart: 1, Lines: []string{"+x"}}}}); err != nil {
		t.Fatal(err)
	}
	e := &PageExplanation{Model: "opus", Context: "page", Detail: "brief", Summary: "Adds x.", Notes: []ExplainNote{{Place: "c0h0", Text: "The x."}}}
	if err := s.SetPageExplanation("p", e); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePage(&p); err != nil { // the turn saving its page again
		t.Fatal(err)
	}
	got, err := s.Page("p")
	if err != nil {
		t.Fatal(err)
	}
	if got.Explanation == nil || got.Explanation.Summary != "Adds x." || len(got.Explanation.Notes) != 1 || got.Explanation.Notes[0].Place != "c0h0" {
		t.Errorf("explanation = %+v", got.Explanation)
	}
	if err := s.SetPageExplanation("nope", e); err == nil {
		t.Error("explained a page that isn't there")
	}
	if err := s.DeleteSession("s"); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.PageChanges("s", "p"); len(c) != 0 {
		t.Errorf("changes outlived their session: %+v", c)
	}
}
