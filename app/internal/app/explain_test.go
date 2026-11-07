package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/store"
)

var explainPages = []store.Page{
	{ID: "p1", Question: "Set up the project", AnswerMD: "Made the module."},
	{ID: "p2", Question: "Add a greeting", AnswerMD: "Added greet()."},
	{ID: "p3", Question: "Make greet handle late nights", AnswerMD: "Greet now says Up late before 5am, because the user works nights."},
}

var explainChanges = []store.FileChange{
	{Path: `C:\w\src\greet.js`, Hunks: []store.Hunk{{OldStart: 1, NewStart: 1, Lines: []string{" function greet(name) {", "+  if (hour < 5) return 'Up late'", " }"}}}},
	{Path: `C:\w\README.md`, Hunks: []store.Hunk{{OldStart: 3, NewStart: 3, Lines: []string{"-Says hello", "+Says hello, or up late"}}, {OldStart: 9, NewStart: 9, Lines: []string{"+More"}}}},
}

func TestExplainPrompt(t *testing.T) {
	prompt, ids := explainPrompt(explainPages, 2, explainChanges, `C:\w`, ExplainOptions{})
	if !slices.Equal(ids, []string{"c0h0", "c1h0", "c1h1"}) {
		t.Errorf("ids = %v", ids)
	}
	for _, want := range []string{"### c0h0: src/greet.js (from line 1)", "+  if (hour < 5)", "### c1h1: README.md", "because the user works nights", "50 words at most"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	// This page only, by default: nothing earlier.
	if strings.Contains(prompt, "Set up the project") || strings.Contains(prompt, "Add a greeting") {
		t.Error("the page's prompt had earlier pages")
	}
	// The last page before it, or the whole session, latest first.
	if p, _ := explainPrompt(explainPages, 2, explainChanges, `C:\w`, ExplainOptions{Context: "pages", Pages: 1}); !strings.Contains(p, "Add a greeting") || strings.Contains(p, "Set up the project") {
		t.Error("the last page wasn't the one before")
	}
	p, _ := explainPrompt(explainPages, 2, explainChanges, `C:\w`, ExplainOptions{Context: "session", Detail: "thorough"})
	if i, j := strings.Index(p, "Add a greeting"), strings.Index(p, "Set up the project"); i < 0 || j < 0 || i > j || !strings.Contains(p, "150 words") {
		t.Error("the whole session wasn't there, latest first, or thorough wasn't longer")
	}
}

func TestParseExplanation(t *testing.T) {
	ids := []string{"c0h0", "c1h0", "c1h1"}
	raw, _ := json.Marshal(map[string]any{"summary": " Greets night owls. ", "notes": []map[string]string{
		{"place": "c1h0", "text": "Says so in the README."},
		{"place": "c0h0", "text": "The early return."},
		{"place": "c9h9", "text": "A place that isn't there."},
		{"place": "c0h0", "text": "A second note for the same place."},
		{"place": "c1h1", "text": "  "},
	}})
	e, err := parseExplanation(core.TextResult{Structured: raw}, ids)
	if err != nil {
		t.Fatal(err)
	}
	if e.Summary != "Greets night owls." || len(e.Notes) != 2 || e.Notes[0].Place != "c0h0" || e.Notes[0].Text != "The early return." || e.Notes[1].Place != "c1h0" {
		t.Errorf("explanation = %+v", e)
	}
	if _, err := parseExplanation(core.TextResult{Text: `{"summary":"","notes":[]}`}, ids); err == nil {
		t.Error("an empty answer was taken")
	}
	if _, err := parseExplanation(core.TextResult{Text: "Sorry, I can't."}, ids); err == nil {
		t.Error("prose was taken")
	}
}
