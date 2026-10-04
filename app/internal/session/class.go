package session

import (
	"path/filepath"
	"sort"
	"strings"
	"uncli/internal/core"

	"uncli/internal/store"
)

// Command classes: what one "This is safe" (or "This should prompt")
// remembers. A class is a command's identity without what changes from
// run to run: the program, its subcommand where it has one (npm run test,
// git push, gh pr create), and the flags that change its risk (git push
// --force, npm install --global, Remove-Item -Recurse). Files, paths,
// messages and other options are left out, so marking `npm run test -- -u
// src/a.test.ts` safe covers every run of the test script, and marking
// `git push` safe never covers `git push --force`.
//
// Some things can't be learned, because what makes them unsafe isn't the
// command but this use of it: code passed inline (node -e), a command too
// complex to read, or one that works outside the session folder, on
// secrets, or on code piped into it.

// Why nothing can be learned.
const (
	FixedInline  = "inline"  // code on the command line (node -e, python -c, iex)
	FixedComplex = "complex" // substitutions, braces, file redirects
	FixedContext = "context" // unsafe because of where it works, not what it is
)

// classOf is a command part's class, and why it can't be learned (empty if
// it can). The class is always filled in, so a Blocked entry for the
// program still matches.
func classOf(p part) (store.SafeClass, string) {
	w := p.words
	prog, args := w[0], w[1:]
	pos := positional(args)
	sub := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub = args[0]
	}
	words := []string{prog}
	fixed := ""
	switch {
	case codeRunners[prog]:
		fixed = FixedInline
	case interpreters[prog] && (has(args, inlineCode...) || p.inline || p.piped || len(args) == 0 || has(args, "-")):
		fixed = FixedInline // code given on the command line, in a heredoc or through a pipe
	case interpreters[prog]:
		if i := indexOf(args, "-m"); i >= 0 && i+1 < len(args) { // python -m pytest, python -m pip install
			words = append(words, "-m", args[i+1])
			if rest := positional(args[i+2:]); len(rest) > 0 && subcommandStyle[strings.ToLower(args[i+1])] {
				words = append(words, rest[0])
			}
		} else if len(pos) > 0 {
			words = append(words, filepath.ToSlash(pos[0])) // the script is the class
		}
	case packageManagers[prog]:
		if (sub == "run" || sub == "run-script") && len(pos) > 1 {
			words = append(words, "run", pos[1])
		} else if sub != "" {
			words = append(words, sub)
		}
	case prog == "gh":
		words = append(words, pos[:min(2, len(pos))]...)
	case prog == "git" || subcommandStyle[prog] || cloudCLIs[prog] || osInstallers[prog] || systemTools[prog] || prog == "pip" || prog == "pip3" || prog == "uv" || prog == "poetry":
		if sub != "" {
			words = append(words, sub)
		}
	case deleteCommands[prog] || prog == "remove-item" || prog == "ri":
		t := targets(pos)
		if len(t) == 0 || strings.Contains(strings.Join(t, " "), "$") {
			fixed = FixedComplex // what it deletes is only known when it runs
		}
		words = append(words, t...) // rm -rf node_modules, not every recursive delete
	case prog == "taskkill" || prog == "stop-process" || prog == "spps" || prog == "pkill" || prog == "killall":
		for i, a := range args {
			if (strings.EqualFold(a, "/IM") || strings.EqualFold(a, "-Name")) && i+1 < len(args) {
				words = append(words, strings.ToLower(args[i+1]))
			}
		}
		if prog == "pkill" || prog == "killall" {
			words = append(words, pos...)
		}
	case remoteCommands[prog] && len(pos) > 0:
		pos = positional(skipValues(args, "-o", "-i", "-p", "-P", "-l", "-F", "-J", "-e"))
		host := pos[min(0, len(pos)-1)]
		if prog == "scp" || prog == "rsync" {
			for _, a := range pos {
				if h, _, ok := strings.Cut(a, ":"); ok && !drivePath.MatchString(a) {
					host = h
				}
			}
		}
		words = append(words, host) // ssh me@server, not every connection
	case toolchain(prog):
		// Any stack's build tool or task runner: its task is the class
		// (mvn deploy, gradle test), files aren't (pytest tests/a.py).
		if sub != "" && taskWord(sub) {
			words = append(words, strings.ToLower(sub))
		} else {
			for _, w := range pos { // mvn -B deploy: the publishing task, after the options
				if taskWord(w) && publishing(prog, []string{w}) {
					words = append(words, strings.ToLower(w))
					break
				}
			}
		}
	case strings.Contains(prog, "-"), lookCommands[prog], outputFilters[prog], fileCommands[prog],
		downloaders[prog], systemCommands[prog], stopCommands[prog], remoteCommands[prog], osInstallers[prog]:
		// The program alone; its risk flags say the rest.
	default: // a program UNCLI doesn't know: its first word, unless that's a file
		if sub != "" && !pathLike(sub) {
			words = append(words, sub)
		}
	}
	flags := riskFlags(prog, sub, args)
	// Publishing the words don't show (docker buildx build --push, gradle
	// build publish): a flag, so an entry without it never covers it.
	m := indexOf(args, "-m")
	if (toolchain(prog) && publishing(prog, args) || interpreters[prog] && m >= 0 && publishing(prog, args[m+1:])) &&
		!publishing(prog, words[1:]) {
		flags = append(flags, "--publish")
		sort.Strings(flags)
	}
	if toolchain(prog) && installsSystemWide(prog, args) && !hasWord(words[1:], "install", "global") {
		flags = append(flags, "--install") // cmake --install
		sort.Strings(flags)
	}
	return store.SafeClass{Kind: store.KindCommand, Words: strings.Join(words, " "),
		Flags: strings.Join(flags, " ")}, fixed
}

// riskFlags are the options that change what a command can do, in a
// canonical spelling, so `Remove-Item -Recurse` and `rm -rf` both carry -r.
func riskFlags(prog, sub string, args []string) []string {
	var out []string
	add := func(f string) { out = append(out, f) }
	switch {
	case prog == "git":
		switch sub {
		case "push":
			if has(args, "-f", "--force", "--force-with-lease", "--mirror", "--prune") {
				add("--force")
			}
			if has(args, "-d", "--delete") {
				add("--delete")
			}
		case "reset":
			if has(args, "--hard", "--merge", "--keep") {
				add("--hard")
			}
		case "branch":
			if has(args, "-D") || has(args, "-d", "--delete") && has(args, "-f", "--force") {
				add("-D")
			}
		case "checkout", "switch":
			if has(args, "--", "-f", "--force", ".", "--discard-changes") {
				add("--discard")
			}
		case "stash":
			if hasWord(args, "drop", "clear") {
				add("--drop")
			}
		case "tag", "remote":
			if has(args, "-d", "--delete") || hasWord(args, "remove", "rm", "prune") {
				add("--delete")
			}
		}
	case packageManagers[prog]:
		if has(args, "-g", "--global", "--location=global") {
			add("--global")
		}
	case deleteCommands[prog]:
		if has(args, "-r", "-R", "-rf", "-fr", "-Recurse", "-recurse", "/s", "/S", "--recursive") {
			add("-r")
		}
		if has(args, "-f", "-rf", "-fr", "-Force", "-force", "--force", "/q", "/Q") {
			add("-f")
		}
		if wildcard(args) {
			add("*")
		}
	case downloaders[prog]:
		if has(args, sendFlags...) {
			add("--send")
		}
	case prog == "find":
		for _, a := range findActions {
			if has(args, a) {
				add(a) // -delete, -exec…
				break
			}
		}
	case prog == "start-process":
		if hasWord(args, "RunAs") {
			add("-runas")
		}
	case prog == "go" || prog == "cargo" || prog == "dotnet":
		if has(args, "-g", "--global") {
			add("--global")
		}
	}
	sort.Strings(out)
	return out
}

// targets are the paths a command works on, as the class remembers them.
// System tools whose first word is what they do (reg add, sc stop,
// systemctl restart): part of their class, so "reg query" isn't "reg add".
var systemTools = set("reg", "sc", "net", "netsh", "schtasks", "systemctl", "service", "launchctl", "defaults", "dism", "bcdedit")

func targets(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		t := strings.TrimRight(filepath.ToSlash(p), "/")
		if t == "" {
			t = "/"
		}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// skipValues drops the given options and the value after each.
func skipValues(args []string, opts ...string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if hasWord([]string{args[i]}, opts...) {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

// pathLike: a word that names a file or folder rather than a subcommand.
func pathLike(w string) bool {
	return strings.ContainsAny(w, `/\:~*`) || strings.HasPrefix(w, ".") || filepath.Ext(w) != ""
}

// flagsCovered reports whether an entry's flags include every risk flag a
// command has: "git push" doesn't cover "git push --force".
func flagsCovered(have, entry string) bool {
	allowed := map[string]bool{}
	for _, f := range strings.Fields(entry) {
		allowed[f] = true
	}
	for _, f := range strings.Fields(have) {
		if !allowed[f] {
			return false
		}
	}
	return true
}

// matchEntry finds the safe-list entry that applies to a tool use or a
// command part: the most specific (more words, then a git class, then
// more flags), and on a tie the project's over all projects'.
func matchEntry(list []store.SafeEntry, a core.ToolAction, class store.SafeClass, words []string) (store.SafeEntry, bool) {
	var best store.SafeEntry
	bestScore := 0
	tool := a.Tool
	shell := a.Kind == core.ActShell
	cw := strings.Fields(class.Words)
	for _, e := range list {
		s := 0
		switch {
		case !shell && e.Kind == store.KindTool && e.Words == tool:
			s = 10
		case shell && e.Kind == store.KindCommand && strings.HasPrefix(e.Words, "git:"):
			if len(words) > 0 && words[0] == "git" && gitClass(words[1:]) == e.Words {
				s = 25
			}
		case shell && e.Kind == store.KindCommand:
			ew := strings.Fields(e.Words)
			// Risk flags only narrow what Safe covers: a Blocked or Unsafe
			// "git push" also covers "git push --force".
			if len(ew) <= len(cw) && wordsMatch(ew, cw) && (e.Verdict != store.Safe || flagsCovered(class.Flags, e.Flags)) {
				s = 10*len(ew) + len(strings.Fields(e.Flags))
			}
		}
		if s == 0 {
			continue
		}
		s *= 2
		if e.Folder != "" {
			s++ // this project's entry wins a tie
		}
		if s > bestScore {
			best, bestScore = e, s
		}
	}
	return best, bestScore > 0
}
