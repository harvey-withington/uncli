package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

// The safe list: the user's corrections to what UNCLI counts as safe. An
// entry names a command class (its words, such as "npm run test", or a git
// class such as "git:local", plus the risk flags that set it apart, such
// as "--force") or a tool, and says it is safe, unsafe or blocked, in one
// project or in all of them.

// Verdicts.
const (
	Safe    = "safe"
	Unsafe  = "unsafe"
	Blocked = "blocked"
)

// Entry kinds.
const (
	KindCommand = "command"
	KindTool    = "tool"
)

// SafeEntry is one entry on the safe list.
type SafeEntry struct {
	Kind    string `json:"kind"`            // command | tool
	Words   string `json:"words"`           // the class's words, or the tool's name
	Flags   string `json:"flags,omitempty"` // risk flags, sorted, space separated
	Verdict string `json:"verdict"`         // safe | unsafe | blocked
	// Folder is the project the entry belongs to; empty means all projects.
	Folder string `json:"folder,omitempty"`
}

// SafeClass is what an entry covers, without its verdict and scope.
type SafeClass struct {
	Kind  string `json:"kind"`
	Words string `json:"words"`
	Flags string `json:"flags,omitempty"`
}

// Class is the entry's class.
func (e SafeEntry) Class() SafeClass { return SafeClass{Kind: e.Kind, Words: e.Words, Flags: e.Flags} }

// projectKey normalises a working folder, so entries match however the
// path was spelled (case on Windows, trailing separators).
func projectKey(workdir string) string {
	k := filepath.Clean(workdir)
	if filepath.Separator == '\\' {
		k = strings.ToLower(k)
	}
	return k
}

func normWords(s string) string { return strings.Join(strings.Fields(s), " ") }

// NormFlags sorts and de-duplicates a list of flags.
func NormFlags(s string) string {
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Fields(s) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func scopeOf(folder string) string {
	if folder == "" {
		return ""
	}
	return projectKey(folder)
}

// SafeList lists the entries that apply in a project: its own, then those
// for all projects.
func (s *Store) SafeList(workdir string) ([]SafeEntry, error) {
	rows, err := s.db.Query(`SELECT folder, kind, words, flags, verdict FROM safe_list WHERE scope=? OR scope=''
		ORDER BY scope='' , kind, words, flags`, projectKey(workdir))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SafeEntry{}
	for rows.Next() {
		var e SafeEntry
		if err := rows.Scan(&e.Folder, &e.Kind, &e.Words, &e.Flags, &e.Verdict); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetSafeEntry adds an entry, or changes the verdict of the one for the
// same class and scope.
func (s *Store) SetSafeEntry(e SafeEntry) error {
	e.Words, e.Flags = normWords(e.Words), NormFlags(e.Flags)
	switch {
	case e.Kind != KindCommand && e.Kind != KindTool:
		return errors.New("an entry is for a command or a tool")
	case e.Words == "":
		return errors.New("an entry needs a command or tool")
	case e.Verdict != Safe && e.Verdict != Unsafe && e.Verdict != Blocked:
		return errors.New("an entry is safe, unsafe or blocked")
	}
	_, err := s.db.Exec(`INSERT INTO safe_list (scope, folder, kind, words, flags, verdict, created_at) VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(scope, kind, words, flags) DO UPDATE SET verdict=excluded.verdict, folder=excluded.folder`,
		scopeOf(e.Folder), e.Folder, e.Kind, e.Words, e.Flags, e.Verdict, now())
	return err
}

// DeleteSafeEntry removes an entry.
func (s *Store) DeleteSafeEntry(e SafeEntry) error {
	_, err := s.db.Exec(`DELETE FROM safe_list WHERE scope=? AND kind=? AND words=? AND flags=?`,
		scopeOf(e.Folder), e.Kind, normWords(e.Words), NormFlags(e.Flags))
	return err
}

// MoveSafeEntry gives an entry another scope (a project's folder, or ""
// for all projects). An entry already there for the same class takes the
// moved entry's verdict.
func (s *Store) MoveSafeEntry(e SafeEntry, folder string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM safe_list WHERE scope=? AND kind=? AND words=? AND flags=?`,
		scopeOf(e.Folder), e.Kind, normWords(e.Words), NormFlags(e.Flags)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO safe_list (scope, folder, kind, words, flags, verdict, created_at) VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(scope, kind, words, flags) DO UPDATE SET verdict=excluded.verdict`,
		scopeOf(folder), folder, e.Kind, normWords(e.Words), NormFlags(e.Flags), e.Verdict, now()); err != nil {
		return err
	}
	return tx.Commit()
}

// migrateSafeList moves the project rules and tool classes from before the
// safe list into it: allow became safe, ask unsafe, deny blocked; a tool
// class that changed things became unsafe, one that only looked safe.
func migrateSafeList(db *sql.DB) error {
	exists := func(table string) bool {
		var n string
		return db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n) == nil
	}
	verdicts := map[string]string{"allow": Safe, "ask": Unsafe, "deny": Blocked}
	shell := map[string]bool{"Bash": true, "PowerShell": true}
	if exists("tool_rules") {
		rows, err := db.Query(`SELECT workdir, tool, prefix, action FROM tool_rules`)
		if err != nil {
			return err
		}
		type rule struct{ workdir, tool, prefix, action string }
		var rules []rule
		for rows.Next() {
			var r rule
			if err := rows.Scan(&r.workdir, &r.tool, &r.prefix, &r.action); err == nil {
				rules = append(rules, r)
			}
		}
		rows.Close()
		for _, r := range rules {
			kind, words := KindTool, r.tool
			if shell[r.tool] {
				if r.prefix == "" {
					continue // "any command" can't be a class
				}
				kind, words = KindCommand, r.prefix
			}
			if _, err := db.Exec(`INSERT OR IGNORE INTO safe_list (scope, folder, kind, words, flags, verdict, created_at)
				VALUES (?,?,?,?,'',?,?)`, r.workdir, r.workdir, kind, normWords(words), verdicts[r.action], now()); err != nil {
				return err
			}
		}
		if _, err := db.Exec(`DROP TABLE tool_rules`); err != nil {
			return err
		}
	}
	if exists("tool_classes") {
		if _, err := db.Exec(`INSERT OR IGNORE INTO safe_list (scope, folder, kind, words, flags, verdict, created_at)
			SELECT '', '', 'tool', tool, '', CASE WHEN write THEN 'unsafe' ELSE 'safe' END, updated_at FROM tool_classes`); err != nil {
			return err
		}
		if _, err := db.Exec(`DROP TABLE tool_classes`); err != nil {
			return err
		}
	}
	return nil
}

// Judgement is what the decision model said about a command (or tool)
// UNCLI didn't recognise: looks, routine or risky, how it could do harm,
// a one-line reason for the user when it gave one, and which decider said
// it and how sure it was.
type Judgement struct {
	Level      string  `json:"level"`
	Risk       string  `json:"risk,omitempty"`
	Note       string  `json:"note,omitempty"`
	Model      string  `json:"model,omitempty"`
	Decider    string  `json:"decider,omitempty"`    // core.DeciderInfo.Key; empty before deciders
	Confidence float64 `json:"confidence,omitempty"` // of the level; 1 from an uncalibrated decider
}

// Judgement looks up an earlier judgement.
func (s *Store) Judgement(key string) (Judgement, bool) {
	var j Judgement
	var risk, note, model, decider sql.NullString
	var confidence sql.NullFloat64
	err := s.db.QueryRow(`SELECT level, risk, note, model, decider, confidence FROM risk_judgements WHERE key=?`, key).
		Scan(&j.Level, &risk, &note, &model, &decider, &confidence)
	if err != nil {
		return Judgement{}, false
	}
	j.Risk, j.Note, j.Model, j.Decider, j.Confidence = risk.String, note.String, model.String, decider.String, confidence.Float64
	return j, true
}

// SetJudgement records a judgement.
func (s *Store) SetJudgement(key string, j Judgement) error {
	_, err := s.db.Exec(`INSERT INTO risk_judgements (key, level, risk, note, model, decider, confidence, created_at) VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET level=excluded.level, risk=excluded.risk, note=excluded.note, model=excluded.model,
		decider=excluded.decider, confidence=excluded.confidence, created_at=excluded.created_at`,
		key, j.Level, nullStr(j.Risk), nullStr(j.Note), nullStr(j.Model), nullStr(j.Decider), j.Confidence, now())
	return err
}
