package session

import (
	"encoding/json"
	"testing"
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
		parts, _, ok := readScript(cmd, DialectBash, "")
		if !ok || len(parts) != 1 {
			t.Fatalf("%q isn't one command", cmd)
		}
		if got := gitClass(parts[0].words[1:]); got != want {
			t.Errorf("%q = %q, want %q", cmd, got, want)
		}
	}
}
