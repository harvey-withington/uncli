// Command uncli-record records a CLI's stream fixtures again for another
// version, from the fixtures of the version pinned now (Claude Code here;
// Antigravity, with -cli antigravity, in agy.go): each
// fixture's arguments (<name>.args.txt) and what UNCLI wrote to it
// (<name>.in.jsonl) are replayed against the new CLI, and what it prints
// becomes the new <name>.jsonl. Moving to a new CLI version records with
// this, passes the parser tests, then bumps PinnedVersion (decision 0001;
// the conventions skill says when).
//
//	cd app && go run ./cmd/uncli-record [-cli antigravity] -version <v> [name ...]
//
// Turns go one at a time, each after the last one's result; interrupt and
// mcp_status go mid-turn, once the answer has started; other control
// requests go between turns. The CLI's own requests (can_use_tool…) get the
// recorded answers in order, for the request it actually made. It runs in
// a config folder of its own, signed in with the models-only token
// containers use, so nothing of the user's (MCP servers, connectors,
// skills, conversations) is in a recording or left behind; the turns are
// real but small (Haiku). Paths and the account are scrubbed as the
// fixtures' README says. A fixture it can't set up (a WSL container, an
// MCP config that isn't in testdata) is copied over as it was, and listed.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/harvey-withington/uncli/app/internal/adapter/antigravity"
	"github.com/harvey-withington/uncli/app/internal/adapter/claude"
	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/secrets"
)

const streams = "testdata/streams/claude"

// The scrubbed names the fixtures use.
const (
	scrubWork = `C:\uncli-spike\work`
	scrubHome = `C:\Users\user`
	scrubMail = "user@example.com"
	zeroUUID  = "00000000-0000-0000-0000-000000000000"
)

func main() {
	cli := flag.String("cli", "claude", "the CLI to record: claude or antigravity")
	version := flag.String("version", "", "the CLI version to record (required)")
	from := flag.String("from", "", "the version whose fixtures to replay (default: the one pinned)")
	flag.Parse()
	record, pinned := run, claude.PinnedVersion
	if *cli == "antigravity" {
		record, pinned = runAntigravity, antigravity.PinnedVersion
	} else if *cli != "claude" {
		fmt.Fprintln(os.Stderr, "-cli is claude or antigravity")
		os.Exit(2)
	}
	if *from == "" {
		*from = pinned
	}
	if *version == "" || *version == *from {
		fmt.Fprintln(os.Stderr, "give -version, a version other than the one recorded already")
		os.Exit(2)
	}
	if err := record(*from, *version, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(from, version string, only []string) error {
	src, dst := filepath.Join(streams, from), filepath.Join(streams, version)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return err
	}
	ctx := context.Background()
	cli, err := claude.NewInstaller(filepath.Join(paths.Cache, "cli", "claude")).Ensure(ctx, version, nil)
	if err != nil {
		return fmt.Errorf("getting Claude Code %s: %w", version, err)
	}
	// A config folder of the run's own, signed in with the models-only token
	// containers use, so nothing of the user's (their MCP servers,
	// connectors, skills, commands, conversations) is in the recordings or
	// left behind.
	token, _ := secrets.Get("claude-oauth-token")
	if token == "" {
		return fmt.Errorf("record with the models-only token: sign in for containers first (Settings → AI Providers)")
	}
	cfg, err := os.MkdirTemp("", "uncli-record-config-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cfg)
	work, err := seedWork()
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	defer os.Remove(work + "-mcp.json") // mcpConfig's copy

	names, err := fixtures(src)
	if err != nil {
		return err
	}
	if len(only) > 0 {
		names = slices.DeleteFunc(names, func(n string) bool { return !slices.Contains(only, n) })
	}
	// Sessions a later fixture resumes, by the id the old recording had.
	sessions := map[string]string{}
	var carried, failed []string
	for _, name := range names {
		f, err := load(src, name)
		if err != nil {
			return err
		}
		f.config, f.token = cfg, token
		if why := f.unsupported(); why != "" {
			carried = append(carried, fmt.Sprintf("%s (%s)", name, why))
			if err := carry(src, dst, name); err != nil {
				return err
			}
			continue
		}
		fmt.Printf("%-34s ", name)
		start := time.Now()
		out, stderr, err := f.record(ctx, cli, work, sessions)
		if err != nil {
			fmt.Println("FAILED:", err)
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if id := sessionOf(f.old); id != "" {
			if nid := sessionOf(out); nid != "" {
				sessions[id] = nid
			}
		}
		if err := f.save(dst, out, stderr, work); err != nil {
			return err
		}
		fmt.Printf("%d lines, %s\n", len(out), time.Since(start).Round(time.Second))
	}
	if len(only) == 0 || slices.Contains(only, "transcript") {
		fmt.Printf("%-34s ", "transcript")
		if err := recordTranscript(ctx, cli, from, version, work, cfg, token); err != nil {
			fmt.Println("FAILED:", err)
			failed = append(failed, "transcript: "+err.Error())
		} else {
			fmt.Println("ok")
		}
	}
	// Files that go with the fixtures but aren't streams.
	for _, extra := range []string{"text-task-json-schema.json"} {
		if b, err := os.ReadFile(filepath.Join(src, extra)); err == nil {
			_ = os.WriteFile(filepath.Join(dst, extra), b, 0o644)
		}
	}
	if len(carried) > 0 {
		fmt.Println("\ncopied as they were (record these by hand):")
		for _, c := range carried {
			fmt.Println("  " + c)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("\n%d failed:\n  %s", len(failed), strings.Join(failed, "\n  "))
	}
	return nil
}

var notWord = regexp.MustCompile(`[^A-Za-z0-9]`)

// replaceEscaped replaces from with to as it is, and JSON-escaped once and
// twice (a string inside a string, as a tool's input quoted in an error).
func replaceEscaped(s, from, to string) string {
	quote := func(v string) string { b, _ := json.Marshal(v); return strings.Trim(string(b), `"`) }
	return strings.NewReplacer(quote(quote(from)), quote(quote(to)), quote(from), quote(to), from, to).Replace(s)
}

// projectKey is the folder in a config folder's projects where the CLI
// keeps a work folder's conversations.
func projectKey(folder string) string { return notWord.ReplaceAllString(folder, "-") }

// The transcript fixture (testdata/transcripts/claude/<version>): a
// conversation as the CLI saves it, for importing (four turns: a write, a
// question, /compact, one more), under the same session id as before.
const transcriptID = "939053cc-a279-4983-bad0-d37f5fab3c0d"

var transcriptTurns = []string{
	"Use the Write tool to create hello.txt containing the word hi. Then reply DONE.",
	"In one short line: what file did you write?",
	"/compact",
	"Reply with just the word AFTER.",
}

func recordTranscript(ctx context.Context, cli, from, version, work, cfg, token string) error {
	dir := filepath.Join(work, "w-import")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f := &fixture{name: "transcript", dir: filepath.Join(streams, from), env: map[string]string{}, config: cfg, token: token,
		args: []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--model", "haiku",
			"--tools", "Write", "--allowedTools", "Write", "--session-id", transcriptID}}
	for _, q := range transcriptTurns {
		b, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": q}, "parent_tool_use_id": nil, "session_id": ""})
		f.in = append(f.in, string(b))
	}
	if _, _, err := f.record(ctx, cli, dir, map[string]string{}); err != nil {
		return err
	}
	saved, err := os.ReadFile(filepath.Join(cfg, "projects", projectKey(dir), transcriptID+".jsonl"))
	if err != nil {
		return fmt.Errorf("the CLI didn't save the conversation: %w", err)
	}
	const scrubbed = `C:\uncli-spike\w-import`
	home, _ := os.UserHomeDir()
	s := string(saved)
	for _, p := range []struct{ from, to string }{
		{projectKey(dir), projectKey(scrubbed)}, {dir, scrubbed}, {cfg, scrubHome + `\.claude`}, {home, scrubHome},
	} {
		s = replaceEscaped(s, p.from, p.to)
	}
	s = emailRE.ReplaceAllString(s, scrubMail)
	out := filepath.Join("testdata", "transcripts", "claude", version, "projects", projectKey(scrubbed))
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, transcriptID+".jsonl"), []byte(s), 0o644)
}

// seedWork is the folder the CLI runs in: a git repository with the
// notes.txt the edit and plan fixtures change.
func seedWork() (string, error) {
	dir, err := os.MkdirTemp("", "uncli-record-")
	if err != nil {
		return "", err
	}
	dir, _ = filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"), 0o644); err != nil {
		return "", err
	}
	_ = exec.Command("git", "-C", dir, "init", "-q").Run()
	return dir, nil
}

func fixtures(dir string) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(dir, "*.args.txt"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range m {
		out = append(out, strings.TrimSuffix(filepath.Base(p), ".args.txt"))
	}
	// By name, which records each session a later fixture resumes first
	// (multi-turn-partial before resume-partial, append-system-prompt before
	// system-prompt-snapshot-resume).
	slices.Sort(out)
	return out, nil
}

type fixture struct {
	name   string
	dir    string
	args   []string
	env    map[string]string
	in     []string
	old    []string
	stderr bool   // the old one kept its stderr
	config string // the CLAUDE_CONFIG_DIR it records in: the run's, or its own (ownConfig)
	token  string // the models-only token it signs in with
}

func load(dir, name string) (*fixture, error) {
	f := &fixture{name: name, dir: dir, env: map[string]string{}}
	raw, err := os.ReadFile(filepath.Join(dir, name+".args.txt"))
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(strings.ReplaceAll(string(raw), "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case i == 0:
			f.args = splitArgs(line)
		case strings.HasPrefix(line, "env: "):
			kv, _, _ := strings.Cut(strings.TrimPrefix(line, "env: "), " ")
			k, v, _ := strings.Cut(kv, "=")
			f.env[k] = v
		}
	}
	f.in, _ = lines(filepath.Join(dir, name+".in.jsonl"))
	f.old, _ = lines(filepath.Join(dir, name+".jsonl"))
	_, err = os.Stat(filepath.Join(dir, name+".stderr.txt"))
	f.stderr = err == nil
	return f, nil
}

// splitArgs reads an args.txt line: each flag's value runs to the next
// flag, so a prompt with spaces stays one argument; "" (or nothing, after
// a flag that always takes a value) is an empty one.
func splitArgs(line string) []string {
	var out []string
	var val []string
	flush := func() {
		if len(val) == 0 && len(out) > 0 && slices.Contains(valued, out[len(out)-1]) {
			out = append(out, "")
		}
		if len(val) > 0 {
			v := strings.Join(val, " ")
			if v == `""` {
				v = ""
			}
			out = append(out, v)
			val = nil
		}
	}
	for _, w := range strings.Fields(line) {
		if strings.HasPrefix(w, "-") {
			flush()
			out = append(out, w)
		} else {
			val = append(val, w)
		}
	}
	flush()
	return out
}

// valued are flags that always take a value, even an empty one.
var valued = []string{"--setting-sources", "--tools"}

func lines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

// unsupported says why a fixture can't be recorded here, or "".
func (f *fixture) unsupported() string {
	if strings.HasPrefix(f.name, "wsl-") {
		return "runs in a WSL container"
	}
	for i, a := range f.args {
		if a == "--mcp-config" && i+1 < len(f.args) {
			if _, err := os.Stat(filepath.Join(f.dir, f.args[i+1])); err != nil {
				return "its MCP config " + f.args[i+1] + " isn't in testdata"
			}
		}
	}
	return ""
}

func carry(src, dst, name string) error {
	m, _ := filepath.Glob(filepath.Join(src, name+".*"))
	for _, p := range m {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, filepath.Base(p)), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// msg is the little of a line the replay looks at.
type msg struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
	Response  json.RawMessage `json:"response"`
	SessionID string          `json:"session_id"`
}

func parse(line string) msg {
	var m msg
	_ = json.Unmarshal([]byte(line), &m)
	return m
}

// stream collects what the CLI prints, for the replay to wait on.
type stream struct {
	mu    sync.Mutex
	lines []string
	done  bool
}

func (s *stream) snapshot() ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.lines), s.done
}

// waitFor waits until ok holds for what's been printed, or the CLI ends.
func (s *stream) waitFor(limit time.Duration, ok func([]string) bool) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		l, done := s.snapshot()
		if ok(l) {
			return nil
		}
		if done {
			return fmt.Errorf("the CLI ended first")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out")
}

func count(ls []string, typ string) int {
	n := 0
	for _, l := range ls {
		if parse(l).Type == typ {
			n++
		}
	}
	return n
}

func (f *fixture) record(ctx context.Context, cli, work string, sessions map[string]string) ([]string, string, error) {
	args := slices.Clone(f.args)
	for i, a := range args {
		if i > 0 && args[i-1] == "--mcp-config" {
			p, err := mcpConfig(filepath.Join(f.dir, a), work)
			if err != nil {
				return nil, "", err
			}
			args[i] = p
		}
		if i > 0 && args[i-1] == "--resume" {
			if nid, ok := sessions[a]; ok {
				args[i] = nid
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	done, err := f.ownConfig()
	if err != nil {
		return nil, "", err
	}
	defer done()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Dir = work
	cmd.Env = cliEnv(f)
	if strings.Contains(strings.Join(args, " "), "mcp-oauth.json") {
		stop, err := startOAuthServer(f.dir)
		if err != nil {
			return nil, "", err
		}
		defer stop()
	}
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, "", err
	}
	s := &stream{}
	// Recorded answers to the CLI's own requests, in order.
	var answers []string
	for _, l := range f.in {
		if parse(l).Type == "control_response" {
			answers = append(answers, l)
		}
	}
	var wmu sync.Mutex
	write := func(l string) {
		wmu.Lock()
		defer wmu.Unlock()
		_, _ = io.WriteString(stdin, l+"\n")
	}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			l := sc.Text()
			s.mu.Lock()
			s.lines = append(s.lines, l)
			s.mu.Unlock()
			if m := parse(l); m.Type == "control_request" {
				var a string
				s.mu.Lock()
				if len(answers) > 0 {
					a, answers = answers[0], answers[1:]
				}
				s.mu.Unlock()
				write(answer(a, m))
			}
		}
		s.mu.Lock()
		s.done = true
		s.mu.Unlock()
	}()

	const turn = 5 * time.Minute
	turns := 0
	results := func(n int) func([]string) bool { return func(ls []string) bool { return count(ls, "result") >= n } }
	var failure error
	for _, l := range f.in {
		m := parse(l)
		switch {
		case m.Type == "user":
			if err := s.waitFor(turn, results(turns)); err != nil {
				failure = fmt.Errorf("waiting for turn %d: %w", turns, err)
			}
			turns++
			write(l)
		case m.Type == "control_request":
			var r struct {
				Subtype string `json:"subtype"`
			}
			_ = json.Unmarshal(m.Request, &r)
			if r.Subtype == "interrupt" || r.Subtype == "mcp_status" {
				// Mid-turn: once the answer has started (an interrupt, once
				// some of its text has streamed, so there's something to
				// carry on from).
				n := len(firstLines(s))
				_ = s.waitFor(turn, func(ls []string) bool {
					texts := 0
					for _, x := range ls[min(n, len(ls)):] {
						t := parse(x).Type
						if r.Subtype == "interrupt" && t == "stream_event" && strings.Contains(x, `"text_delta"`) {
							texts++
						}
						if r.Subtype != "interrupt" && (t == "assistant" || t == "stream_event") {
							return true
						}
					}
					return texts >= 40 || count(ls, "result") >= turns
				})
				write(l)
				continue
			}
			if err := s.waitFor(turn, results(turns)); err != nil {
				failure = fmt.Errorf("before %s: %w", r.Subtype, err)
			}
			if r.Subtype == "mcp_oauth_callback_url" {
				l = withCallback(l, firstLines(s))
			}
			write(l)
			id := m.RequestID
			_ = s.waitFor(time.Minute, func(ls []string) bool {
				for _, x := range ls {
					if mm := parse(x); mm.Type == "control_response" && strings.Contains(string(mm.Response), id) {
						return true
					}
				}
				return false
			})
			// A sign-in through the CLI's own listener: open its link, as a
			// browser would (the test server approves and redirects).
			if r.Subtype == "mcp_authenticate" && !slices.ContainsFunc(f.in, func(x string) bool { return strings.Contains(x, "mcp_oauth_callback_url") }) {
				if err := openAuth(firstLines(s)); err != nil {
					failure = err
				}
			}
		}
		if failure != nil {
			break
		}
	}
	// As many results as the old recording had (a background agent's turn
	// counts), then the CLI is let go.
	want := max(count(f.old, "result"), turns)
	if failure == nil {
		if err := s.waitFor(turn, results(want)); err != nil {
			failure = fmt.Errorf("waiting for %d results: %w", want, err)
		}
	}
	_ = stdin.Close()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(time.Minute):
		_ = cmd.Process.Kill()
		<-exited
	}
	out, _ := s.snapshot()
	if failure != nil {
		if last := strings.TrimSpace(stderr.String()); last != "" {
			failure = fmt.Errorf("%w; stderr: %s", failure, last[max(0, len(last)-400):])
		}
		return out, stderr.String(), failure
	}
	return out, stderr.String(), nil
}

func firstLines(s *stream) []string { l, _ := s.snapshot(); return l }

var authURLRE = regexp.MustCompile(`"authUrl":"([^"]+)"`)

// openAuth follows the last sign-in link the CLI gave, through the test
// server's redirect to the CLI's listener, and gives the CLI a moment to
// finish signing in.
func openAuth(out []string) error {
	var link string
	for _, x := range out {
		if m := authURLRE.FindStringSubmatch(x); m != nil {
			link, _ = strconv.Unquote(`"` + m[1] + `"`)
		}
	}
	if link == "" {
		return fmt.Errorf("the CLI gave no sign-in link")
	}
	resp, err := http.Get(link)
	if err != nil {
		return fmt.Errorf("opening the sign-in link: %w", err)
	}
	_ = resp.Body.Close()
	time.Sleep(2 * time.Second)
	return nil
}

// withCallback points a recorded OAuth callback at this run's sign-in: the
// redirect and state of the last authUrl the CLI gave, with the test
// server's code (mcp-oauth-server.mjs approves every sign-in). The old
// state belongs to the old run's sign-in, which the CLI rightly refuses.
func withCallback(l string, out []string) string {
	var auth *url.URL
	for _, x := range out {
		if m := authURLRE.FindStringSubmatch(x); m != nil {
			if raw, err := strconv.Unquote(`"` + m[1] + `"`); err == nil {
				auth, _ = url.Parse(raw)
			}
		}
	}
	if auth == nil {
		return l
	}
	q := auth.Query()
	cb, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		return l
	}
	var a map[string]any
	if json.Unmarshal([]byte(l), &a) != nil {
		return l
	}
	req, _ := a["request"].(map[string]any)
	old, _ := req["callbackUrl"].(string)
	code := "code-abc"
	if u, err := url.Parse(old); err == nil && u.Query().Get("code") != "" {
		code = u.Query().Get("code")
	}
	cb.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	req["callbackUrl"] = cb.String()
	b, _ := json.Marshal(a)
	return string(b)
}

// answer is a recorded answer to the CLI's request m: the request's own
// id, and for an allow, the input it asked about.
func answer(recorded string, m msg) string {
	if recorded == "" {
		recorded = `{"type":"control_response","response":{"subtype":"success","request_id":"","response":{"behavior":"deny","message":"not in the recording"}}}`
	}
	var a map[string]any
	if json.Unmarshal([]byte(recorded), &a) != nil {
		return recorded
	}
	resp, _ := a["response"].(map[string]any)
	if resp == nil {
		return recorded
	}
	resp["request_id"] = m.RequestID
	if inner, _ := resp["response"].(map[string]any); inner != nil {
		if _, ok := inner["updatedInput"]; ok {
			var req struct {
				Input json.RawMessage `json:"input"`
			}
			if json.Unmarshal(m.Request, &req) == nil && len(req.Input) > 0 {
				inner["updatedInput"] = req.Input
			}
		}
	}
	b, _ := json.Marshal(a)
	return string(b)
}

// cliEnv is this process's environment without any sign-in or config of
// the user's, or what a CLI running UNCLI (or this tool) leaves behind; then
// the fixture's config folder and token, and its own environment.
func cliEnv(f *fixture) []string {
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		ku := strings.ToUpper(k)
		if strings.HasPrefix(ku, "CLAUDE_") || ku == "CLAUDECODE" || ku == "ANTHROPIC_API_KEY" {
			continue
		}
		if _, mine := f.env[k]; mine {
			continue
		}
		out = append(out, kv)
	}
	// Nor the user's git name, which the CLI puts in its context.
	out = append(out, "CLAUDE_CONFIG_DIR="+f.config, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if _, own := f.env["CLAUDE_CODE_OAUTH_TOKEN"]; !own && f.name != "error-not-logged-in" {
		out = append(out, "CLAUDE_CODE_OAUTH_TOKEN="+f.token)
	}
	for k, v := range f.env {
		out = append(out, k+"="+v)
	}
	return out
}

// ownConfig gives the fixtures that sign in or out a config folder of
// their own: signed out, and the MCP sign-ins, whose tokens stay with
// that folder.
func (f *fixture) ownConfig() (func(), error) {
	if f.name != "error-not-logged-in" && !strings.HasPrefix(f.name, "mcp-auth-") {
		return func() {}, nil
	}
	dir, err := os.MkdirTemp("", "uncli-record-config-")
	if err != nil {
		return nil, err
	}
	f.config = dir // kept, for save to scrub
	return func() { _ = os.RemoveAll(dir) }, nil
}

// mcpConfig is an MCP config from testdata as the CLI gets it: {dir} is
// the folder the config is in (where its servers' scripts are). The copy
// goes beside the work folder, so the recording doesn't show it.
func mcpConfig(path, work string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(b), "{dir}") {
		return abs, nil
	}
	out := work + "-" + filepath.Base(abs)
	return out, os.WriteFile(out, []byte(strings.ReplaceAll(string(b), "{dir}", filepath.ToSlash(filepath.Dir(abs)))), 0o644)
}

func startOAuthServer(dir string) (func(), error) {
	cmd := exec.Command("node", filepath.Join(dir, "..", "mcp-oauth-server.mjs"), "47011")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting the OAuth MCP server: %w", err)
	}
	time.Sleep(time.Second)
	return func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }, nil
}

func sessionOf(ls []string) string {
	for _, l := range ls {
		if m := parse(l); m.Type == "system" && m.Subtype == "init" && m.SessionID != "" {
			return m.SessionID
		}
	}
	return ""
}

var (
	emailRE   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	accountRE = regexp.MustCompile(`"(organizationUuid|accountUuid|organizationId|accountId|orgId)":"[0-9a-fA-F-]{36}"`)
)

// save writes the new recording, scrubbed, with its args and input as they
// were (and its stderr, if the old one kept that).
func (f *fixture) save(dst string, out []string, stderr, work string) error {
	home, _ := os.UserHomeDir()
	scripts, _ := filepath.Abs(filepath.Join(f.dir, "..")) // the MCP servers' scripts
	scrub := func(s string) string {
		for _, p := range []struct{ from, to string }{
			{projectKey(work), projectKey(scrubWork)}, // the work folder's conversations, by the CLI's name for it
			{work, scrubWork}, {scripts, `C:\uncli-spike`}, {f.config, scrubHome + `\.claude`}, {home, scrubHome},
		} {
			if p.from == "" {
				continue
			}
			s = replaceEscaped(s, p.from, p.to)
			s = strings.ReplaceAll(s, filepath.ToSlash(p.from), filepath.ToSlash(p.to))
		}
		s = accountRE.ReplaceAllString(s, `"$1":"`+zeroUUID+`"`)
		return emailRE.ReplaceAllStringFunc(s, func(m string) string {
			if strings.HasSuffix(m, "@example.com") || strings.HasSuffix(m, "@anthropic.com") {
				return m
			}
			return scrubMail
		})
	}
	var b strings.Builder
	for _, l := range out {
		b.WriteString(scrub(l) + "\n")
	}
	if err := os.WriteFile(filepath.Join(dst, f.name+".jsonl"), []byte(b.String()), 0o644); err != nil {
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
