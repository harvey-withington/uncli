package store

import (
	"errors"
	"path/filepath"
	"strings"
)

// ToolRule answers a tool permission request in a project without asking
// (allow, deny) or makes sure it is asked (ask). Prefix narrows a Bash rule
// to commands starting with those words ("git", "git push"); empty means
// the whole tool.
type ToolRule struct {
	Tool   string `json:"tool"`
	Prefix string `json:"prefix,omitempty"`
	Action string `json:"action"` // allow | ask | deny
}

// Rule actions.
const (
	RuleAllow = "allow"
	RuleAsk   = "ask"
	RuleDeny  = "deny"
)

// projectKey normalises a working folder, so rules match however the path
// was spelled (case on Windows, trailing separators).
func projectKey(workdir string) string {
	k := filepath.Clean(workdir)
	if filepath.Separator == '\\' {
		k = strings.ToLower(k)
	}
	return k
}

// Rules lists a project's tool rules.
func (s *Store) Rules(workdir string) ([]ToolRule, error) {
	rows, err := s.db.Query(`SELECT tool, prefix, action FROM tool_rules WHERE workdir=? ORDER BY tool, prefix`, projectKey(workdir))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ToolRule{}
	for rows.Next() {
		var r ToolRule
		if err := rows.Scan(&r.Tool, &r.Prefix, &r.Action); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetRule adds a rule, or changes the action of the one for the same tool
// and prefix.
func (s *Store) SetRule(workdir string, r ToolRule) error {
	r.Tool, r.Prefix = strings.TrimSpace(r.Tool), strings.Join(strings.Fields(r.Prefix), " ")
	if r.Tool == "" {
		return errors.New("a rule needs a tool")
	}
	if r.Action != RuleAllow && r.Action != RuleAsk && r.Action != RuleDeny {
		return errors.New("a rule's action is allow, ask or deny")
	}
	_, err := s.db.Exec(`INSERT INTO tool_rules (workdir, tool, prefix, action, created_at) VALUES (?,?,?,?,?)
		ON CONFLICT(workdir, tool, prefix) DO UPDATE SET action=excluded.action`, projectKey(workdir), r.Tool, r.Prefix, r.Action, now())
	return err
}

// DeleteRule removes the rule for a tool and prefix.
func (s *Store) DeleteRule(workdir, tool, prefix string) error {
	_, err := s.db.Exec(`DELETE FROM tool_rules WHERE workdir=? AND tool=? AND prefix=?`, projectKey(workdir), tool, strings.Join(strings.Fields(prefix), " "))
	return err
}
