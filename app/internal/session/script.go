package session

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Reading a command line into the commands it runs, for bash and
// PowerShell. Each command (a "part") is judged on its own, so one part
// UNCLI can't place never makes the rest unreadable:
//
//   - parts are split at ; && || | & and newlines outside quotes; a part
//     after a single | reads the previous one's output;
//   - code nested in $( ), backticks (bash), { } and ( ) is read the same
//     way, recursively, and judged as parts of its own;
//   - heredocs and PowerShell here-strings are data (but code when fed to
//     an interpreter);
//   - redirects become the files a part writes or reads (> f, >> f, 2> f,
//     < f; 2>&1 and > $null are harmless);
//   - assignments (VAR=x, $x = …), bare strings and numbers, and shell
//     keywords (if, for, do, done, foreach…) are structure, not programs;
//   - wrappers (timeout, time, env, nohup, xargs…) are seen through, and
//     the PowerShell call operator (& prog) is dropped.

// part is one command of a command line.
type part struct {
	text   string   // as written (redirects included)
	words  []string // program (normalised) and arguments; quotes removed, global options dropped
	piped  bool     // reads the previous part's output
	writes []string // files its output is redirected to
	reads  []string // files redirected into it
	inline bool     // it is fed code it runs (a heredoc into an interpreter)
	kind   string   // "" (a program), or structure: assign, literal, keyword
	cwd    string   // the folder it runs in, when a cd before it changed that
	from   string   // the program whose output it reads (piped)
	body   string   // the heredoc fed to it, if any
}

// Dialects of command line.
const (
	DialectBash       = "bash"
	DialectPowerShell = "powershell"
)

var (
	bashAssign   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	psAssign     = regexp.MustCompile(`^\$[A-Za-z_][\w:.]*(\[[^\]]*\])?\s*(\+|-|\*|/)?=(\s|$)`)
	literalWord  = regexp.MustCompile(`^(-?\d+(\.\d+)?|\$[\w:]+(\.\w+)*|\$null|\$true|\$false)$`)
	redirectWord = regexp.MustCompile(`^(\d|\*|&)?(>>?|<)(&\d)?$`)
	staticCall   = regexp.MustCompile(`^\[[\w.]+\]::(\w+)`)
	redirectJoin = regexp.MustCompile(`^(\d|\*|&)?(>>?|<)(&\d|.+)$`)
)

var keywords = set("if", "then", "elif", "else", "fi", "for", "while", "until", "do", "done", "case", "esac", "in",
	"function", "select", "{", "}", "!", "elseif", "foreach", "switch", "try", "catch", "finally", "param", "return",
	"break", "continue", "trap", "begin", "process", "end")

// Wrappers run the command that follows them; their own options come first.
var wrappers = map[string]map[string]bool{ // wrapper -> options that take a value
	"timeout": {"-k": true, "-s": true, "--signal": true, "--kill-after": true},
	"time":    {}, "nohup": {}, "nice": {"-n": true}, "ionice": {"-c": true, "-n": true}, "command": {}, "builtin": {},
	"exec": {}, "stdbuf": {"-i": true, "-o": true, "-e": true}, "env": {"-u": true, "-C": true}, "xargs": {"-I": true, "-n": true, "-P": true, "-d": true, "-L": true, "-E": true, "-s": true},
}

// readScript reads a command line into its parts, or false when it can't
// be read at all (an unclosed quote or bracket). cwd is the folder the
// shell is in; a cd changes it for the parts after it, and the folder it
// ends in is returned.
func readScript(cmd, dialect, cwd string) ([]part, string, bool) {
	r := &reader{dialect: dialect, vars: map[string]string{}, cwd: cwd, funcs: map[string]bool{}}
	parts, ok := r.seq(cmd)
	if !ok {
		return nil, cwd, false
	}
	return parts, r.cwd, true
}

// workingParts are the parts of a command line that run programs, less
// those that only filter output (head, Select-Object…) when anything else
// runs; or false when it can't be read.
func workingParts(cmd, dialect, cwd string) ([]part, bool) {
	parts, _, ok := readScript(cmd, dialect, cwd)
	if !ok {
		return nil, false
	}
	var all, main []part
	for _, p := range parts {
		if p.kind != "" {
			continue
		}
		all = append(all, p)
		if !outputFilters[p.words[0]] || len(p.writes) > 0 {
			main = append(main, p)
		}
	}
	if len(main) == 0 {
		main = all // a filter on its own (grep x file) still counts
	}
	return main, true
}

type reader struct {
	last    string // the program of the last part read
	heredoc string // the body of the heredoc in the part being read
	dialect string
	vars    map[string]string // simple assignments seen so far, for paths
	cwd     string            // the folder the shell is in
	funcs   map[string]bool   // functions defined in the command line; their bodies are judged where they're defined
}

// Programs that change the shell's folder.
var cdCommands = set("cd", "chdir", "set-location", "sl", "pushd", "push-location")

// seq reads a sequence of commands.
func (r *reader) seq(s string) ([]part, bool) {
	var out []part
	var b strings.Builder
	piped := false
	var nested []part
	flush := func(nextPiped bool) {
		from := r.last
		got := r.part(b.String(), piped)
		for k := range got {
			if got[k].kind == "" {
				if piped {
					got[k].from = from
				}
				if r.heredoc != "" {
					got[k].body = r.heredoc
				}
				r.last = got[k].words[0]
			}
		}
		r.heredoc = ""
		out = append(out, got...)
		out = append(out, nested...)
		b.Reset()
		nested = nil
		piped = nextPiped
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\'':
			j := closing(rs, i, '\'')
			if j < 0 {
				return nil, false
			}
			if r.dialect == DialectPowerShell && i > 0 && rs[i-1] == '@' { // @' … '@ here-string
				if k := strings.Index(string(rs[i+1:]), "\n'@"); k >= 0 {
					j = i + 1 + len([]rune(string(rs[i+1:])[:k])) + 2
				}
			}
			b.WriteString(string(rs[i : j+1]))
			i = j
		case c == '"':
			j, inner, ok := r.doubleQuoted(rs, i)
			if !ok {
				return nil, false
			}
			nested = append(nested, inner...)
			b.WriteString(string(rs[i : j+1]))
			i = j
		case c == '`' && r.dialect == DialectBash:
			j := closing(rs, i, '`')
			if j < 0 {
				return nil, false
			}
			inner, ok := r.seq(string(rs[i+1 : j]))
			if !ok {
				return nil, false
			}
			nested = append(nested, inner...)
			b.WriteString("$SUB")
			i = j
		case c == '`' && r.dialect == DialectPowerShell: // escape: keep the next character as it is
			b.WriteRune(c)
			if i+1 < len(rs) {
				i++
				b.WriteRune(rs[i])
			}
		case c == '\\' && r.dialect == DialectBash && i+1 < len(rs):
			if rs[i+1] != '\n' { // an escaped character stays as it is; a line continuation goes
				b.WriteRune(c)
				b.WriteRune(rs[i+1])
			}
			i++
		case c == '$' && i+1 < len(rs) && rs[i+1] == '{':
			j := matching(rs, i+1) // a variable: ${NAME}
			if j < 0 {
				return nil, false
			}
			b.WriteString(string(rs[i : j+1]))
			i = j
		case c == '$' && i+1 < len(rs) && rs[i+1] == '(', c == '@' && i+1 < len(rs) && rs[i+1] == '(':
			j := matching(rs, i+1)
			if j < 0 {
				return nil, false
			}
			inner, ok := r.seq(string(rs[i+2 : j]))
			if !ok {
				return nil, false
			}
			nested = append(nested, inner...)
			b.WriteString("$SUB")
			i = j
		case c == '{' && r.dialect == DialectBash && (i+1 >= len(rs) || rs[i+1] != ' ' && rs[i+1] != '\n'):
			b.WriteRune(c) // brace expansion ({a,b}), not a group
		case c == '@' && i+1 < len(rs) && rs[i+1] == '{' && r.dialect == DialectPowerShell:
			j := matching(rs, i+1) // a hashtable: data
			if j < 0 {
				return nil, false
			}
			b.WriteString(" $BLOCK ")
			i = j
		case c == '{' || c == '(':
			j := matching(rs, i)
			if j < 0 {
				return nil, false
			}
			inner, ok := r.seq(string(rs[i+1 : j]))
			if !ok {
				return nil, false
			}
			nested = append(nested, inner...)
			b.WriteString(" $BLOCK ")
			i = j
		case c == '<' && i+1 < len(rs) && rs[i+1] == '<' && r.dialect == DialectBash:
			// A heredoc: its body is data, up to the line that is its delimiter.
			j := i + 2
			for j < len(rs) && (rs[j] == '-' || rs[j] == '~' || rs[j] == ' ') {
				j++
			}
			k := j
			for k < len(rs) && rs[k] != '\n' && rs[k] != ' ' && rs[k] != ';' && rs[k] != ')' && rs[k] != '|' && rs[k] != '&' {
				k++
			}
			delim := strings.Trim(string(rs[j:k]), `'"\`)
			b.WriteString(" $HEREDOC ")
			if delim == "" {
				i = k - 1
				continue
			}
			rest := string(rs[k:])
			nl := strings.Index(rest, "\n")
			if nl < 0 {
				i = k - 1
				continue
			}
			// Keep what follows the delimiter on its line (a pipe, a redirect…).
			b.WriteString(rest[:nl])
			body := rest[nl+1:]
			end := -1
			for off, line := 0, ""; off <= len(body); {
				e := strings.Index(body[off:], "\n")
				if e < 0 {
					line = body[off:]
				} else {
					line = body[off : off+e]
				}
				if strings.TrimSpace(line) == delim {
					end = off + len(line)
					break
				}
				if e < 0 {
					break
				}
				off += e + 1
			}
			if end < 0 {
				r.heredoc = body
			} else {
				r.heredoc = body[:max(0, end-len(delim))]
			}
			if end < 0 {
				i = len(rs) - 1
			} else {
				i = k + nl + 1 + len([]rune(body[:end])) - 1
			}
		case c == '|' && i+1 < len(rs) && rs[i+1] == '|', c == '&' && i+1 < len(rs) && rs[i+1] == '&':
			flush(false)
			i++
		case c == '|':
			flush(true)
		case c == ';' || c == '\n' || c == '\r':
			flush(false)
		case c == '&' && r.dialect == DialectBash && !(i > 0 && (rs[i-1] == '>' || rs[i-1] == '<')) && !(i+1 < len(rs) && rs[i+1] == '>'):
			flush(false) // runs in the background; the next command is separate
		case c == '#' && (i == 0 || rs[i-1] == ' ' || rs[i-1] == '\n' || rs[i-1] == '\t'):
			for i < len(rs) && rs[i] != '\n' { // a comment
				i++
			}
			i--
		default:
			b.WriteRune(c)
		}
	}
	flush(false)
	return out, true
}

// doubleQuoted finds the end of a double-quoted string and reads the
// substitutions inside it.
func (r *reader) doubleQuoted(rs []rune, i int) (int, []part, bool) {
	var nested []part
	for j := i + 1; j < len(rs); j++ {
		switch {
		case rs[j] == '\\' && r.dialect == DialectBash, rs[j] == '`' && r.dialect == DialectPowerShell:
			j++
		case rs[j] == '$' && j+1 < len(rs) && rs[j+1] == '(':
			k := matching(rs, j+1)
			if k < 0 {
				return 0, nil, false
			}
			inner, ok := r.seq(string(rs[j+2 : k]))
			if !ok {
				return 0, nil, false
			}
			nested = append(nested, inner...)
			j = k
		case rs[j] == '`' && r.dialect == DialectBash:
			k := closing(rs, j, '`')
			if k < 0 {
				return 0, nil, false
			}
			inner, ok := r.seq(string(rs[j+1 : k]))
			if !ok {
				return 0, nil, false
			}
			nested = append(nested, inner...)
			j = k
		case rs[j] == '"':
			return j, nested, true
		}
	}
	return 0, nil, false
}

// closing finds the next unescaped quote character q after position i.
func closing(rs []rune, i int, q rune) int {
	for j := i + 1; j < len(rs); j++ {
		if rs[j] == q {
			return j
		}
	}
	return -1
}

// matching finds the bracket closing the one at position i, skipping
// quoted text and nested brackets.
func matching(rs []rune, i int) int {
	open := rs[i]
	shut := map[rune]rune{'(': ')', '{': '}', '[': ']'}[open]
	depth := 0
	for j := i; j < len(rs); j++ {
		switch rs[j] {
		case '\'', '"':
			k := closing(rs, j, rs[j])
			if k < 0 {
				return -1
			}
			j = k
		case open:
			depth++
		case shut:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// part reads one command (possibly empty) into parts: the command itself,
// or nothing when it is only structure.
func (r *reader) part(text string, piped bool) []part {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	p := part{text: text, piped: piped}
	raw := splitWords(text)
	// Redirects: their targets are files written or read.
	var words []string
	for k := 0; k < len(raw); k++ {
		w := raw[k]
		if w == "$HEREDOC" {
			continue
		}
		if m := redirectJoin.FindStringSubmatch(w); m != nil && !redirectWord.MatchString(w) && !strings.HasPrefix(m[3], "&") {
			r.redirect(&p, m[2], m[3])
			continue
		}
		if redirectWord.MatchString(w) {
			if strings.HasSuffix(w, "&1") || strings.HasSuffix(w, "&2") || strings.Contains(w, "&") && len(w) > 2 {
				continue
			}
			if k+1 < len(raw) {
				r.redirect(&p, strings.TrimLeft(w, "0123456789*&"), raw[k+1])
				k++
			}
			continue
		}
		words = append(words, w)
	}
	if strings.Contains(text, "$HEREDOC") || strings.Contains(text, "<<") {
		p.inline = true // marked here; only interpreters treat it as code (classOf)
	}
	// The PowerShell call operator and dot-sourcing.
	for len(words) > 0 && (words[0] == "&" || words[0] == "." && len(words) > 1) {
		words = words[1:]
	}
	// Assignments.
	if r.dialect == DialectPowerShell {
		if m := psAssign.FindString(text); m != "" {
			name := strings.TrimSpace(strings.SplitN(m, "=", 2)[0])
			rest := strings.TrimSpace(text[len(m):])
			if v := unquote(rest); v != rest || !strings.ContainsAny(rest, " 	$(") {
				r.vars[strings.ToLower(name)] = v // a plain value, usable in paths
			}
			sub := r.part(rest, piped)
			if len(sub) == 0 || sub[0].kind != "" {
				return []part{{text: text, kind: "assign"}}
			}
			return sub
		}
	}
	// Assignments, also after a keyword (do x=…).
	for k := 0; k < len(words); k++ {
		if keywords[strings.ToLower(words[k])] {
			continue
		}
		if !bashAssign.MatchString(words[k]) {
			break
		}
		kv := strings.SplitN(words[k], "=", 2)
		r.vars["$"+strings.ToLower(kv[0])] = kv[1]
		words = append(words[:k:k], words[k+1:]...)
		k--
	}
	// Keywords and nested code: what follows them may still be a command
	// (do X, then X, if X; } else { … }).
	// PowerShell's unary comma makes an array of a value (,$c.properties):
	// data, not a command.
	if len(words) == 1 && r.dialect == DialectPowerShell && strings.HasPrefix(words[0], ",$") && !strings.Contains(words[0], "(") {
		return []part{{text: text, kind: "assign"}}
	}
	for len(words) > 0 && (keywords[strings.ToLower(words[0])] || words[0] == "$BLOCK" || words[0] == "$SUB" || words[0] == ",") {
		switch strings.ToLower(words[0]) {
		case "function":
			if len(words) > 1 {
				r.funcs[strings.ToLower(strings.TrimSuffix(words[1], "()"))] = true
			}
			return []part{{text: text, kind: "keyword"}}
		case "for", "foreach", "select", "case", "param", "switch":
			return []part{{text: text, kind: "keyword"}} // a loop header or a definition
		}
		words = words[1:]
	}
	// name() { … }: a bash function; calling one runs what was judged in it.
	if len(words) == 3 && words[1] == "$BLOCK" && words[2] == "$BLOCK" && r.dialect == DialectBash {
		r.funcs[strings.ToLower(words[0])] = true
		return []part{{text: text, kind: "keyword"}}
	}
	if len(words) > 0 && r.funcs[strings.ToLower(words[0])] {
		return []part{{text: text, kind: "call"}}
	}
	if len(words) == 0 {
		if p.writes != nil || p.reads != nil {
			p.words = []string{"$redirect"}
			return []part{p}
		}
		return []part{{text: text, kind: "keyword"}}
	}
	// .NET static calls: [IO.File]::ReadAllText(…) looks, ::WriteAllText(…) writes.
	if m := staticCall.FindStringSubmatch(words[0]); m != nil {
		method := strings.ToLower(m[1])
		for _, look := range []string{"read", "get", "exists", "test", "combine", "join", "parse", "format", "equals", "to", "is", "now", "new", "match", "escape", "replace", "split", "unescape", "compare", "concat", "trim"} {
			if strings.HasPrefix(method, look) {
				return []part{{text: text, kind: "literal"}}
			}
		}
		p.words = []string{"$dotnet." + method}
		return []part{p}
	}
	// Expressions: a property (.Count), an operator (-not), @.
	if r.dialect == DialectPowerShell && (strings.HasPrefix(words[0], ".") && len(words[0]) > 1 && words[0][1] != '/' && words[0][1] != '\\' && words[0][1] != '.' ||
		strings.HasPrefix(words[0], "-") || words[0] == "@" || words[0] == "+" || words[0] == "!") {
		return []part{{text: text, kind: "literal"}}
	}
	// A bare string, number or variable prints itself.
	if literalWord.MatchString(strings.TrimRight(words[0], ",")) && strings.HasSuffix(words[0], ",") {
		return []part{{text: text, kind: "literal"}} // a list: 0, $s
	}
	if len(words) == 1 && (literalWord.MatchString(words[0]) || isQuoted(text)) || isQuoted(text) && !strings.ContainsAny(text, " \t") {
		return []part{{text: text, kind: "literal"}}
	}
	if isQuoted(strings.TrimSpace(strings.SplitN(text, " ", 2)[0])) && r.dialect == DialectPowerShell && len(words) == 1 {
		return []part{{text: text, kind: "literal"}}
	}
	// Wrappers run what follows them.
	for len(words) > 1 {
		opts, ok := wrappers[strings.ToLower(words[0])]
		if !ok {
			break
		}
		rest := words[1:]
		for len(rest) > 1 && (strings.HasPrefix(rest[0], "-") || bashAssign.MatchString(rest[0]) || literalWord.MatchString(rest[0]) && strings.ToLower(words[0]) == "timeout") {
			if opts[rest[0]] {
				rest = rest[1:]
			}
			rest = rest[1:]
		}
		words = rest
	}
	// A program held in a variable; in PowerShell a statement that starts
	// with a variable is an expression ($_ -match 'x'), not a program.
	if strings.HasPrefix(words[0], "$") {
		if v, ok := r.vars[strings.ToLower(strings.Trim(words[0], "{}"))]; ok && v != "" && !strings.ContainsAny(v, " ") {
			words[0] = v
		} else if r.dialect == DialectPowerShell {
			return []part{{text: text, kind: "literal"}}
		}
	}
	for i := range words {
		words[i] = r.expand(words[i])
	}
	p.words = normalise(words)
	p.inline = p.inline && interpreters[p.words[0]] // a heredoc is code only for an interpreter
	p.cwd = r.cwd
	if cdCommands[p.words[0]] {
		if args := positional(p.words[1:]); len(args) > 0 && r.cwd != "" {
			dir := args[0]
			if !filepath.IsAbs(dir) && !drivePath.MatchString(dir) {
				dir = filepath.Join(r.cwd, dir)
			}
			r.cwd = filepath.Clean(dir)
		}
	}
	return []part{p}
}

func (r *reader) redirect(p *part, op, target string) {
	target = strings.Trim(target, `"'`)
	l := strings.ToLower(target)
	if target == "" || l == "$null" || l == "/dev/null" || l == "nul" || strings.HasPrefix(target, "&") {
		return
	}
	if op == "<" {
		p.reads = append(p.reads, r.expand(target))
	} else {
		p.writes = append(p.writes, r.expand(target))
	}
}

// expand replaces variables set earlier in the command line ($S/x, ${S}/x)
// and the temp folder's ($TEMP, $env:TEMP, %TEMP%).
func (r *reader) expand(w string) string {
	for _, v := range []string{"$env:TEMP", "$env:TMP", "${TEMP}", "${TMPDIR}", "$TEMP", "$TMPDIR", "$TMP", "%TEMP%", "%TMP%"} {
		if i := strings.Index(strings.ToLower(w), strings.ToLower(v)); i >= 0 {
			w = w[:i] + os.TempDir() + w[i+len(v):]
		}
	}
	if !strings.Contains(w, "$") {
		return w
	}
	for name, v := range r.vars {
		bare := strings.TrimPrefix(name, "$")
		for _, form := range []string{"${" + bare + "}", "$" + bare} {
			if i := strings.Index(strings.ToLower(w), form); i >= 0 {
				w = w[:i] + v + w[i+len(form):]
			}
		}
	}
	return w
}

func isQuoted(s string) bool {
	return len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'')
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if isQuoted(s) {
		return s[1 : len(s)-1]
	}
	return s
}

// normalise gives the program its plain name (no folder, no extension,
// lower case) and drops global options before a subcommand.
func normalise(words []string) []string {
	prog := filepath.Base(strings.ReplaceAll(words[0], `\`, "/"))
	prog = strings.ToLower(prog)
	for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1"} {
		prog = strings.TrimSuffix(prog, ext)
	}
	words = append([]string{prog}, words[1:]...)
	if opts, ok := globalOptions[prog]; ok {
		rest := words[1:]
		for len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
			if opts[rest[0]] && len(rest) > 1 {
				rest = rest[1:]
			}
			rest = rest[1:]
		}
		words = append([]string{prog}, rest...)
	}
	// Placeholders for nested code aren't arguments.
	out := words[:0:0]
	for _, w := range words {
		if w = strings.ReplaceAll(w, "$SUB", ""); w != "" && w != "$BLOCK" {
			out = append(out, w)
		}
	}
	return out
}
