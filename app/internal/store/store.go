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
  mode          TEXT,              -- when to prompt: always | unsafe | never (NULL = unsafe)
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

-- The user's safe list: corrections to what UNCLI counts as safe. Each
-- entry is a command class (words and risk flags) or a tool, with a
-- verdict (safe, unsafe, blocked), for one project ('' = all projects).
CREATE TABLE IF NOT EXISTS safe_list (
  scope      TEXT NOT NULL,            -- '' or the project key (projectKey)
  folder     TEXT NOT NULL DEFAULT '', -- the project folder as the user knows it
  kind       TEXT NOT NULL,            -- command | tool
  words      TEXT NOT NULL,            -- "npm run test", a git class "git:local", or a tool name
  flags      TEXT NOT NULL DEFAULT '', -- risk flags, sorted, space separated
  verdict    TEXT NOT NULL,            -- safe | unsafe | blocked
  label      TEXT NOT NULL DEFAULT '', -- the user's own name for it
  created_at INTEGER,
  PRIMARY KEY (scope, kind, words, flags)
);

-- What the quick-task model said about commands UNCLI didn't recognise,
-- so each (command class, where one can be told) is judged once.
CREATE TABLE IF NOT EXISTS risk_judgements (
  key        TEXT PRIMARY KEY,
  level      TEXT NOT NULL,
  risk       TEXT,
  note       TEXT,
  model      TEXT,
  decider    TEXT,               -- which decision model said it (core.DeciderInfo.Key)
  confidence REAL,               -- how sure, of the level
  created_at INTEGER
);
`

// migrate adds columns introduced after the first schema.
func migrate(db *sql.DB) error {
	has := func(table, col string) bool {
		rows, err := db.Query("SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var n string
			if rows.Scan(&n) == nil && n == col {
				return true
			}
		}
		return false
	}
	for _, c := range []struct{ table, col, def string }{
		{"pages", "outline", "TEXT"}, {"pages", "attachments", "TEXT"},
		{"sessions", "mode", "TEXT"},
		{"risk_judgements", "decider", "TEXT"}, {"risk_judgements", "confidence", "REAL"},
		{"safe_list", "label", "TEXT NOT NULL DEFAULT ''"},
	} {
		if !has(c.table, c.col) {
			if _, err := db.Exec("ALTER TABLE " + c.table + " ADD COLUMN " + c.col + " " + c.def); err != nil {
				return err
			}
		}
	}
	// Session modes before "when to prompt": Read-only, Ask and Full.
	for old, level := range map[string]string{"readonly": "always", "ask": "unsafe", "full": "never"} {
		if _, err := db.Exec("UPDATE sessions SET mode=? WHERE mode=?", level, old); err != nil {
			return err
		}
	}
	return migrateSafeList(db)
}

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
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := setupSearch(db); err != nil {
		db.Close()
		return nil, err
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
	// Mode is when Claude stops to prompt the user: always (anything but
	// reading), unsafe (only what's unsafe; empty means this) or never.
	Mode      string `json:"mode"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
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
		model, modifiers, sort_order, archived, mode, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		x.ID, x.Title, x.Adapter, x.Runtime, x.ProfileID, x.Workdir, nullStr(x.ProviderSID), nullStr(x.CLIVersion),
		x.Model, jsonList(x.Modifiers), x.SortOrder, x.Archived, nullStr(x.Mode), x.CreatedAt, x.UpdatedAt)
	return err
}

func (s *Store) UpdateSession(x *Session) error {
	x.UpdatedAt = now()
	var old sql.NullString
	_ = s.db.QueryRow(`SELECT title FROM sessions WHERE id=?`, x.ID).Scan(&old)
	_, err := s.db.Exec(`UPDATE sessions SET title=?, workdir=?, provider_sid=?, cli_version=?, model=?, modifiers=?,
		sort_order=?, archived=?, mode=?, updated_at=? WHERE id=?`,
		x.Title, x.Workdir, nullStr(x.ProviderSID), nullStr(x.CLIVersion), x.Model, jsonList(x.Modifiers),
		x.SortOrder, x.Archived, nullStr(x.Mode), x.UpdatedAt, x.ID)
	if err == nil && old.String != x.Title {
		err = retitleSession(s.db, x.ID, x.Title)
	}
	return err
}

const sessionCols = `id, title, adapter, runtime, profile_id, workdir, provider_sid, cli_version, model, modifiers,
	sort_order, archived, mode, created_at, updated_at`

func scanSession(r interface{ Scan(...any) error }) (Session, error) {
	var x Session
	var title, sid, ver, mods, mode sql.NullString
	var order sql.NullFloat64
	var archived sql.NullBool
	var created, updated sql.NullInt64
	err := r.Scan(&x.ID, &title, &x.Adapter, &x.Runtime, &x.ProfileID, &x.Workdir, &sid, &ver, &x.Model, &mods,
		&order, &archived, &mode, &created, &updated)
	x.Title, x.ProviderSID, x.CLIVersion, x.Mode = title.String, sid.String, ver.String, mode.String
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
	if err := unindexSession(tx, id); err != nil {
		return err
	}
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
	// Approved says who let a tool use run when the CLI asked first: "you"
	// (on its card), "rule" (a project rule), "session" (a session rule),
	// "profile" (the session type's allowlist), "looks" (it only looks),
	// "full" (Full mode) or "mixed" (a command whose parts were allowed for
	// different reasons). Empty when it didn't ask.
	Approved string `json:"approved,omitempty"`
	// Why gives the reason for each part of a command (or the tool) when
	// UNCLI answered without the user, allowed or denied.
	Why []TraceReason `json:"why,omitempty"`
}

// TraceReason is why one part of a command (or a whole tool use) was
// allowed, asked about or refused.
type TraceReason struct {
	Part string `json:"part,omitempty"` // the part of the command; empty for a whole tool use
	// By says what decided it. It ran: looks (reading), safe (UNCLI or the
	// quick-task model judged it safe), listed (the user's safe list),
	// builtin (the session type's list), never (Run without prompting).
	// It prompted: always (Always prompt me), unsafe, unknown (UNCLI can't
	// tell), judging (the model is checking it), user (a question for the
	// user). blocked: the user's safe list blocks it. (Older traces also
	// hold rule, session, profile, routine, full, deny, readonly, ask…)
	By    string     `json:"by"`
	Entry *SafeEntry `json:"entry,omitempty"` // the safe-list entry that decided it
	Allow string     `json:"allow,omitempty"` // the session type's entry that matched (builtin), as written
	// Class is what "This is safe" or "This should prompt" would remember;
	// Fixed says why nothing can be (inline code, too complex, or unsafe
	// because of where it works rather than what it is).
	Class *SafeClass `json:"class,omitempty"`
	Fixed string     `json:"fixed,omitempty"`
	// Risk says how an unsafe part could do harm (deletes, outside, publishes…).
	Risk string `json:"risk,omitempty"`
	// Judged names the model that judged a part UNCLI didn't recognise, and
	// Note is its one-line reason.
	Judged string `json:"judged,omitempty"`
	Note   string `json:"note,omitempty"`
}

// TouchedFile is a file Claude changed during a page's turn.
type TouchedFile struct {
	Path string `json:"path"`           // absolute
	Line int    `json:"line,omitempty"` // the latest edit's first changed line
	How  string `json:"how"`            // write (created or replaced) | edit | command (a command changed it) | deleted (a command deleted it)
	// Lines added and removed, summed over the turn's tool writes; zero
	// for both when no tool said (a command's change).
	Added   int `json:"added,omitempty"`
	Removed int `json:"removed,omitempty"`
}

// ArtifactVersion is an artifact as a page's turn left it: its content
// by hash in the artifact store, or deleted.
type ArtifactVersion struct {
	Path    string `json:"path"` // relative to the artifacts folder, with slashes
	Hash    string `json:"hash,omitempty"`
	Size    int64  `json:"size"`
	Deleted bool   `json:"deleted,omitempty"`
}

type Page struct {
	ID           string            `json:"id"`
	SessionID    string            `json:"sessionId"`
	Seq          int               `json:"seq"`
	Question     string            `json:"question"`
	Directives   string            `json:"directives,omitempty"`
	Model        string            `json:"model"`
	Modifiers    []string          `json:"modifiers"`
	AnswerMD     string            `json:"answerMd"`
	Trace        []TraceItem       `json:"trace"`
	TouchedFiles []TouchedFile     `json:"touchedFiles"`
	Artifacts    []ArtifactVersion `json:"artifacts"` // artifacts the turn added, changed or deleted
	Status       string            `json:"status"`    // open | done | error | interrupted
	Error        string            `json:"error,omitempty"`
	Bookmarked   bool              `json:"bookmarked"`
	Pinned       bool              `json:"pinned"`
	InputTokens  int               `json:"inputTokens"`
	OutputTokens int               `json:"outputTokens"`
	CacheRead    int               `json:"cacheRead"`
	CacheWrite   int               `json:"cacheWrite"`
	CostUSD      float64           `json:"costUsd"`
	DurationMS   int               `json:"durationMs"`
	StartedAt    int64             `json:"startedAt"`
	FinishedAt   int64             `json:"finishedAt"`
	Outline      *PageOutline      `json:"outline,omitempty"` // a summary table of contents, if one was made
	Attachments  []PageAttachment  `json:"attachments"`       // files sent with the question (their content is in the CLI's transcript)
}

// PageAttachment records a file sent with a page's question.
type PageAttachment struct {
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"` // empty for pasted data
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

// PageOutline is a table of contents written by the quick-task model,
// anchored to the answer's top-level markdown blocks.
type PageOutline struct {
	Provider string           `json:"provider"`
	Model    string           `json:"model"`
	Sections []OutlineSection `json:"sections"`
	CostUSD  float64          `json:"costUsd"`
	At       int64            `json:"at"`
}

type OutlineSection struct {
	Block int    `json:"block"`
	Title string `json:"title"`
	Kind  string `json:"kind,omitempty"` // one of the section kinds; empty = let the UI guess
}

// SetPageOutline stores (or with nil, clears) a page's summary outline.
// SavePage never touches it, so a live page can't overwrite it.
func (s *Store) SetPageOutline(pageID string, o *PageOutline) error {
	var v any
	if o != nil {
		b, err := json.Marshal(o)
		if err != nil {
			return err
		}
		v = string(b)
	}
	res, err := s.db.Exec(`UPDATE pages SET outline=? WHERE id=?`, v, pageID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("page not found")
	}
	return indexPage(s.db, pageID) // summary titles are searchable
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
	attached, _ := json.Marshal(nonNil(p.Attachments))
	_, err := s.db.Exec(`INSERT INTO pages (id, session_id, seq, question, directives, model, modifiers, answer_md, trace,
		touched_files, artifacts, status, bookmarked, pinned, input_tokens, output_tokens, cache_read, cache_write,
		cost_usd, duration_ms, started_at, finished_at, attachments) VALUES (?,?,?,?,?,?,?,?,?,?,'[]',?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET answer_md=excluded.answer_md, trace=excluded.trace, touched_files=excluded.touched_files,
		status=excluded.status, bookmarked=excluded.bookmarked, pinned=excluded.pinned, model=excluded.model,
		input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens, cache_read=excluded.cache_read,
		cache_write=excluded.cache_write, cost_usd=excluded.cost_usd, duration_ms=excluded.duration_ms,
		finished_at=excluded.finished_at`,
		p.ID, p.SessionID, p.Seq, p.Question, p.Directives, p.Model, jsonList(p.Modifiers), p.AnswerMD, string(trace),
		string(touched), p.Status, p.Bookmarked, p.Pinned, p.InputTokens, p.OutputTokens, p.CacheRead, p.CacheWrite,
		p.CostUSD, p.DurationMS, p.StartedAt, p.FinishedAt, string(attached))
	if err != nil {
		return err
	}
	return indexPage(s.db, p.ID)
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// SetPageFiles stores what a page's turn changed, found after it closed:
// its touched files and artifact versions. Only those columns change, so
// a bookmark set meanwhile is kept.
func (s *Store) SetPageFiles(pageID string, touched []TouchedFile, artifacts []ArtifactVersion) error {
	t, _ := json.Marshal(nonNil(touched))
	a, _ := json.Marshal(nonNil(artifacts))
	res, err := s.db.Exec(`UPDATE pages SET touched_files=?, artifacts=? WHERE id=?`, string(t), string(a), pageID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("page not found")
	}
	return nil
}

const pageColumns = `id, session_id, seq, question, directives, model, modifiers, answer_md, trace,
		touched_files, status, bookmarked, pinned, input_tokens, output_tokens, cache_read, cache_write, cost_usd,
		duration_ms, started_at, finished_at, outline, attachments, artifacts`

// Page reads one page.
func (s *Store) Page(pageID string) (Page, error) {
	rows, err := s.db.Query(`SELECT `+pageColumns+` FROM pages WHERE id=?`, pageID)
	if err != nil {
		return Page{}, err
	}
	pages, err := scanPages(rows)
	if err != nil {
		return Page{}, err
	}
	if len(pages) == 0 {
		return Page{}, errors.New("page not found")
	}
	return pages[0], nil
}

func (s *Store) ListPages(sessionID string) ([]Page, error) {
	rows, err := s.db.Query(`SELECT `+pageColumns+` FROM pages WHERE session_id=? ORDER BY seq`, sessionID)
	if err != nil {
		return nil, err
	}
	return scanPages(rows)
}

func scanPages(rows *sql.Rows) ([]Page, error) {
	defer rows.Close()
	out := []Page{}
	for rows.Next() {
		var p Page
		var directives, mods, answer, trace, touched, status sql.NullString
		var bm, pin sql.NullBool
		var in, outT, cr, cw, dur, st, fin sql.NullInt64
		var cost sql.NullFloat64
		var outline, attached, artifacts sql.NullString
		if err := rows.Scan(&p.ID, &p.SessionID, &p.Seq, &p.Question, &directives, &p.Model, &mods, &answer, &trace,
			&touched, &status, &bm, &pin, &in, &outT, &cr, &cw, &cost, &dur, &st, &fin, &outline, &attached, &artifacts); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(artifacts.String), &p.Artifacts)
		p.Artifacts = nonNil(p.Artifacts)
		_ = json.Unmarshal([]byte(attached.String), &p.Attachments)
		p.Attachments = nonNil(p.Attachments)
		if outline.Valid && outline.String != "" {
			var o PageOutline
			if json.Unmarshal([]byte(outline.String), &o) == nil {
				p.Outline = &o
			}
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
