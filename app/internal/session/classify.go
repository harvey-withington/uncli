package session

import (
	"strings"

	"uncli/internal/core"
)

// Tool classes: whether a tool use only looks or changes something, may
// delete, and reaches outside this computer. Risk (risk.go) is worked out
// from the class.
//
// A tool's class comes from the built-in table for the CLI's own tools (a
// shell command by what each of its parts runs), then the tool's MCP
// annotations, which are hints from its server; otherwise it counts as
// changing things, destructively and reaching outside, the MCP spec's
// defaults for a tool that says nothing. The user corrects UNCLI through
// the safe list (class.go), not here.

// Class is what UNCLI takes a tool use to do.
type Class struct {
	Write       bool   `json:"write"`                 // changes something
	Destructive bool   `json:"destructive,omitempty"` // may delete or overwrite
	OpenWorld   bool   `json:"openWorld,omitempty"`   // reaches outside this computer
	Source      string `json:"source"`                // where the class came from
}

// Where a class came from, most trusted first.
const (
	SourceBuiltIn = "built-in"
	SourceServer  = "server" // the MCP server's annotations
	SourceUnknown = "unknown"
)

// classify says what a tool use does, from its provider-neutral action.
// hints are the MCP servers' word on their tools.
func classify(a core.ToolAction, hints map[string]core.ToolHint) Class {
	switch a.Kind {
	case core.ActShell:
		return shellClass(a.Command, a.Dialect)
	case core.ActRead, core.ActSearch, core.ActInternal, core.ActAgent, core.ActQuestion, core.ActPlan:
		return Class{Source: SourceBuiltIn} // sub-agents' own tool uses are asked about one by one
	case core.ActWeb:
		return Class{OpenWorld: true, Source: SourceBuiltIn}
	case core.ActWrite, core.ActEdit:
		return Class{Write: true, Source: SourceBuiltIn}
	case core.ActMCP:
		// The CLI reports only the hints that are true, so one left out
		// counts as false.
		if h, ok := hints[a.Tool]; ok {
			return Class{Write: !h.ReadOnly, Destructive: !h.ReadOnly && h.Destructive, OpenWorld: h.OpenWorld, Source: SourceServer}
		}
		return Class{Write: true, Destructive: true, OpenWorld: true, Source: SourceUnknown}
	}
	return Class{Write: true, Destructive: true, Source: SourceUnknown}
}

// asksUser: a question or a plan for the user, not an action; it always
// goes to them.
func asksUser(a core.ToolAction) bool { return a.Kind == core.ActQuestion || a.Kind == core.ActPlan }

// fileAction: an action on one file.
func fileAction(a core.ToolAction) bool {
	return a.Kind == core.ActRead || a.Kind == core.ActWrite || a.Kind == core.ActEdit || a.Kind == core.ActSearch
}

// shellClass classes a command line by its parts: it only looks when
// every part does, and reaches outside when any part does. A command
// rules can't cover (substitutions, file redirects…) changes things.
func shellClass(cmd, dialect string) Class {
	parts, _, ok := readScript(cmd, dialect, "")
	if !ok {
		return Class{Write: true, Source: SourceBuiltIn}
	}
	c := Class{Source: SourceBuiltIn}
	for _, p := range parts {
		if p.kind != "" {
			continue
		}
		pc := commandClass(p.words)
		if len(p.writes) > 0 {
			pc.Write = true
		}
		c.Write = c.Write || pc.Write
		c.Destructive = c.Destructive || pc.Destructive
		c.OpenWorld = c.OpenWorld || pc.OpenWorld
	}
	return c
}

// Programs that only look (besides the output filters in rules.go), and
// the ones that change the shell's own folder, which touches nothing.
var lookCommands = set("ls", "dir", "pwd", "echo", "exit", "throw", "read", "disown", "wait", "jobs", "unset", "shift", "printf", "whoami", "hostname", "date", "tree", "stat", "file",
	"du", "df", "uname", "which", "where", "type", "find", "rg", "fd", "diff", "cmp", "realpath", "dirname", "basename",
	"cd", "pushd", "popd", "true", "false", "comm", "tr", "seq", "sleep", "start-sleep", "cygpath", "netstat", "[", "[[", "test",
	"readlink", "id", "ps", "tasklist", "env", "printenv", "awk", "sed", "jq", "yq", "xxd", "od", "hexdump", "md5sum",
	"sha256sum", "certutil", "nproc", "uptime", "free", "lsof", "ss", "ping", "nslookup", "where.exe", "file", "column",
	"get-member", "gm", "get-acl", "get-service", "get-ciminstance", "get-wmiobject", "get-psdrive", "get-variable",
	"get-module", "get-alias", "get-history", "get-uptime", "get-nettcpconnection", "get-eventlog", "get-winevent",
	"select-xml", "convertfrom-json", "convertto-json", "out-string", "write-error", "write-warning", "write-verbose",
	"get-childitem", "gci", "get-content", "gc", "get-item", "gi", "get-itemproperty", "get-location", "gl",
	"test-path", "resolve-path", "split-path", "join-path", "get-command", "gcm", "get-process", "gps", "get-date",
	"get-help", "get-filehash", "write-output", "write-host", "set-location", "sl", "push-location", "pop-location")

// inPlace reports whether a filter that usually only reads (sed, awk)
// edits its files in place: sed -i, -i.bak, -Ei or --in-place, and gawk's
// -i inplace.
func inPlace(prog string, args []string) bool {
	switch prog {
	case "sed":
		for _, a := range args {
			if strings.HasPrefix(a, "--in-place") || strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "i") {
				return true
			}
		}
	case "awk", "gawk":
		for i, a := range args {
			if a == "-iinplace" || a == "--include=inplace" || (a == "-i" || a == "--include") && i+1 < len(args) && args[i+1] == "inplace" {
				return true
			}
		}
	}
	return false
}

// Programs that reach outside this computer.
var outsideCommands = set("curl", "wget", "ssh", "scp", "sftp", "rsync", "ftp", "telnet", "nc", "gh",
	"invoke-webrequest", "iwr", "invoke-restmethod", "irm")

// Programs that delete.
var deleteCommands = set("rm", "rmdir", "del", "erase", "rd", "remove-item", "ri", "shred")

// Options that make find change things.
var findActions = []string{"-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls"}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// commandClass classes one part of a command line (its words).
func commandClass(w []string) Class {
	prog := w[0]
	switch {
	case prog == "git":
		g := gitClass(w[1:])
		out := len(w) > 1 && (w[1] == "fetch" || w[1] == "pull" || w[1] == "push" || w[1] == "clone" || w[1] == "ls-remote")
		return Class{Write: g != GitRead, Destructive: g == GitDestructive, OpenWorld: out}
	case prog == "find" && has(w[1:], findActions...), prog == "sort" && has(w[1:], "-o", "--output"):
		return Class{Write: true}
	case inPlace(prog, w[1:]):
		return Class{Write: true}
	case lookCommands[prog] || outputFilters[prog]:
		return Class{}
	case outsideCommands[prog]:
		return Class{Write: true, OpenWorld: true}
	case deleteCommands[prog]:
		return Class{Write: true, Destructive: true}
	}
	return Class{Write: true}
}
