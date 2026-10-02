package store

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestToolRules(t *testing.T) {
	s := searchStore(t)
	dir := filepath.Join(t.TempDir(), "Repo")
	if err := s.SetRule(dir, ToolRule{Tool: "Bash", Prefix: "git  push", Action: RuleAsk}); err != nil {
		t.Fatal(err)
	}
	// Same tool and prefix: the action changes, no duplicate.
	s.SetRule(dir, ToolRule{Tool: "Bash", Prefix: "git push", Action: RuleDeny})
	s.SetRule(dir, ToolRule{Tool: "Write", Action: RuleAllow})
	got, _ := s.Rules(dir + string(filepath.Separator))
	if len(got) != 2 || got[0] != (ToolRule{Tool: "Bash", Prefix: "git push", Action: RuleDeny}) {
		t.Fatalf("rules = %+v", got)
	}
	if runtime.GOOS == "windows" {
		if r, _ := s.Rules(strings.ToUpper(dir)); len(r) != 2 {
			t.Error("Windows paths match whatever their case")
		}
	}
	if r, _ := s.Rules(t.TempDir()); len(r) != 0 {
		t.Error("rules belong to their project")
	}
	if err := s.SetRule(dir, ToolRule{Tool: "Bash", Action: "sometimes"}); err == nil {
		t.Error("unknown actions are refused")
	}
	s.DeleteRule(dir, "Bash", "git push")
	if r, _ := s.Rules(dir); len(r) != 1 || r[0].Tool != "Write" {
		t.Errorf("after delete = %+v", r)
	}
}
