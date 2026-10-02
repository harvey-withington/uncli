package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func searchStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "uncli.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func addPage(t *testing.T, s *Store, sessionID string, seq int, question, answer string, extra ...func(*Page)) *Page {
	t.Helper()
	p := &Page{ID: fmt.Sprintf("%s-p%d", sessionID, seq), SessionID: sessionID, Seq: seq, Question: question, Model: "sonnet",
		AnswerMD: answer, Status: "done", StartedAt: time.Now().UnixMilli()}
	for _, f := range extra {
		f(p)
	}
	if err := s.SavePage(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func ids(r SearchResult) []string {
	out := []string{}
	for _, h := range r.Hits {
		out = append(out, h.PageID)
	}
	return out
}

func TestSearchFindsAcrossSessions(t *testing.T) {
	s := searchStore(t)
	s.CreateSession(&Session{ID: "a", Title: "Lisbon trip", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	s.CreateSession(&Session{ID: "b", Title: "Parser bug", Adapter: "claude", Runtime: "local", ProfileID: "code", Workdir: "y", Model: "opus"})
	addPage(t, s, "a", 1, "Plan a weekend", "Pastéis de nata at **Belém**, then the Jerónimos Monastery.")
	addPage(t, s, "b", 1, "Why does the test flake?", "The parser resets its state on every `system/status` line.")
	addPage(t, s, "b", 2, "Fix it", "Only the result line resets turn state now.", func(p *Page) {
		p.Attachments = []PageAttachment{{Name: "parser_trace.log", MediaType: "text/plain", Size: 10}}
	})

	cases := []struct {
		q    string
		want []string
	}{
		{"jeronimos", []string{"a-p1"}},           // accents folded
		{"JERÓNIMOS monastery", []string{"a-p1"}}, // case, and every word must match
		{"pars", []string{"b-p1", "b-p2"}},        // the last word is a prefix while typing (b-p2 via its attachment parser_trace.log)
		{"resets", []string{"b-p1", "b-p2"}},
		{`"resets turn state"`, []string{"b-p2"}}, // a phrase
		{`"turn resets"`, []string{}},             // the phrase in another order doesn't match
		{"lisbon", []string{"a-p1"}},              // session title
		{"parser_trace", []string{"b-p2"}},        // attachment name
		{"nothing-like-this", []string{}},
		{"", []string{}},
	}
	for _, c := range cases {
		r, err := s.Search(SearchQuery{Text: c.q})
		if err != nil {
			t.Fatalf("%q: %v", c.q, err)
		}
		got := ids(r)
		if strings.Join(sorted(got), ",") != strings.Join(sorted(c.want), ",") {
			t.Errorf("%q = %v, want %v", c.q, got, c.want)
		}
	}
}

func sorted(v []string) []string {
	out := append([]string{}, v...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestSearchSnippetsAndFields(t *testing.T) {
	s := searchStore(t)
	s.CreateSession(&Session{ID: "a", Title: "Trip", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	addPage(t, s, "a", 1, "Plan a weekend in Lisbon", "Start in Alfama. "+strings.Repeat("Walk on. ", 30)+"Finish at the Miradouro.")
	r, _ := s.Search(SearchQuery{Text: "miradouro"})
	if len(r.Hits) != 1 || r.Hits[0].Field != "answer" || !strings.Contains(r.Hits[0].Snippet, markOpen+"Miradouro"+markClose) {
		t.Fatalf("hits = %+v", r.Hits)
	}
	if !strings.HasPrefix(r.Hits[0].Snippet, "…") || r.Hits[0].SessionTitle != "Trip" || r.Hits[0].ProfileID != "chat" || r.Hits[0].Seq != 1 {
		t.Errorf("hit = %+v", r.Hits[0])
	}
	r, _ = s.Search(SearchQuery{Text: "weekend"})
	if r.Hits[0].Field != "question" || r.Hits[0].Question != "Plan a weekend in Lisbon" {
		t.Errorf("question hit = %+v", r.Hits[0])
	}
	if strings.Join(r.Terms, ",") != "weekend" {
		t.Errorf("terms = %v", r.Terms)
	}
}

// The index follows the pages: new answers, summaries, renames and deletes.
func TestSearchStaysInStep(t *testing.T) {
	s := searchStore(t)
	s.CreateSession(&Session{ID: "a", Title: "Old name", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	p := addPage(t, s, "a", 1, "Q", "")
	count := func(q string) int {
		r, err := s.Search(SearchQuery{Text: q})
		if err != nil {
			t.Fatal(err)
		}
		return len(r.Hits)
	}
	p.AnswerMD = "Streaming finished: kumquat."
	s.SavePage(p)
	if count("kumquat") != 1 {
		t.Error("an updated answer must be searchable")
	}
	p.AnswerMD = "Changed."
	s.SavePage(p)
	if count("kumquat") != 0 || count("changed") != 1 {
		t.Error("the old answer must leave the index")
	}
	s.SetPageOutline(p.ID, &PageOutline{Sections: []OutlineSection{{Block: 0, Title: "Quokka facts"}}})
	if count("quokka") != 1 {
		t.Error("summary titles must be searchable")
	}
	x, _ := s.GetSession("a")
	x.Title = "Brand new title"
	s.UpdateSession(&x)
	if count("brand") != 1 || count("old name") != 0 {
		t.Error("a rename must update the index")
	}
	s.DeleteSession("a")
	if count("changed") != 0 {
		t.Error("a deleted session must leave the index")
	}
	var rows int
	s.db.QueryRow(`SELECT COUNT(*) FROM search_rows`).Scan(&rows)
	if rows != 0 {
		t.Errorf("%d index rows left after delete", rows)
	}
}

func TestSearchFilters(t *testing.T) {
	s := searchStore(t)
	s.CreateSession(&Session{ID: "a", Title: "A", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	s.CreateSession(&Session{ID: "b", Title: "B", Adapter: "claude", Runtime: "local", ProfileID: "code", Workdir: "y", Model: "opus"})
	old := time.Now().Add(-40 * 24 * time.Hour).UnixMilli()
	addPage(t, s, "a", 1, "Q", "widget one", func(p *Page) { p.StartedAt = old })
	addPage(t, s, "a", 2, "Q", "widget two", func(p *Page) { p.Bookmarked = true })
	addPage(t, s, "b", 1, "Q", "widget three")
	check := func(q SearchQuery, want ...string) {
		t.Helper()
		q.Text = "widget"
		r, err := s.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(sorted(ids(r)), ",") != strings.Join(sorted(want), ",") {
			t.Errorf("%+v = %v, want %v", q, ids(r), want)
		}
	}
	check(SearchQuery{}, "a-p1", "a-p2", "b-p1")
	check(SearchQuery{Profiles: []string{"code"}}, "b-p1")
	check(SearchQuery{SessionID: "a"}, "a-p1", "a-p2")
	check(SearchQuery{Bookmarked: true}, "a-p2")
	check(SearchQuery{Since: time.Now().Add(-7 * 24 * time.Hour).UnixMilli()}, "a-p2", "b-p1")
	check(SearchQuery{Until: time.Now().Add(-7 * 24 * time.Hour).UnixMilli()}, "a-p1")
	check(SearchQuery{SessionID: "a", Bookmarked: true}, "a-p2")
}

// Recent pages come first when the match is equally good.
func TestSearchPrefersRecent(t *testing.T) {
	s := searchStore(t)
	s.CreateSession(&Session{ID: "a", Title: "A", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	addPage(t, s, "a", 1, "Q", "same words here", func(p *Page) { p.StartedAt = time.Now().Add(-300 * 24 * time.Hour).UnixMilli() })
	addPage(t, s, "a", 2, "Q", "same words here")
	r, _ := s.Search(SearchQuery{Text: "same words"})
	if got := ids(r); len(got) != 2 || got[0] != "a-p2" {
		t.Errorf("order = %v", got)
	}
}

// Whatever the user types, the query is valid FTS5.
func TestSearchHostileInput(t *testing.T) {
	s := searchStore(t)
	s.CreateSession(&Session{ID: "a", Title: "A", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	addPage(t, s, "a", 1, "Q", "C++ and node-js: NEAR (things) AND OR NOT")
	for _, q := range []string{`"`, `"""`, `*`, `-`, `:`, `(`, `)`, `^`, `NEAR(`, `AND`, `OR NOT`, `a:b`, `col:x`, `"unclosed phrase`, `{x}`, `c++`, `node-js`, `'`, `\`, `*foo`} {
		if _, err := s.Search(SearchQuery{Text: q}); err != nil {
			t.Errorf("%q: %v", q, err)
		}
	}
	r, _ := s.Search(SearchQuery{Text: "node-js"})
	if len(r.Hits) != 1 {
		t.Errorf("node-js should match as two words: %+v", r)
	}
	r, _ = s.Search(SearchQuery{Text: "NEAR"})
	if len(r.Hits) != 1 {
		t.Errorf("FTS5 keywords are plain words: %+v", r)
	}
}

func TestFTSQuery(t *testing.T) {
	cases := map[string]string{
		"lisbon":           `"lisbon"*`,
		"lisbon ":          `"lisbon"`,
		"weekend lisb":     `"weekend" "lisb"*`,
		`"turn state" now`: `"turn state" "now"*`,
		`now "turn state"`: `"turn state" "now"`, // phrases first; every term must match either way
		"deploy* go":       `"deploy"* "go"*`,
		`say "hi`:          `"say" "hi"*`,
		"a-b c:d":          `"a" "b" "c" "d"*`,
		"   ":              ``,
		"Jerónimos":        `"Jerónimos"*`,
	}
	for in, want := range cases {
		if got, _ := FTSQuery(in); got != want {
			t.Errorf("FTSQuery(%q) = %s, want %s", in, got, want)
		}
	}
}

// Opening an existing database indexes its history once.
func TestSearchIndexesExistingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uncli.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.CreateSession(&Session{ID: "a", Title: "A", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "x", Model: "sonnet"})
	addPage(t, s, "a", 1, "Q", "historical aardvark")
	// Simulate a database from before search: drop the index.
	for _, q := range []string{`DROP TABLE search`, `DROP TABLE search_rows`} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, _ := s.Search(SearchQuery{Text: "aardvark"})
	if len(r.Hits) != 1 {
		t.Errorf("history not indexed: %+v", r)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM search_rows`).Scan(&n)
	if n != 1 {
		t.Errorf("rows = %d", n)
	}
	_ = sql.ErrNoRows
}

// Speed at scale: go test -run TestSearchSpeed -v ./internal/store (with UNCLI_SEARCH_SPEED=1).
func TestSearchSpeed(t *testing.T) {
	if os.Getenv("UNCLI_SEARCH_SPEED") == "" {
		t.Skip("set UNCLI_SEARCH_SPEED=1 to run")
	}
	s := searchStore(t)
	words := strings.Fields("the parser session answer model token stream lisbon weekend deploy build test cache index query page outline summary kind")
	const sessions, perSession = 200, 100
	tx, _ := s.db.Begin()
	for i := 0; i < sessions; i++ {
		tx.Exec(`INSERT INTO sessions (id, title, adapter, runtime, profile_id, workdir, model, modifiers) VALUES (?,?,?,?,?,?,?,?)`,
			fmt.Sprint("s", i), fmt.Sprint("Session ", i, " ", words[i%len(words)]), "claude", "local", "chat", "x", "sonnet", "[]")
		for j := 0; j < perSession; j++ {
			var b strings.Builder
			for k := 0; k < 400; k++ { // about 400 words per answer
				b.WriteString(words[(i*7+j*13+k*k)%len(words)])
				b.WriteByte(' ')
			}
			id := fmt.Sprint("s", i, "-p", j)
			tx.Exec(`INSERT INTO pages (id, session_id, seq, question, model, modifiers, answer_md, status, started_at) VALUES (?,?,?,?,?,?,?,?,?)`,
				id, fmt.Sprint("s", i), j+1, "question "+words[j%len(words)], "sonnet", "[]", b.String(), "done", time.Now().UnixMilli())
			if err := indexPage(tx, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"lisbon", "parser sess", `"deploy build"`, "kin"} {
		start := time.Now()
		r, err := s.Search(SearchQuery{Text: q})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%-16q %3d hits in %v (of %d pages)", q, len(r.Hits), time.Since(start), sessions*perSession)
	}
}
