package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Search: a full-text index (SQLite FTS5) of every page's question and
// answer, its session's title, its attachment names and its summary's
// section titles. It is kept in step as pages are saved, summarised and
// deleted and sessions renamed; existing history is indexed once, when the
// index is first created.
//
// FTS rows are keyed by search_rows.id, an INTEGER PRIMARY KEY, because
// the implicit rowid of pages (a TEXT-keyed table) can change on VACUUM.
const searchSchema = `
CREATE TABLE IF NOT EXISTS search_rows (
  id         INTEGER PRIMARY KEY,
  page_id    TEXT NOT NULL UNIQUE,
  session_id TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS search_rows_session ON search_rows(session_id);
-- Covers what ranking and filters read from pages, so the large rows
-- (answer, trace) are never loaded for a search.
CREATE INDEX IF NOT EXISTS pages_search ON pages(id, started_at, bookmarked, seq);
CREATE VIRTUAL TABLE IF NOT EXISTS search USING fts5(
  title, question, answer, extra,
  tokenize = 'unicode61 remove_diacritics 2'
);
`

// Column weights for ranking: a match in the title or question counts
// for more than one in a long answer.
const searchRank = `bm25(search, 3.0, 4.0, 1.0, 1.5)`

// Snippet markers: control characters that can't occur in answers, so
// the UI can turn them into highlights without parsing HTML.
const (
	markOpen  = "\x01"
	markClose = "\x02"
)

func setupSearch(db *sql.DB) error {
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='search_rows'`).Scan(&n)
	fresh := n == 0
	if _, err := db.Exec(searchSchema); err != nil {
		return fmt.Errorf("search index: %w", err)
	}
	if !fresh {
		return nil
	}
	// First run with search: index the history once.
	rows, err := db.Query(`SELECT id FROM pages`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if err := indexPage(tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// indexPage (re)writes one page's row in the index from what's stored.
func indexPage(db execer, pageID string) error {
	var sessionID, question, title string
	var answer, attached, outline sql.NullString
	err := db.QueryRow(`SELECT p.session_id, p.question, p.answer_md, p.attachments, p.outline, COALESCE(s.title, '')
		FROM pages p LEFT JOIN sessions s ON s.id = p.session_id WHERE p.id=?`, pageID).
		Scan(&sessionID, &question, &answer, &attached, &outline, &title)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var extra []string
	var files []PageAttachment
	_ = json.Unmarshal([]byte(attached.String), &files)
	for _, f := range files {
		extra = append(extra, f.Name)
	}
	var o PageOutline
	if json.Unmarshal([]byte(outline.String), &o) == nil {
		for _, sec := range o.Sections {
			extra = append(extra, sec.Title)
		}
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO search_rows (page_id, session_id) VALUES (?, ?)`, pageID, sessionID); err != nil {
		return err
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM search_rows WHERE page_id=?`, pageID).Scan(&id); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM search WHERE rowid=?`, id); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO search (rowid, title, question, answer, extra) VALUES (?, ?, ?, ?, ?)`,
		id, title, question, answer.String, strings.Join(extra, "\n"))
	return err
}

// retitleSession updates the title on every indexed page of a session.
func retitleSession(db execer, sessionID, title string) error {
	_, err := db.Exec(`UPDATE search SET title=? WHERE rowid IN (SELECT id FROM search_rows WHERE session_id=?)`, title, sessionID)
	return err
}

func unindexSession(db execer, sessionID string) error {
	if _, err := db.Exec(`DELETE FROM search WHERE rowid IN (SELECT id FROM search_rows WHERE session_id=?)`, sessionID); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM search_rows WHERE session_id=?`, sessionID)
	return err
}

// SearchQuery is what the user typed plus optional filters.
type SearchQuery struct {
	Text       string   `json:"text"`
	Profiles   []string `json:"profiles,omitempty"`  // session types to include; empty = all
	SessionID  string   `json:"sessionId,omitempty"` // only this session
	Bookmarked bool     `json:"bookmarked,omitempty"`
	Since      int64    `json:"since,omitempty"` // page started at or after (ms)
	Until      int64    `json:"until,omitempty"` // page started before (ms)
	Limit      int      `json:"limit,omitempty"`
}

// SearchHit is one matching page. Snippet marks matches with \x01 … \x02.
type SearchHit struct {
	PageID       string `json:"pageId"`
	SessionID    string `json:"sessionId"`
	SessionTitle string `json:"sessionTitle"`
	ProfileID    string `json:"profileId"`
	Seq          int    `json:"seq"`
	Question     string `json:"question"`
	Field        string `json:"field"` // where the snippet comes from: title | question | answer | extra
	Snippet      string `json:"snippet"`
	Bookmarked   bool   `json:"bookmarked"`
	StartedAt    int64  `json:"startedAt"`
}

type SearchResult struct {
	Hits  []SearchHit `json:"hits"`
	Terms []string    `json:"terms"` // the words searched for, for highlighting in the page
}

// Search finds pages matching the query, best first (with a small boost
// for recent pages). An empty query finds nothing.
func (s *Store) Search(q SearchQuery) (SearchResult, error) {
	match, terms := FTSQuery(q.Text)
	res := SearchResult{Hits: []SearchHit{}, Terms: terms}
	if match == "" {
		return res, nil
	}
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	where := []string{"search MATCH ?"}
	args := []any{match}
	if len(q.Profiles) > 0 {
		where = append(where, "s.profile_id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(q.Profiles)), ",")+")")
		for _, p := range q.Profiles {
			args = append(args, p)
		}
	}
	if q.SessionID != "" {
		where, args = append(where, "r.session_id = ?"), append(args, q.SessionID)
	}
	if q.Bookmarked {
		where = append(where, "p.bookmarked = 1")
	}
	if q.Since > 0 {
		where, args = append(where, "p.started_at >= ?"), append(args, q.Since)
	}
	if q.Until > 0 {
		where, args = append(where, "p.started_at < ?"), append(args, q.Until)
	}
	// Recency: up to one bm25 point for pages from the last year.
	nowMS := time.Now().UnixMilli()
	// Rank every match cheaply, then build snippets for the top few only:
	// snippet() is the costly part, and with ORDER BY … LIMIT SQLite would
	// otherwise compute it for every match before sorting.
	// Placeholders in SQL order: the recency term, the filters, the limit,
	// then the outer MATCH.
	args = append(append([]any{nowMS}, args...), limit, match)
	// Without filters, FTS5 picks the best candidates on text alone first
	// (fast), and recency only reorders those.
	from := `search`
	if len(where) == 1 {
		from = `(SELECT rowid, ` + searchRank + ` AS bm FROM search WHERE search MATCH ? ORDER BY bm LIMIT 500) AS search`
	}
	rank := searchRank
	if len(where) == 1 {
		rank = `search.bm`
		where = []string{"1"}
	}
	rows, err := s.db.Query(`WITH top AS (
			SELECT search.rowid AS rid, `+rank+` + MIN(MAX((? - COALESCE(p.started_at, 0)) / 86400000.0, 0), 365) / 365.0 AS score
			FROM `+from+`
			JOIN search_rows r ON r.id = search.rowid
			JOIN pages p ON p.id = r.page_id
			LEFT JOIN sessions s ON s.id = r.session_id
			WHERE `+strings.Join(where, " AND ")+`
			ORDER BY score LIMIT ?)
		SELECT r.page_id, r.session_id, COALESCE(s.title, ''), COALESCE(s.profile_id, ''), p.seq, p.question,
			COALESCE(p.bookmarked, 0), COALESCE(p.started_at, 0),
			snippet(search, 0, '`+markOpen+`', '`+markClose+`', '…', 10),
			snippet(search, 1, '`+markOpen+`', '`+markClose+`', '…', 16),
			snippet(search, 2, '`+markOpen+`', '`+markClose+`', '…', 16),
			snippet(search, 3, '`+markOpen+`', '`+markClose+`', '…', 10)
		FROM top
		JOIN search ON search.rowid = top.rid
		JOIN search_rows r ON r.id = top.rid
		JOIN pages p ON p.id = r.page_id
		LEFT JOIN sessions s ON s.id = r.session_id
		WHERE search MATCH ?
		ORDER BY top.score`, args...)
	if err != nil {
		return res, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var h SearchHit
		var title, question, answer, extra string
		if err := rows.Scan(&h.PageID, &h.SessionID, &h.SessionTitle, &h.ProfileID, &h.Seq, &h.Question,
			&h.Bookmarked, &h.StartedAt, &title, &question, &answer, &extra); err != nil {
			return res, err
		}
		// Show where the words were found: the answer first (the usual
		// reason to search), then the question, attachments and summary,
		// then the session title.
		for _, c := range []struct{ field, text string }{{"answer", answer}, {"question", question}, {"extra", extra}, {"title", title}} {
			if strings.Contains(c.text, markOpen) {
				h.Field, h.Snippet = c.field, c.text
				break
			}
		}
		res.Hits = append(res.Hits, h)
	}
	return res, rows.Err()
}

// FTSQuery turns what the user typed into a safe FTS5 query: every word
// must match; "quoted text" is a phrase; a word ending in * is a prefix,
// and so is the last word (results update as the user types). Nothing the
// user types reaches FTS5's syntax unquoted. It also returns the plain
// words, for highlighting.
func FTSQuery(text string) (string, []string) {
	var parts, terms []string
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	rest := text
	for {
		i := strings.IndexByte(rest, '"')
		if i < 0 {
			break
		}
		j := strings.IndexByte(rest[i+1:], '"')
		if j < 0 {
			rest = rest[:i] + " " + rest[i+1:] // an unclosed quote is just a character
			break
		}
		phrase := strings.Join(words(rest[i+1:i+1+j]), " ")
		if phrase != "" {
			parts = append(parts, quote(phrase))
			terms = append(terms, phrase)
		}
		rest = rest[:i] + " " + rest[i+2+j:]
	}
	ws := words(rest)
	trailingSpace := len(text) > 0 && unicode.IsSpace(rune(text[len(text)-1]))
	for k, w := range ws {
		prefix := strings.HasSuffix(w, "*") || (k == len(ws)-1 && !trailingSpace && !strings.HasSuffix(strings.TrimSpace(text), `"`))
		w = strings.TrimRight(w, "*")
		if w == "" {
			continue
		}
		p := quote(w)
		if prefix {
			p += "*"
		}
		parts = append(parts, p)
		terms = append(terms, w)
	}
	return strings.Join(parts, " "), terms
}

// words splits text into searchable words, keeping a trailing * as a
// prefix marker and dropping everything that isn't a letter or digit.
func words(s string) []string {
	var out []string
	var b strings.Builder
	flush := func(star bool) {
		if b.Len() > 0 {
			w := b.String()
			if star {
				w += "*"
			}
			out = append(out, w)
			b.Reset()
		}
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r):
			b.WriteRune(r)
		case r == '*':
			flush(true)
		default:
			flush(false)
		}
	}
	flush(false)
	return out
}
