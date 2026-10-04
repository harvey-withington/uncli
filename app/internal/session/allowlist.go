package session

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"uncli/internal/core"
)

// The profile's allowlist (allowed_tools), in the CLI's own syntax. When
// the CLI routes its permission prompts to UNCLI, the allowlist isn't
// passed to the CLI: the CLI would approve those tools itself, before the
// session mode or the project's deny rules had a say. UNCLI applies it
// instead, in Ask mode, after the rules.
//
//	Write                  the whole tool
//	Bash(git status:*)     commands starting with those words
//	Bash(npm test)         that command exactly
//	Write(./artifacts/**)  files matching a glob: relative to the session
//	                       folder ("./x", "x" or "/x"), absolute ("//x") or
//	                       under the home folder ("~/x")
//
// Other qualified forms (WebFetch(domain:…)) are never matched.

type allowEntry struct {
	src   string // as written in the profile
	tool  string
	any   bool           // the whole tool
	words []string       // shell: the command's words
	exact bool           // shell: the whole command, not a prefix
	glob  *regexp.Regexp // file tools: the path
}

var allowSyntax = regexp.MustCompile(`^([A-Za-z0-9_]+)(?:\((.*)\))?$`)

// File tools and the input field holding their path.
var fileTools = map[string]bool{"Read": true, "Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true}

func parseAllowlist(list []string, workdir string) []allowEntry {
	var out []allowEntry
	for _, s := range list {
		m := allowSyntax.FindStringSubmatch(strings.TrimSpace(s))
		if m == nil {
			continue
		}
		e := allowEntry{src: strings.TrimSpace(s), tool: m[1]}
		spec := strings.TrimSpace(m[2])
		switch {
		case spec == "":
			e.any = true
		case IsShell(e.tool):
			e.exact = !strings.HasSuffix(spec, ":*")
			e.words = strings.Fields(strings.TrimSuffix(spec, ":*"))
			if len(e.words) == 0 {
				e.any = true
			}
		case fileTools[e.tool]:
			e.glob = globRegexp(globPath(spec, workdir))
		default:
			continue
		}
		out = append(out, e)
	}
	return out
}

// globPath makes a pattern absolute, with forward slashes.
func globPath(p, workdir string) string {
	home, _ := os.UserHomeDir()
	switch {
	case strings.HasPrefix(p, "//"):
		p = p[1:]
	case strings.HasPrefix(p, "~/"):
		p = filepath.ToSlash(home) + p[1:]
	default:
		p = filepath.ToSlash(workdir) + "/" + strings.TrimPrefix(strings.TrimPrefix(p, "./"), "/")
	}
	return p
}

func globRegexp(p string) *regexp.Regexp {
	var b strings.Builder
	if runtime.GOOS == "windows" {
		b.WriteString("(?i)")
	}
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '*' && i+1 < len(p) && p[i+1] == '*':
			i++
			if i+1 < len(p) && p[i+1] == '/' {
				i++
				b.WriteString("(?:.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// absPath is a file path made absolute (from the session folder), with
// forward slashes; empty for none.
func absPath(p, workdir string) string {
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(workdir, p)
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// allowedBy reports whether an allowlist covers a tool use: for a shell,
// one working part of its command (words); otherwise the tool's input.
func allowedBy(list []allowEntry, a core.ToolAction, words []string, workdir string) bool {
	_, ok := allowEntryFor(list, a, words, workdir)
	return ok
}

// allowEntryFor is the allowlist entry (as written) that covers a tool use.
func allowEntryFor(list []allowEntry, a core.ToolAction, words []string, workdir string) (string, bool) {
	for _, e := range list {
		if e.tool != a.Tool && !(a.Kind == core.ActShell && IsShell(e.tool)) {
			continue
		}
		switch {
		case e.any:
			return e.src, true
		case e.words != nil:
			if len(e.words) <= len(words) && wordsMatch(e.words, words) && (!e.exact || len(e.words) == len(words)) {
				return e.src, true
			}
		case e.glob != nil:
			if p := absPath(a.Path, workdir); p != "" && e.glob.MatchString(p) {
				return e.src, true
			}
		}
	}
	return "", false
}
