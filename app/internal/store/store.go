// Package store keeps UNCLI's app state in SQLite: sessions, pages,
// bookmarks, usage and the raw event log. The CLI's own transcript remains
// the source of truth for conversation content.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
  id            TEXT PRIMARY KEY,
  title         TEXT,
  adapter       TEXT NOT NULL,
  runtime       TEXT NOT NULL,
  runtime_ref   TEXT,
  profile_id    TEXT NOT NULL,
  workdir       TEXT NOT NULL,
  provider_sid  TEXT,
  cli_version   TEXT,
  model         TEXT NOT NULL,
  modifiers     TEXT NOT NULL DEFAULT '[]',
  sort_order    REAL,
  archived      INTEGER DEFAULT 0,
  created_at    INTEGER, updated_at INTEGER
);

CREATE TABLE IF NOT EXISTS pages (
  id            TEXT PRIMARY KEY,
  session_id    TEXT REFERENCES sessions(id),
  seq           INTEGER NOT NULL,
  question      TEXT NOT NULL,
  directives    TEXT,
  model         TEXT NOT NULL,
  modifiers     TEXT NOT NULL,
  answer_md     TEXT,
  trace         TEXT,
  touched_files TEXT,
  artifacts     TEXT,
  status        TEXT,
  bookmarked    INTEGER DEFAULT 0,
  pinned        INTEGER DEFAULT 0,
  input_tokens  INTEGER, output_tokens INTEGER,
  cache_read    INTEGER, cache_write INTEGER,
  cost_usd      REAL, duration_ms INTEGER,
  started_at    INTEGER, finished_at INTEGER,
  UNIQUE(session_id, seq)
);

CREATE TABLE IF NOT EXISTS events (
  session_id TEXT, page_seq INTEGER, n INTEGER,
  kind TEXT, at INTEGER, data TEXT, raw TEXT,
  PRIMARY KEY (session_id, page_seq, n)
);

CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT);
`

type Store struct {
	db *sql.DB
	// EventCap is the most raw events kept per session; oldest pages go first.
	EventCap int
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; keeps SQLite simple and safe
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{db: db, EventCap: 20000}, nil
}

func (s *Store) Close() error { return s.db.Close() }

type Session struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Adapter     string   `json:"adapter"`
	Runtime     string   `json:"runtime"`
	ProfileID   string   `json:"profileId"`
	Workdir     string   `json:"workdir"`
	ProviderSID string   `json:"providerSid"`
	CLIVersion  string   `json:"cliVersion"`
	Model       string   `json:"model"`
	Modifiers   []string `json:"modifiers"`
	SortOrder   float64  `json:"sortOrder"`
	Archived    bool     `json:"archived"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

func now() int64 { return time.Now().UnixMilli() }

func jsonList(v []string) string {
	if v == nil {
		v = []string{}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func parseList(s sql.NullString) []string {
	out := []string{}
	if s.Valid {
		_ = json.Unmarshal([]byte(s.String), &out)
	}
	return out
}

func (s *Store) CreateSession(x *Session) error {
	t := now()
	x.CreatedAt, x.UpdatedAt = t, t
	if x.SortOrder == 0 {
		var max sql.NullFloat64
		_ = s.db.QueryRow(`SELECT MAX(sort_order) FROM sessions`).Scan(&max)
		x.SortOrder = max.Float64 + 1
	}
	_, err := s.db.Exec(`INSERT INTO sessions (id, title, adapter, runtime, profile_id, workdir, provider_sid, cli_version,
		model, modifiers, sort_order, archived, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		x.ID, x.Title, x.Adapter, x.Runtime, x.ProfileID, x.Workdir, nullStr(x.ProviderSID), nullStr(x.CLIVersion),
		x.Model, jsonList(x.Modifiers), x.SortOrder, x.Archived, x.CreatedAt, x.UpdatedAt)
	return err
}

func (s *Store) UpdateSession(x *Session) error {
	x.UpdatedAt = now()
	_, err := s.db.Exec(`UPDATE sessions SET title=?, workdir=?, provider_sid=?, cli_version=?, model=?, modifiers=?,
		sort_order=?, archived=?, updated_at=? WHERE id=?`,
		x.Title, x.Workdir, nullStr(x.ProviderSID), nullStr(x.CLIVersion), x.Model, jsonList(x.Modifiers),
		x.SortOrder, x.Archived, x.UpdatedAt, x.ID)
	return err
}

const sessionCols = `id, title, adapter, runtime, profile_id, workdir, provider_sid, cli_version, model, modifiers,
	sort_order, archived, created_at, updated_at`

func scanSession(r interface{ Scan(...any) error }) (Session, error) {
	var x Session
	var title, sid, ver, mods sql.NullString
	var order sql.NullFloat64
	var archived sql.NullBool
	var created, updated sql.NullInt64
	err := r.Scan(&x.ID, &title, &x.Adapter, &x.Runtime, &x.ProfileID, &x.Workdir, &sid, &ver, &x.Model, &mods,
		&order, &archived, &created, &updated)
	x.Title, x.ProviderSID, x.CLIVersion = title.String, sid.String, ver.String
	x.Modifiers = parseList(mods)
	x.SortOrder, x.Archived, x.CreatedAt, x.UpdatedAt = order.Float64, archived.Bool, created.Int64, updated.Int64
	return x, err
}

func (s *Store) GetSession(id string) (Session, error) {
	return scanSession(s.db.QueryRow(`SELECT `+sessionCols+` FROM sessions WHERE id=?`, id))
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT ` + sessionCols + ` FROM sessions WHERE archived=0 ORDER BY sort_order DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		x, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// DeleteSession removes a session with its pages and events.
func (s *Store) DeleteSession(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`DELETE FROM events WHERE session_id=?`, `DELETE FROM pages WHERE session_id=?`, `DELETE FROM sessions WHERE id=?`} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TraceItem is one tool call on a page.
type TraceItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Done    bool   `json:"done"`
	OK      bool   `json:"ok"`
	Denied  bool   `json:"denied,omitempty"`
	Output  string `json:"output,omitempty"`
}

type TouchedFile struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	How  string `json:"how"`
}

type Page struct {
	ID           string        `json:"id"`
	SessionID    string        `json:"sessionId"`
	Seq          int           `json:"seq"`
	Question     string        `json:"question"`
	Directives   string        `json:"directives,omitempty"`
	Model        string        `json:"model"`
	Modifiers    []string      `json:"modifiers"`
	AnswerMD     string        `json:"answerMd"`
	Trace        []TraceItem   `json:"trace"`
	TouchedFiles []TouchedFile `json:"touchedFiles"`
	Status       string        `json:"status"` // open | done | error | interrupted
	Error        string        `json:"error,omitempty"`
	Bookmarked   bool          `json:"bookmarked"`
	Pinned       bool          `json:"pinned"`
	InputTokens  int           `json:"inputTokens"`
	OutputTokens int           `json:"outputTokens"`
	CacheRead    int           `json:"cacheRead"`
	CacheWrite   int           `json:"cacheWrite"`
	CostUSD      float64       `json:"costUsd"`
	DurationMS   int           `json:"durationMs"`
	StartedAt    int64         `json:"startedAt"`
	FinishedAt   int64         `json:"finishedAt"`
}

// The pages table has no error column; errors ride in the trace JSON so
// the schema stays as the brief describes it.
type traceDoc struct {
	Items []TraceItem `json:"items"`
	Error string      `json:"error,omitempty"`
}

func (s *Store) NextSeq(sessionID string) (int, error) {
	var max sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(seq) FROM pages WHERE session_id=?`, sessionID).Scan(&max)
	return int(max.Int64) + 1, err
}

func (s *Store) SavePage(p *Page) error {
	trace, _ := json.Marshal(traceDoc{Items: nonNil(p.Trace), Error: p.Error})
	touched, _ := json.Marshal(nonNil(p.TouchedFiles))
	_, err := s.db.Exec(`INSERT INTO pages (id, session_id, seq, question, directives, model, modifiers, answer_md, trace,
		touched_files, artifacts, status, bookmarked, pinned, input_tokens, output_tokens, cache_read, cache_write,
		cost_usd, duration_ms, started_at, finished_at) VALUES (?,?,?,?,?,?,?,?,?,?,'[]',?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET answer_md=excluded.answer_md, trace=excluded.trace, touched_files=excluded.touched_files,
		status=excluded.status, bookmarked=excluded.bookmarked, pinned=excluded.pinned, model=excluded.model,
		input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens, cache_read=excluded.cache_read,
		cache_write=excluded.cache_write, cost_usd=excluded.cost_usd, duration_ms=excluded.duration_ms,
		finished_at=excluded.finished_at`,
		p.ID, p.SessionID, p.Seq, p.Question, p.Directives, p.Model, jsonList(p.Modifiers), p.AnswerMD, string(trace),
		string(touched), p.Status, p.Bookmarked, p.Pinned, p.InputTokens, p.OutputTokens, p.CacheRead, p.CacheWrite,
		p.CostUSD, p.DurationMS, p.StartedAt, p.FinishedAt)
	return err
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

func (s *Store) ListPages(sessionID string) ([]Page, error) {
	rows, err := s.db.Query(`SELECT id, session_id, seq, question, directives, model, modifiers, answer_md, trace,
		touched_files, status, bookmarked, pinned, input_tokens, output_tokens, cache_read, cache_write, cost_usd,
		duration_ms, started_at, finished_at FROM pages WHERE session_id=? ORDER BY seq`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Page{}
	for rows.Next() {
		var p Page
		var directives, mods, answer, trace, touched, status sql.NullString
		var bm, pin sql.NullBool
		var in, outT, cr, cw, dur, st, fin sql.NullInt64
		var cost sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Seq, &p.Question, &directives, &p.Model, &mods, &answer, &trace,
			&touched, &status, &bm, &pin, &in, &outT, &cr, &cw, &cost, &dur, &st, &fin); err != nil {
			return nil, err
		}
		p.Directives, p.AnswerMD, p.Status = directives.String, answer.String, status.String
		p.Modifiers = parseList(mods)
		var td traceDoc
		_ = json.Unmarshal([]byte(trace.String), &td)
		p.Trace, p.Error = nonNil(td.Items), td.Error
		_ = json.Unmarshal([]byte(touched.String), &p.TouchedFiles)
		p.TouchedFiles = nonNil(p.TouchedFiles)
		p.Bookmarked, p.Pinned = bm.Bool, pin.Bool
		p.InputTokens, p.OutputTokens, p.CacheRead, p.CacheWrite = int(in.Int64), int(outT.Int64), int(cr.Int64), int(cw.Int64)
		p.CostUSD, p.DurationMS, p.StartedAt, p.FinishedAt = cost.Float64, int(dur.Int64), st.Int64, fin.Int64
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) SetBookmark(pageID string, on bool) error {
	res, err := s.db.Exec(`UPDATE pages SET bookmarked=? WHERE id=?`, on, pageID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("page not found")
	}
	return nil
}

// MarkOpenPages closes pages left open by a crash or quit mid-turn.
func (s *Store) MarkOpenPages(status string) error {
	_, err := s.db.Exec(`UPDATE pages SET status=?, finished_at=? WHERE status='open'`, status, now())
	return err
}

// AppendEvent adds one raw event to the log.
func (s *Store) AppendEvent(sessionID string, pageSeq, n int, kind string, at time.Time, data, raw []byte) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO events (session_id, page_seq, n, kind, at, data, raw) VALUES (?,?,?,?,?,?,?)`,
		sessionID, pageSeq, n, kind, at.UnixMilli(), string(data), string(raw))
	return err
}

// PruneEvents keeps the event log under EventCap rows per session by
// dropping the oldest pages' events first.
func (s *Store) PruneEvents(sessionID string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE session_id=?`, sessionID).Scan(&count); err != nil {
		return err
	}
	for count > s.EventCap {
		var oldest sql.NullInt64
		if err := s.db.QueryRow(`SELECT MIN(page_seq) FROM events WHERE session_id=?`, sessionID).Scan(&oldest); err != nil || !oldest.Valid {
			return err
		}
		var last sql.NullInt64
		_ = s.db.QueryRow(`SELECT MAX(page_seq) FROM events WHERE session_id=?`, sessionID).Scan(&last)
		if oldest.Int64 == last.Int64 { // never drop the current page
			return nil
		}
		res, err := s.db.Exec(`DELETE FROM events WHERE session_id=? AND page_seq=?`, sessionID, oldest.Int64)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		count -= int(n)
	}
	return nil
}

func (s *Store) CountEvents(sessionID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE session_id=?`, sessionID).Scan(&n)
	return n, err
}

func (s *Store) Setting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
