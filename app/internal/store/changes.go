package store

import (
	"encoding/json"
	"fmt"
)

// A page's changes are kept apart from the page (decision 0029): a page
// goes to the UI whole each time a session loads, and its changes' text is
// needed only when the user walks through them.

const changesSchema = `
CREATE TABLE IF NOT EXISTS page_changes (
  session_id TEXT NOT NULL,
  page_id    TEXT NOT NULL,
  n          INTEGER NOT NULL,  -- in the order the tools made them
  path       TEXT NOT NULL,
  hunks      TEXT NOT NULL,     -- JSON []Hunk
  PRIMARY KEY (page_id, n)
);
`

// Hunk is one changed place in a file, as a unified diff's: its first line
// in the old and new file, and its lines, each starting with "+", "-" or
// " " (unchanged, around the change).
type Hunk struct {
	OldStart int      `json:"oldStart"`
	NewStart int      `json:"newStart"`
	Lines    []string `json:"lines"`
}

// FileChange is what one tool call changed in one file.
type FileChange struct {
	Path  string `json:"path"`
	Hunks []Hunk `json:"hunks"`
}

// AddPageChange records a tool's change to a file during a page's turn.
func (s *Store) AddPageChange(sessionID, pageID string, c FileChange) error {
	b, err := json.Marshal(c.Hunks)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO page_changes (session_id, page_id, n, path, hunks)
		VALUES (?, ?, (SELECT COALESCE(MAX(n), 0) + 1 FROM page_changes WHERE page_id=?), ?, ?)`,
		sessionID, pageID, pageID, c.Path, string(b))
	return err
}

// PageChanges are a session's page's recorded changes, in the order they
// were made.
func (s *Store) PageChanges(sessionID, pageID string) ([]FileChange, error) {
	rows, err := s.db.Query(`SELECT path, hunks FROM page_changes WHERE session_id=? AND page_id=? ORDER BY n`, sessionID, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FileChange{}
	for rows.Next() {
		var c FileChange
		var hunks string
		if err := rows.Scan(&c.Path, &hunks); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(hunks), &c.Hunks); err != nil {
			return nil, fmt.Errorf("page %s's changes to %s: %w", pageID, c.Path, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
