package session

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"uncli/internal/store"
)

// Tool rules: per project (the session's working folder), a rule allows,
// asks about or denies a tool, or for Bash a command. A Bash rule's prefix
// is either words the command starts with ("git", "git push") or a git
// class ("git:local"), so one rule can cover a kind of git command. The
// most specific matching rule wins: a prefix of two or more words, then a
// class, then a one-word prefix (the program), then the whole tool. No
// rule means ask.
//
// A Bash rule only ever covers a plain command: one program with its
// arguments, with no chaining, pipes, redirects or substitutions, so
// "git status && rm -rf ." never rides on a git rule. Such commands are
// always asked about.

// Shell tools run a command line: Bash, and PowerShell on Windows. Their
// rules are about the command, not the tool, so a rule written for one
// covers both, and none of them is ever allowed wholesale from a card.
var shellTools = map[string]bool{"Bash": true, "PowerShell": true}

// IsShell reports whether a tool runs command lines.
func IsShell(tool string) bool { return shellTools[tool] }

// Git classes, from harmless to consequential.
const (
	GitRead        = "git:read"        // looks, changes nothing
	GitLocal       = "git:local"       // small local changes that are easy to undo
	GitCommit      = "git:commit"      // records history
	GitPublish     = "git:publish"     // sends to a remote
	GitDestructive = "git:destructive" // can lose work
)

// GitClasses lists the classes for the UI, in order.
var GitClasses = []string{GitRead, GitLocal, GitCommit, GitPublish, GitDestructive}

// commandWords splits a plain command into words, or false when the
// command does more than run one program. The program loses its folder
// and .exe; for git, global options before the subcommand are dropped.
func commandWords(cmd string) ([]string, bool) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" || strings.ContainsAny(cmd, ";&|<>`\n\r") || strings.Contains(cmd, "$(") {
		return nil, false
	}
	words := strings.Fields(cmd)
	if strings.Contains(words[0], "=") { // VAR=value cmd
		return nil, false
	}
	prog := filepath.Base(strings.ReplaceAll(strings.Trim(words[0], `"'`), `\`, "/"))
	prog = strings.TrimSuffix(strings.ToLower(prog), ".exe")
	words[0] = prog
	if prog == "git" {
		rest := words[1:]
		for len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
			switch rest[0] {
			case "-C", "-c", "--git-dir", "--work-tree", "--namespace":
				if len(rest) > 1 {
					rest = rest[1:]
				}
			}
			rest = rest[1:]
		}
		words = append([]string{"git"}, rest...)
	}
	return words, true
}

func has(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f || strings.HasPrefix(a, f+"=") {
				return true
			}
		}
	}
	return false
}

// positional are the arguments that aren't flags.
func positional(args []string) []string {
	var out []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			out = append(out, a)
		}
	}
	return out
}

// gitClass classifies a git command (words after "git"), or "" when it
// isn't sure.
func gitClass(args []string) string {
	if len(args) == 0 {
		return GitRead
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "status", "diff", "log", "show", "blame", "shortlog", "describe", "ls-files", "ls-tree", "ls-remote",
		"rev-parse", "rev-list", "grep", "reflog", "whatchanged", "cat-file", "show-ref", "merge-base", "help", "version":
		return GitRead
	case "branch":
		switch {
		case has(rest, "-D", "--delete", "-d") && has(rest, "--force", "-f") || has(rest, "-D"):
			return GitDestructive
		case has(rest, "-d", "--delete", "-m", "-M", "--move", "-c", "-C", "--copy", "-u", "--set-upstream-to", "--unset-upstream"):
			return GitLocal
		case len(positional(rest)) == 0:
			return GitRead // listing
		default:
			return GitLocal // creating a branch
		}
	case "tag":
		switch {
		case has(rest, "-d", "--delete"):
			return GitDestructive
		case len(positional(rest)) == 0 || has(rest, "-l", "--list", "-v", "--verify"):
			return GitRead
		default:
			return GitCommit
		}
	case "remote":
		if len(rest) == 0 || rest[0] == "-v" || rest[0] == "show" || rest[0] == "get-url" {
			return GitRead
		}
		if rest[0] == "remove" || rest[0] == "rm" || rest[0] == "prune" {
			return GitDestructive
		}
		return GitLocal // add, set-url, rename, set-head
	case "config":
		if has(rest, "--get", "--get-all", "--get-regexp", "-l", "--list", "--show-origin") || len(positional(rest)) <= 1 && !has(rest, "--unset", "--unset-all") {
			return GitRead
		}
		return GitLocal
	case "stash":
		if len(rest) > 0 {
			switch rest[0] {
			case "list", "show":
				return GitRead
			case "drop", "clear":
				return GitDestructive
			}
		}
		return GitLocal // push, pop, apply
	case "add", "mv", "fetch", "init", "sparse-checkout", "worktree", "notes", "submodule":
		return GitLocal
	case "switch":
		if has(rest, "--discard-changes", "-f", "--force") {
			return GitDestructive
		}
		return GitLocal
	case "checkout":
		// "checkout -- file" and "checkout ." throw away changes. A plain
		// "checkout name" may be a branch or a file, so it isn't classed.
		switch {
		case has(rest, "--", "-f", "--force", "."):
			return GitDestructive
		case has(rest, "-b", "-B", "--orphan"):
			return GitLocal
		}
		return ""
	case "restore":
		if has(rest, "--staged", "-S") && !has(rest, "--worktree", "-W") {
			return GitLocal // unstaging
		}
		return GitDestructive
	case "rm":
		if has(rest, "--cached") {
			return GitLocal
		}
		return GitDestructive
	case "commit", "merge", "cherry-pick", "revert", "am", "apply":
		if sub == "merge" && has(rest, "--abort") {
			return GitLocal
		}
		return GitCommit
	case "pull":
		return GitCommit // fetches and merges into your branch
	case "push":
		if has(rest, "--force", "-f", "--force-with-lease", "--delete", "-d", "--mirror", "--prune") {
			return GitDestructive
		}
		return GitPublish
	case "reset":
		if has(rest, "--hard", "--merge", "--keep") {
			return GitDestructive
		}
		return GitLocal // unstaging, soft resets
	case "clean", "rebase", "filter-branch", "filter-repo", "gc", "prune", "update-ref", "replace":
		return GitDestructive
	}
	return ""
}

// ruleMatch is the action of the most specific rule matching a request,
// or "" when none does (ask).
func ruleMatch(rules []store.ToolRule, tool string, input json.RawMessage) string {
	action, _ := ruleScore(rules, tool, input)
	return action
}

// decide picks between the session's rules and the project's: the more
// specific match wins, and on a tie the session's (the more recent,
// deliberate choice). It also says where the answer came from.
func decide(sessionRules, projectRules []store.ToolRule, tool string, input json.RawMessage) (action, from string) {
	sa, ss := ruleScore(sessionRules, tool, input)
	pa, ps := ruleScore(projectRules, tool, input)
	switch {
	case ss == 0 && ps == 0:
		return "", ""
	case ss >= ps:
		return sa, "session"
	default:
		return pa, "rule"
	}
}

// ruleScore is the action of the most specific matching rule and how
// specific it was (0: none matched).
func ruleScore(rules []store.ToolRule, tool string, input json.RawMessage) (string, int) {
	var words []string
	var class string
	shell := IsShell(tool)
	if shell {
		w, ok := commandWords(bashCommand(input))
		if !ok {
			return "", 0 // only plain commands are covered by rules
		}
		words = w
		if words[0] == "git" {
			class = gitClass(words[1:])
		}
	}
	best, bestScore := "", 0
	for _, r := range rules {
		if r.Tool != tool && !(shell && IsShell(r.Tool)) {
			continue
		}
		score := 0
		switch {
		case r.Prefix == "":
			score = 1
		case strings.HasPrefix(r.Prefix, "git:"):
			if r.Prefix == class {
				score = 15
			}
		default:
			p := strings.Fields(r.Prefix)
			if len(p) <= len(words) && wordsMatch(p, words) {
				score = 10 * len(p)
				if len(p) == 1 {
					score = 5
				}
			}
		}
		if score > bestScore {
			best, bestScore = r.Action, score
		}
	}
	return best, bestScore
}

func wordsMatch(prefix, words []string) bool {
	for i, p := range prefix {
		if i == 0 && !strings.EqualFold(p, words[0]) || i > 0 && p != words[i] {
			return false
		}
	}
	return true
}

// suggestions are the rules an approval card offers under "Always allow":
// for a command, the command with its subcommand, its git class and the
// program (never every command); for other tools, the tool.
func suggestions(tool string, input json.RawMessage) []store.ToolRule {
	if !IsShell(tool) {
		return []store.ToolRule{{Tool: tool, Action: store.RuleAllow}}
	}
	words, ok := commandWords(bashCommand(input))
	if !ok {
		return nil // a compound command can only be allowed once
	}
	var out []store.ToolRule
	if len(words) > 1 && subcommandStyle[words[0]] && !strings.HasPrefix(words[1], "-") {
		out = append(out, store.ToolRule{Tool: tool, Prefix: words[0] + " " + words[1], Action: store.RuleAllow})
	}
	if words[0] == "git" {
		if c := gitClass(words[1:]); c != "" {
			out = append(out, store.ToolRule{Tool: tool, Prefix: c, Action: store.RuleAllow})
		}
	}
	out = append(out, store.ToolRule{Tool: tool, Prefix: words[0], Action: store.RuleAllow})
	return out
}

// Programs whose second word is a subcommand worth a rule of its own.
var subcommandStyle = map[string]bool{
	"git": true, "npm": true, "pnpm": true, "yarn": true, "bun": true, "go": true, "cargo": true, "dotnet": true,
	"docker": true, "kubectl": true, "helm": true, "gh": true, "pip": true, "uv": true, "poetry": true,
	"terraform": true, "az": true, "aws": true, "gcloud": true, "wails": true, "make": true,
}
