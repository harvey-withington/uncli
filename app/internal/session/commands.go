package session

import (
	"strings"
)

// Commands: how UNCLI reads a command line. It is split into parts at
// pipes and chains outside quotes; each part is one program with its
// arguments, read into words with the program normalised and global
// options before a subcommand dropped. Risk (risk.go) and the safe list
// (safelist.go) work part by part, so "git status && rm -rf ." is never
// judged by its first part. A command with substitutions, braces or file
// redirects can't be read into parts at all.

// Shell tools run a command line: Bash, and PowerShell on Windows. Their
// rules are about the command, not the tool, so a rule written for one
// covers both, and none of them is ever allowed wholesale from a card.
var shellTools = map[string]bool{"Bash": true, "PowerShell": true}

// IsShell reports whether a tool runs command lines.
func IsShell(tool string) bool { return shellTools[tool] }

// dialectOf is the command-line dialect of a shell tool.
func dialectOf(tool string) string {
	if tool == "PowerShell" {
		return DialectPowerShell
	}
	return DialectBash
}

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

// Programs that take options before their subcommand, with the options
// that take a value (any other leading option is a flag).
var globalOptions = map[string]map[string]bool{
	"git":    {"-C": true, "-c": true, "--git-dir": true, "--work-tree": true, "--namespace": true},
	"npm":    {"--prefix": true, "-w": true, "--workspace": true, "--loglevel": true, "--userconfig": true, "--cache": true},
	"pnpm":   {"-C": true, "--dir": true, "--filter": true, "-F": true},
	"yarn":   {"--cwd": true},
	"go":     {"-C": true},
	"cargo":  {"-C": true, "--config": true},
	"docker": {"-H": true, "--host": true, "--context": true, "-c": true, "--config": true, "-l": true, "--log-level": true},
	"make":   {"-C": true, "--directory": true, "-f": true, "--file": true},
}

// splitWords splits at spaces outside quotes, removing the quotes.
func splitWords(s string) []string {
	var out []string
	var b strings.Builder
	var quote rune
	word := false
	for _, c := range s {
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				b.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote, word = c, true
		case c == ' ' || c == '\t':
			if word {
				out = append(out, b.String())
				b.Reset()
				word = false
			}
		default:
			b.WriteRune(c)
			word = true
		}
	}
	if word {
		out = append(out, b.String())
	}
	return out
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
		"rev-parse", "rev-list", "check-ignore", "grep", "reflog", "whatchanged", "cat-file", "show-ref", "merge-base", "help", "version":
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

// Output filters: they read what's piped in and print part of it.
var outputFilters = map[string]bool{
	"%": true, "?": true, "where-object": true, "foreach-object": true,
	"head": true, "tail": true, "grep": true, "egrep": true, "sort": true, "uniq": true, "wc": true, "cut": true,
	"less": true, "more": true, "cat": true, "findstr": true, "jq": true, "column": true,
	"select-object": true, "select-string": true, "sort-object": true, "measure-object": true, "group-object": true,
	"out-string": true, "out-host": true, "format-table": true, "format-list": true, "format-wide": true,
	"ft": true, "fl": true, "measure": true,
}

// wordsMatch reports whether words start with prefix (the program's case
// ignored).
func wordsMatch(prefix, words []string) bool {
	for i, p := range prefix {
		if i == 0 && !strings.EqualFold(p, words[0]) || i > 0 && p != words[i] {
			return false
		}
	}
	return true
}

// Programs whose second word is a subcommand: it is part of a command's
// class (risk.go).
var subcommandStyle = map[string]bool{
	"git": true, "npm": true, "pnpm": true, "yarn": true, "bun": true, "go": true, "cargo": true, "dotnet": true,
	"docker": true, "kubectl": true, "helm": true, "gh": true, "pip": true, "uv": true, "poetry": true,
	"terraform": true, "az": true, "aws": true, "gcloud": true, "wails": true, "make": true,
}
