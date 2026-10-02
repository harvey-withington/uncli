package session

import (
	"encoding/json"
	"strings"
	"testing"

	"uncli/internal/store"
)

func bash(cmd string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"command": cmd})
	return b
}

func TestGitClasses(t *testing.T) {
	cases := map[string]string{
		"git status":                        GitRead,
		"git -C ../repo log --oneline":      GitRead,
		"git --no-pager diff HEAD~1":        GitRead,
		"git branch":                        GitRead,
		"git branch -a":                     GitRead,
		"git branch feature/x":              GitLocal,
		"git branch -d old":                 GitLocal,
		"git branch -D old":                 GitDestructive,
		"git remote -v":                     GitRead,
		"git remote set-url origin git@x:y": GitLocal,
		"git remote add upstream https://x": GitLocal,
		"git remote remove upstream":        GitDestructive,
		"git config user.name":              GitRead,
		"git config user.name Harv":         GitLocal,
		"git config --get core.autocrlf":    GitRead,
		"git add -A":                        GitLocal,
		"git fetch origin":                  GitLocal,
		"git switch main":                   GitLocal,
		"git switch --discard-changes main": GitDestructive,
		"git checkout -b feature":           GitLocal,
		"git checkout -- file.go":           GitDestructive,
		"git checkout .":                    GitDestructive,
		"git checkout main":                 "", // branch or file: not classed
		"git restore --staged a.go":         GitLocal,
		"git restore a.go":                  GitDestructive,
		"git stash":                         GitLocal,
		"git stash list":                    GitRead,
		"git stash drop":                    GitDestructive,
		"git commit -m wip":                 GitCommit,
		"git merge feature":                 GitCommit,
		"git merge --abort":                 GitLocal,
		"git tag v1.0":                      GitCommit,
		"git tag":                           GitRead,
		"git pull":                          GitCommit,
		"git push":                          GitPublish,
		"git push origin main":              GitPublish,
		"git push --force":                  GitDestructive,
		"git push --force-with-lease":       GitDestructive,
		"git reset HEAD a.go":               GitLocal,
		"git reset --hard HEAD~1":           GitDestructive,
		"git clean -fd":                     GitDestructive,
		"git rebase -i main":                GitDestructive,
		"git frobnicate":                    "",
	}
	for cmd, want := range cases {
		w, ok := commandWords(cmd)
		if !ok {
			t.Fatalf("%q isn't a plain command", cmd)
		}
		if got := gitClass(w[1:]); got != want {
			t.Errorf("%q = %q, want %q", cmd, got, want)
		}
	}
}

// Harvey's two usual setups, plus the rule precedence.
func TestRuleMatch(t *testing.T) {
	allowCommitNotPush := []store.ToolRule{
		{Tool: "Bash", Prefix: GitRead, Action: store.RuleAllow},
		{Tool: "Bash", Prefix: GitLocal, Action: store.RuleAllow},
		{Tool: "Bash", Prefix: GitCommit, Action: store.RuleAllow},
	}
	anythingButCommitAndPush := []store.ToolRule{
		{Tool: "Bash", Prefix: "git", Action: store.RuleAllow},
		{Tool: "Bash", Prefix: "git commit", Action: store.RuleAsk},
		{Tool: "Bash", Prefix: "git push", Action: store.RuleAsk},
	}
	cases := []struct {
		rules []store.ToolRule
		cmd   string
		want  string
	}{
		{allowCommitNotPush, "git status", store.RuleAllow},
		{allowCommitNotPush, "git remote set-url origin x", store.RuleAllow},
		{allowCommitNotPush, "git commit -m x", store.RuleAllow},
		{allowCommitNotPush, "git push", ""},
		{allowCommitNotPush, "git reset --hard", ""},
		{anythingButCommitAndPush, "git status", store.RuleAllow},
		{anythingButCommitAndPush, "git rebase main", store.RuleAllow}, // "anything" means anything
		{anythingButCommitAndPush, "git commit -m x", store.RuleAsk},
		{anythingButCommitAndPush, "git push origin main", store.RuleAsk},
		{anythingButCommitAndPush, "git -C sub push", store.RuleAsk}, // global options don't hide the subcommand
		// A specific prefix beats a class, which beats the program.
		{[]store.ToolRule{{Tool: "Bash", Prefix: GitPublish, Action: store.RuleAllow}, {Tool: "Bash", Prefix: "git push", Action: store.RuleDeny}}, "git push", store.RuleDeny},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleDeny}, {Tool: "Bash", Prefix: GitRead, Action: store.RuleAllow}}, "git log", store.RuleAllow},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleDeny}, {Tool: "Bash", Prefix: GitRead, Action: store.RuleAllow}}, "git push", store.RuleDeny},
		// Rules never cover compound commands.
		{[]store.ToolRule{{Tool: "Bash", Action: store.RuleAllow}}, "git status && rm -rf .", ""},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, "git log | head", ""},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, "git log > out.txt", ""},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, "git log $(rm x)", ""},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, "GIT_DIR=x git log", ""},
		// The program's folder and .exe don't matter; other programs don't match.
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, `C:\Program Files\Git\bin\git.exe status`, ""}, // a space in the path splits it: not plain
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, `/usr/bin/git status`, store.RuleAllow},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "git", Action: store.RuleAllow}}, "gitk", ""},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "npm test", Action: store.RuleAllow}}, "npm test -- --watch", store.RuleAllow},
		{[]store.ToolRule{{Tool: "Bash", Prefix: "npm test", Action: store.RuleAllow}}, "npm publish", ""},
	}
	for _, c := range cases {
		if got := ruleMatch(c.rules, "Bash", bash(c.cmd)); got != c.want {
			t.Errorf("%q = %q, want %q", c.cmd, got, c.want)
		}
	}
	// Other tools: a rule for the tool covers it; no rule asks.
	if ruleMatch([]store.ToolRule{{Tool: "Write", Action: store.RuleAllow}}, "Write", json.RawMessage(`{"file_path":"a"}`)) != store.RuleAllow {
		t.Error("a tool rule must cover the tool")
	}
	if ruleMatch([]store.ToolRule{{Tool: "Write", Action: store.RuleAllow}}, "Edit", nil) != "" {
		t.Error("a rule for one tool must not cover another")
	}
}

func TestSuggestions(t *testing.T) {
	show := func(rs []store.ToolRule) string {
		var out []string
		for _, r := range rs {
			out = append(out, r.Tool+":"+r.Prefix)
		}
		return strings.Join(out, ", ")
	}
	cases := map[string]string{
		"git push origin main":   "Bash:git push, Bash:git:publish, Bash:git",
		"git checkout main":      "Bash:git checkout, Bash:git",
		"npm run build":          "Bash:npm run, Bash:npm",
		"node -e console.log(1)": "Bash:node",
		"ls -la":                 "Bash:ls",
	}
	for cmd, want := range cases {
		if got := show(suggestions("Bash", bash(cmd))); got != want {
			t.Errorf("%q = %s, want %s", cmd, got, want)
		}
	}
	if got := suggestions("Bash", bash("git add . && git commit")); got != nil {
		t.Errorf("a compound command can only be allowed once, got %v", got)
	}
	if got := show(suggestions("mcp__bruv__create_card", nil)); got != "mcp__bruv__create_card:" {
		t.Errorf("tool suggestion = %s", got)
	}
}

// PowerShell (Windows) is a shell like Bash: rules are about the command,
// a rule for one covers the other, and a card never offers "every command".
func TestPowerShellIsAShell(t *testing.T) {
	rules := []store.ToolRule{{Tool: "Bash", Prefix: "git:commit", Action: store.RuleAllow}, {Tool: "PowerShell", Prefix: "npm test", Action: store.RuleAllow}}
	cases := []struct {
		tool, cmd, want string
	}{
		{"PowerShell", "git commit -m x", store.RuleAllow}, // a Bash rule covers PowerShell
		{"Bash", "npm test", store.RuleAllow},              // and the other way round
		{"PowerShell", "git push", ""},
		{"PowerShell", "npm test 2>&1 | Select-Object -Last 40", ""}, // the request from Harvey's code review: piped, so it asks
		{"PowerShell", "Test-Path node_modules; Get-Content ci.yml", ""},
	}
	for _, c := range cases {
		if got := ruleMatch(rules, c.tool, bash(c.cmd)); got != c.want {
			t.Errorf("%s %q = %q, want %q", c.tool, c.cmd, got, c.want)
		}
	}
	if s := suggestions("PowerShell", bash("npm test 2>&1 | Select-Object -Last 40")); s != nil {
		t.Errorf("a piped command can only be allowed once, got %v", s)
	}
	got := suggestions("PowerShell", bash("npm test"))
	if len(got) != 2 || got[0].Prefix != "npm test" || got[1].Prefix != "npm" {
		t.Errorf("suggestions = %+v", got)
	}
	for _, r := range got {
		if r.Prefix == "" {
			t.Error("a shell card must never offer every command")
		}
	}
}
