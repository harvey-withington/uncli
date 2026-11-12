package main

// The Antigravity CLI's fixtures are recorded again the same way: each
// fixture's arguments and turns are replayed against the new agy, each
// turn sent after the last one's result. agy runs in a folder holding the
// note.txt the tool fixtures read, with a private state folder
// (--gemini_dir) and the machine's Google sign-in (agy keeps it in the OS
// keyring). Fixtures run with --dangerously-skip-permissions get UNCLI's
// own hooks.json, whose command is this program in hook mode: it records
// each tool call to <name>.hook.jsonl and answers as UNCLI would, allowing
// all but the tools the fixture denies (decision 0012).

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/harvey-withington/uncli/app/internal/adapter/antigravity"
	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/core"
)

const agyStreams = "testdata/streams/antigravity"

// The scrubbed folders the Antigravity fixtures use.
const (
	agyScrubWork  = `C:\uncli-spike`
	agyScrubState = `C:\uncli-agy`
)

// Hook mode: where to record the calls, and the tools to deny.
const (
	agyHookLogEnv  = "UNCLI_RECORD_HOOK_LOG"
	agyHookDenyEnv = "UNCLI_RECORD_HOOK_DENY"
	agyDenyReason  = "UNCLI test: deny"
)

// agyDenies are the tools each fixture's hook denies.
var agyDenies = map[string][]string{"tools-hook-deny": {"run_command"}}

// When agy runs this program as its hook, it answers and exits before
// main.
func init() {
	if log := os.Getenv(agyHookLogEnv); log != "" && len(os.Args) == 2 {
		os.Exit(agyHook(os.Args[1], log, strings.Split(os.Getenv(agyHookDenyEnv), ","), os.Stdin, os.Stdout))
	}
}

// agyHook reads one hook call, records a tool call to log and prints
// UNCLI's answer to it. Anything it can't read is denied, as UNCLI does.
func agyHook(event, log string, deny []string, in io.Reader, out io.Writer) int {
	a := &antigravity.Adapter{}
	raw, err := io.ReadAll(in)
	if err != nil {
		return 1
	}
	call, err := a.ReadHook(event, raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ans := core.HookAnswer{Allow: true}
	if call.Kind == core.HookTool {
		var line bytes.Buffer
		if err := json.Compact(&line, raw); err != nil {
			return 1
		}
		f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 1
		}
		_, err = f.WriteString(line.String() + "\n")
		if cerr := f.Close(); err != nil || cerr != nil {
			return 1
		}
		if slices.Contains(deny, call.Approval.Tool) {
			ans = core.HookAnswer{Reason: agyDenyReason}
		}
	}
	b, err := a.AnswerHook(call, ans)
	if err != nil {
		return 1
	}
	_, _ = out.Write(b)
	return 0
}

// runAntigravity records the fixtures of version from again for version,
// into testdata/streams/antigravity/<version> (only the named ones, if
// any are named).
func runAntigravity(from, version string, only []string) error {
	src, dst := filepath.Join(agyStreams, from), filepath.Join(agyStreams, version)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return err
	}
	ctx := context.Background()
	cli, err := antigravity.NewInstaller(filepath.Join(paths.Cache, "cli", "antigravity")).Ensure(ctx, version, nil)
	if err != nil {
		return fmt.Errorf("getting the Antigravity CLI %s: %w", version, err)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	work, err := agyTemp("uncli-record-agy-work-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := os.WriteFile(filepath.Join(work, "note.txt"), []byte("The colour is teal.\n"), 0o644); err != nil {
		return err
	}
	state, err := agyTemp("uncli-record-agy-state-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(state)
	hookLog := filepath.Join(state, "hook.jsonl")
	// The hook command line, as UNCLI's own would be: quoted, then the event.
	adapter := antigravity.New(nil, state, `"`+self+`"`)

	names, err := fixtures(src)
	if err != nil {
		return err
	}
	if len(only) > 0 {
		names = slices.DeleteFunc(names, func(n string) bool { return !slices.Contains(only, n) })
	}
	// Conversations a later fixture resumes, by the id the old recording had.
	convs := map[string]string{}
	var failed []string
	for _, name := range names {
		f, err := load(src, name)
		if err != nil {
			return err
		}
		fmt.Printf("%-20s ", name)
		start := time.Now()
		hooked := slices.Contains(f.args, "--dangerously-skip-permissions")
		cmd, err := adapter.BuildCommand(cli, core.LaunchSpec{Approvals: hooked})
		if err != nil {
			return err
		}
		if !hooked {
			_ = os.Remove(filepath.Join(state, "config", "hooks.json"))
		}
		_ = os.Remove(hookLog)
		env := cmd.Env
		if hooked {
			env[agyHookLogEnv] = hookLog
			env[agyHookDenyEnv] = strings.Join(agyDenies[name], ",")
		}
		out, stderr, err := agyRecord(ctx, cli, agyArgs(f.args, state, convs), env, work, f)
		if err != nil {
			fmt.Println("FAILED:", err)
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if id := agyConversation(f.old); id != "" {
			if nid := agyConversation(out); nid != "" {
				convs[id] = nid
			}
		}
		var calls []string
		if hooked {
			calls, _ = lines(hookLog)
		}
		if err := agySave(f, dst, out, calls, stderr, work, state); err != nil {
			return err
		}
		fmt.Printf("%d lines, %d hook calls, %s\n", len(out), len(calls), time.Since(start).Round(time.Second))
	}
	if len(failed) > 0 {
		return fmt.Errorf("\n%d failed:\n  %s", len(failed), strings.Join(failed, "\n  "))
	}
	return nil
}

// agyTemp is a new temporary folder by its long name, as agy reports it.
func agyTemp(prefix string) (string, error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	if long, err := filepath.EvalSymlinks(dir); err == nil {
		dir = long
	}
	return dir, nil
}

// agyArgs are the recorded arguments with the state folder this run's and
// a resumed conversation the one recorded now.
func agyArgs(recorded []string, state string, convs map[string]string) []string {
	args := slices.Clone(recorded)
	for i, a := range args {
		if strings.HasPrefix(a, "--gemini_dir=") {
			args[i] = "--gemini_dir=" + state
		}
		if i > 0 && args[i-1] == "--conversation" {
			if nid, ok := convs[a]; ok {
				args[i] = nid
			}
		}
	}
	return args
}

// agyLine is the little of a line the replay looks at.
type agyLine struct {
	Event          string `json:"event"`
	ConversationID string `json:"conversation_id"`
}

func agyParse(l string) agyLine {
	var m agyLine
	_ = json.Unmarshal([]byte(l), &m)
	return m
}

func agyResults(ls []string) int {
	n := 0
	for _, l := range ls {
		if agyParse(l).Event == "result" {
			n++
		}
	}
	return n
}

func agyConversation(ls []string) string {
	for _, l := range ls {
		if m := agyParse(l); m.Event == "init" && m.ConversationID != "" {
			return m.ConversationID
		}
	}
	return ""
}

// agyRecord runs agy once: each turn after the last one's result, then as
// many results as the old recording had, then stdin closes and it ends.
func agyRecord(ctx context.Context, cli string, args []string, env map[string]string, work string, f *fixture) ([]string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Dir = work
	cmd.Env = agyEnv(env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, "", err
	}
	s := &stream{}
	var read sync.WaitGroup
	read.Add(1)
	go func() {
		defer read.Done()
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			s.mu.Lock()
			s.lines = append(s.lines, sc.Text())
			s.mu.Unlock()
		}
		s.mu.Lock()
		s.done = true
		s.mu.Unlock()
	}()

	const turn = 5 * time.Minute
	results := func(n int) func([]string) bool { return func(ls []string) bool { return agyResults(ls) >= n } }
	var failure error
	turns := 0
	for _, l := range f.in {
		if err := s.waitFor(turn, results(turns)); err != nil {
			failure = fmt.Errorf("waiting for turn %d: %w", turns, err)
			break
		}
		turns++
		_, _ = io.WriteString(stdin, l+"\n")
	}
	if want := max(agyResults(f.old), turns); failure == nil {
		if err := s.waitFor(turn, results(want)); err != nil {
			failure = fmt.Errorf("waiting for %d results: %w", want, err)
		}
	}
	_ = stdin.Close()
	exited := make(chan struct{})
	go func() { read.Wait(); _ = cmd.Wait(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(time.Minute):
		_ = cmd.Process.Kill()
		<-exited
	}
	out, _ := s.snapshot()
	return out, stderr.String(), failure
}

// agyEnv is this process's environment without what UNCLI (if this runs
// under it) leaves behind, plus the CLI's own.
func agyEnv(extra map[string]string) []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, mine := extra[k]; mine || strings.HasPrefix(strings.ToUpper(k), "UNCLI_") {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

// agySwap replaces one way of writing a path with the fixtures' name for it.
type agySwap struct {
	re *regexp.Regexp
	to string
}

// agySwaps replace this run's folders and the home folder with the
// fixtures' names, however a line writes the path: as is, escaped in JSON,
// with forward slashes, in either case.
func agySwaps(work, state string) []agySwap {
	home, _ := os.UserHomeDir()
	var swaps []agySwap
	for _, p := range []struct{ from, to string }{{work, agyScrubWork}, {state, agyScrubState}, {home, scrubHome}} {
		if p.from == "" {
			continue
		}
		esc, _ := json.Marshal(p.from)
		escTo, _ := json.Marshal(p.to)
		for _, v := range []struct{ from, to string }{
			{strings.Trim(string(esc), `"`), strings.Trim(string(escTo), `"`)},
			{p.from, p.to},
			{filepath.ToSlash(p.from), filepath.ToSlash(p.to)},
		} {
			swaps = append(swaps, agySwap{regexp.MustCompile(`(?i)` + regexp.QuoteMeta(v.from)), v.to})
		}
	}
	return swaps
}

func agyScrub(swaps []agySwap, s string) string {
	for _, sw := range swaps {
		s = sw.re.ReplaceAllLiteralString(s, sw.to)
	}
	return emailRE.ReplaceAllStringFunc(s, func(m string) string {
		if strings.HasSuffix(m, "@example.com") {
			return m
		}
		return scrubMail
	})
}

// agyScrubDeltas scrubs each answer step's text as a whole, since the
// model can split a path across text_delta lines; each line keeps its
// share of the text.
func agyScrubDeltas(swaps []agySwap, out []string) []string {
	type delta struct {
		Step *struct {
			Conv  string `json:"conversation_id"`
			Index int    `json:"step_index"`
			Text  string `json:"text_delta"`
		} `json:"step_update"`
	}
	runs := map[string][]int{}
	var order []string
	texts := make([]string, len(out))
	for i, l := range out {
		var d delta
		if json.Unmarshal([]byte(l), &d) != nil || d.Step == nil || d.Step.Text == "" {
			continue
		}
		key := fmt.Sprintf("%s:%d", d.Step.Conv, d.Step.Index)
		if runs[key] == nil {
			order = append(order, key)
		}
		runs[key] = append(runs[key], i)
		texts[i] = d.Step.Text
	}
	out = slices.Clone(out)
	for _, key := range order {
		idx := runs[key]
		chunks := make([]string, len(idx))
		for j, i := range idx {
			chunks[j] = texts[i]
		}
		for j, c := range agySplitScrub(swaps, chunks) {
			if i := idx[j]; c != texts[i] {
				out[i] = agyReplaceString(out[i], texts[i], c)
			}
		}
	}
	return out
}

// agySplitScrub scrubs the chunks' joined text and splits it again where
// the chunks ended; an end inside a replaced path moves in proportion.
func agySplitScrub(swaps []agySwap, chunks []string) []string {
	for _, sw := range swaps {
		joined := strings.Join(chunks, "")
		ms := sw.re.FindAllStringIndex(joined, -1)
		if len(ms) == 0 {
			continue
		}
		moved := func(x int) int {
			shift := 0
			for _, m := range ms {
				if x <= m[0] {
					break
				}
				if x < m[1] {
					return m[0] + shift + len(sw.to)*(x-m[0])/(m[1]-m[0])
				}
				shift += len(sw.to) - (m[1] - m[0])
			}
			return x + shift
		}
		scrubbed := sw.re.ReplaceAllLiteralString(joined, sw.to)
		next := make([]string, len(chunks))
		end, from := 0, 0
		for j, c := range chunks {
			end += len(c)
			to := moved(end)
			if j == len(chunks)-1 {
				to = len(scrubbed)
			}
			next[j] = scrubbed[from:to]
			from = to
		}
		// A chunk that had text keeps some: the CLI sends no empty deltas.
		for j := range next {
			if next[j] == "" && chunks[j] != "" && j+1 < len(next) {
				if _, n := utf8.DecodeRuneInString(next[j+1]); n < len(next[j+1]) {
					next[j], next[j+1] = next[j+1][:n], next[j+1][n:]
				}
			}
		}
		chunks = next
	}
	return chunks
}

// agyReplaceString swaps a JSON string value in a line, encoded as the CLI
// encodes it (with or without HTML escaping); a line where it isn't found
// once is left as it was.
func agyReplaceString(line, from, to string) string {
	for _, html := range []bool{true, false} {
		enc := func(s string) string {
			var b bytes.Buffer
			e := json.NewEncoder(&b)
			e.SetEscapeHTML(html)
			_ = e.Encode(s)
			return strings.TrimSuffix(b.String(), "\n")
		}
		if old := enc(from); strings.Count(line, old) == 1 {
			return strings.Replace(line, old, enc(to), 1)
		}
	}
	return line
}

// agySave writes the new recording and its hook calls, scrubbed, with its
// args and turns as they were (and its stderr, if the old one kept that).
func agySave(f *fixture, dst string, out, calls []string, stderr, work, state string) error {
	swaps := agySwaps(work, state)
	scrub := func(s string) string { return agyScrub(swaps, s) }
	out = agyScrubDeltas(swaps, out)
	write := func(ext string, ls []string) error {
		var b strings.Builder
		for _, l := range ls {
			b.WriteString(scrub(l) + "\n")
		}
		return os.WriteFile(filepath.Join(dst, f.name+ext), []byte(b.String()), 0o644)
	}
	if err := write(".jsonl", out); err != nil {
		return err
	}
	if len(calls) > 0 {
		if err := write(".hook.jsonl", calls); err != nil {
			return err
		}
	} else if err := os.Remove(filepath.Join(dst, f.name+".hook.jsonl")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, ext := range []string{".args.txt", ".in.jsonl"} {
		if raw, err := os.ReadFile(filepath.Join(f.dir, f.name+ext)); err == nil {
			if err := os.WriteFile(filepath.Join(dst, f.name+ext), raw, 0o644); err != nil {
				return err
			}
		}
	}
	if f.stderr {
		return os.WriteFile(filepath.Join(dst, f.name+".stderr.txt"), []byte(scrub(stderr)), 0o644)
	}
	return nil
}
