package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"uncli/config"
	"uncli/internal/adapter/claude"
	"uncli/internal/core"
	"uncli/internal/profile"
	"uncli/internal/store"
)

const fixtures = "../../testdata/streams/claude/" + claude.PinnedVersion

// segments splits a recorded stream into one chunk per turn (ending at
// each result line).
func segments(t *testing.T, name string) [][]byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtures, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	var cur bytes.Buffer
	for l := range bytes.SplitSeq(bytes.TrimSpace(b), []byte("\n")) {
		cur.Write(l)
		cur.WriteByte('\n')
		if isResult(l) {
			out = append(out, bytes.Clone(cur.Bytes()))
			cur.Reset()
		}
	}
	return out
}

func isResult(l []byte) bool {
	var v struct{ Type string }
	_ = json.Unmarshal(l, &v)
	return v.Type == "result"
}

// fakeRuntime replays recorded turns: each user line written to stdin
// releases the next recorded turn on stdout.
type fakeRuntime struct {
	mu        sync.Mutex
	turns     [][]byte
	next      int
	starts    []core.Command
	stdin     []string
	procs     []*fakeProc
	hangLast  bool // keep the process alive after the last turn without replying
	onControl bool // an approval answer (control_response) also releases the next turn
	// fail: the next process prints this on stderr and exits 1 after its
	// first turn's output (a CLI that can't carry on).
	fail string
}

func (r *fakeRuntime) ID() string { return "local" }

func (r *fakeRuntime) Start(ctx context.Context, cmd core.Command, workdir string) (core.Proc, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, cmd)
	outR, outW := io.Pipe()
	p := &fakeProc{r: r, out: outR, outW: outW, done: make(chan struct{}), stderr: r.fail}
	r.fail = ""
	r.procs = append(r.procs, p)
	return p, nil
}

func (r *fakeRuntime) lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.stdin)
}

type fakeProc struct {
	r      *fakeRuntime
	out    *io.PipeReader
	outW   *io.PipeWriter
	once   sync.Once
	done   chan struct{}
	code   int
	stderr string
}

func (p *fakeProc) Write(b []byte) (int, error) {
	select {
	case <-p.done:
		return 0, errors.New("closed")
	default:
	}
	p.r.mu.Lock()
	p.r.stdin = append(p.r.stdin, strings.TrimSpace(string(b)))
	var seg []byte
	release := bytes.Contains(b, []byte(`"type":"user"`)) || p.r.onControl && bytes.Contains(b, []byte(`"type":"control_response"`))
	if release && p.r.next < len(p.r.turns) {
		seg = p.r.turns[p.r.next]
		p.r.next++
	}
	p.r.mu.Unlock()
	if seg != nil {
		go func() {
			_, _ = p.outW.Write(seg)
			if p.stderr != "" {
				p.exit(1)
			}
		}()
	}
	return len(b), nil
}

func (p *fakeProc) Stdin() io.Writer  { return p }
func (p *fakeProc) Stdout() io.Reader { return p.out }
func (p *fakeProc) Stderr() io.Reader { return strings.NewReader(p.stderr) }
func (p *fakeProc) Wait() error {
	<-p.done
	if p.code != 0 {
		return exitErr(p.code)
	}
	return nil
}
func (p *fakeProc) Kill() error { p.exit(0); return nil }
func (p *fakeProc) exit(code int) {
	p.once.Do(func() { p.code = code; close(p.done); p.outW.Close() })
}

type exitErr int

func (e exitErr) Error() string { return "exit" }
func (e exitErr) ExitCode() int { return int(e) }

type recSink struct {
	mu     sync.Mutex
	states []State
	events []core.Event
}

func (s *recSink) SessionEvent(_ string, ev core.Event) {
	s.mu.Lock()
	s.events = append(s.events, ev)
	s.mu.Unlock()
}
func (s *recSink) SessionChanged(v View) {
	s.mu.Lock()
	if n := len(s.states); n == 0 || s.states[n-1] != v.State {
		s.states = append(s.states, v.State)
	}
	s.mu.Unlock()
}
func (s *recSink) PageChanged(store.Page) {}

type harness struct {
	t    *testing.T
	m    *Manager
	rt   *fakeRuntime
	sink *recSink
	db   *store.Store
	path string
	dir  string
}

func newHarness(t *testing.T, fixture string) *harness {
	t.Helper()
	dir := t.TempDir()
	return openHarness(t, dir, fixture)
}

func openHarness(t *testing.T, dir, fixture string) *harness {
	t.Helper()
	path := filepath.Join(dir, "uncli.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	set, err := profile.Load(defaults, "")
	if err != nil {
		t.Fatal(err)
	}
	rt := &fakeRuntime{turns: segments(t, fixture)}
	sink := &recSink{}
	m, err := NewManager(Deps{
		Store: db, Adapter: claude.New(nil), Runtime: rt, Profiles: set, Sink: sink,
		ScratchDir: filepath.Join(dir, "scratch"),
		Binary:     func(context.Context) (string, string, error) { return "claude", claude.PinnedVersion, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return &harness{t: t, m: m, rt: rt, sink: sink, db: db, path: path, dir: dir}
}

func (h *harness) send(id, text string) []store.Page {
	h.t.Helper()
	if err := h.m.Send(context.Background(), id, text); err != nil {
		h.t.Fatal(err)
	}
	return h.waitIdle(id)
}

func (h *harness) waitIdle(id string) []store.Page {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := h.m.get(id)
		if !s.View().Busy {
			pages, _ := h.m.Pages(id)
			return pages
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.t.Fatal("turn never finished")
	return nil
}

func TestMultiTurnPages(t *testing.T) {
	h := newHarness(t, "multi-turn-partial")
	v, err := h.m.Create("chat", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Workdir, filepath.Join(h.dir, "scratch")) {
		t.Errorf("chat workdir = %s", v.Workdir)
	}
	h.m.Focus(v.ID)
	h.send(v.ID, "Remember the number 42. Reply with just OK.")
	h.send(v.ID, "Write a list and code")
	pages := h.send(v.ID, "What number did I give you?")

	if len(pages) != 3 {
		t.Fatalf("pages = %d", len(pages))
	}
	if pages[0].AnswerMD != "OK" || pages[2].AnswerMD != "Forty-two." || !strings.Contains(pages[1].AnswerMD, "```js") {
		t.Errorf("answers = %q / %q / %q", pages[0].AnswerMD, pages[1].AnswerMD, pages[2].AnswerMD)
	}
	total := 0.0
	for _, p := range pages {
		if p.Status != "done" || p.OutputTokens == 0 || p.CostUSD <= 0 || p.Model != "sonnet" {
			t.Errorf("page %d = %+v", p.Seq, p)
		}
		total += p.CostUSD
	}
	if total < 0.0294 || total > 0.0295 {
		t.Errorf("page costs should add up to the CLI's running total, got %v", total)
	}
	got, _ := h.m.get(v.ID)
	view := got.View()
	if view.ProviderSID != "df1bfdf0-962d-4fe2-bc28-eb1f3e0b22e2" || view.Title == "" || view.State != Idle {
		t.Errorf("view = %+v", view)
	}
	if len(h.rt.starts) != 1 {
		t.Errorf("one process should serve all turns, started %d", len(h.rt.starts))
	}
	args := strings.Join(h.rt.starts[0].Args, " ")
	if !strings.Contains(args, "--session-id "+v.ID) || !strings.Contains(args, "--permission-prompt-tool stdio") || strings.Contains(args, "--permission-prompts none") {
		t.Errorf("first spawn args = %s", args)
	}
	h.sink.mu.Lock()
	states := slices.Clone(h.sink.states)
	h.sink.mu.Unlock()
	for _, want := range []State{Starting, Thinking, Writing, Idle} {
		if !slices.Contains(states, want) {
			t.Errorf("state %s never reached: %v", want, states)
		}
	}
}

func TestUnreadWhenNotFocused(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	h.send(v.ID, "Say hi")
	s, _ := h.m.get(v.ID)
	if st := s.View().State; st != Unread {
		t.Errorf("state = %s, want unread", st)
	}
	h.m.Focus(v.ID)
	if st := s.View().State; st != Idle {
		t.Errorf("after focus = %s", st)
	}
}

func TestRestartResumes(t *testing.T) {
	dir := t.TempDir()
	h := openHarness(t, dir, "multi-turn-partial")
	v, _ := h.m.Create("chat", "", "")
	h.send(v.ID, "Remember 42")
	pages, _ := h.m.Pages(v.ID)
	h.m.SetBookmark(v.ID, pages[0].ID, true)
	h.m.Close()
	h.db.Close()

	h2 := openHarness(t, dir, "resume-partial")
	list := h2.m.List()
	if len(list) != 1 || list[0].ID != v.ID || list[0].ProviderSID == "" {
		t.Fatalf("restored = %+v", list)
	}
	pages = h2.send(v.ID, "What number?")
	if len(pages) != 2 || !pages[0].Bookmarked || pages[1].AnswerMD != "Forty-two." {
		t.Errorf("pages after restart = %+v", pages)
	}
	if args := strings.Join(h2.rt.starts[0].Args, " "); !strings.Contains(args, "--resume df1bfdf0-962d-4fe2-bc28-eb1f3e0b22e2") {
		t.Errorf("restart should resume: %s", args)
	}
}

func TestInterruptedTurn(t *testing.T) {
	h := newHarness(t, "interrupt-then-continue")
	v, _ := h.m.Create("chat", "", "haiku")
	pages := h.send(v.ID, "Write a story")
	if pages[0].Status != "interrupted" || !strings.Contains(pages[0].AnswerMD, "Last Light") {
		t.Errorf("page = %+v", pages[0])
	}
	pages = h.send(v.ID, "What were you writing?")
	if pages[1].Status != "done" {
		t.Errorf("next turn = %+v", pages[1])
	}
}

func TestInterruptSendsControlAndFallsBackToKill(t *testing.T) {
	h := newHarness(t, "single-turn")
	h.rt.turns = nil // the CLI never answers
	h.m.InterruptGrace = 50 * time.Millisecond
	v, _ := h.m.Create("chat", "", "haiku")
	if err := h.m.Send(context.Background(), v.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Interrupt(v.ID); err != nil {
		t.Fatal(err)
	}
	pages := h.waitIdle(v.ID)
	if pages[0].Status != "interrupted" {
		t.Errorf("status = %s", pages[0].Status)
	}
	if !slices.ContainsFunc(h.rt.lines(), func(l string) bool { return strings.Contains(l, `"subtype":"interrupt"`) }) {
		t.Error("interrupt control request not sent")
	}
	s, _ := h.m.get(v.ID)
	if s.View().Running {
		t.Error("process should have been killed after the grace period")
	}
}

func TestErrorTurn(t *testing.T) {
	h := newHarness(t, "error-not-logged-in")
	v, _ := h.m.Create("chat", "", "haiku")
	pages := h.send(v.ID, "Say hello.")
	if pages[0].Status != "error" || !strings.Contains(pages[0].Error, "authentication_failed") {
		t.Errorf("page = %+v", pages[0])
	}
	s, _ := h.m.get(v.ID)
	if s.View().State != Errored {
		t.Errorf("state = %s", s.View().State)
	}
}

func TestModelSwitchIsLive(t *testing.T) {
	h := newHarness(t, "set-model-live")
	v, _ := h.m.Create("chat", "", "haiku")
	h.send(v.ID, "Reply with just: one")
	if err := h.m.SetModel(v.ID, "sonnet"); err != nil {
		t.Fatal(err)
	}
	pages := h.send(v.ID, "Which model are you?")
	if pages[0].Model != "haiku" || pages[1].Model != "sonnet" || pages[1].AnswerMD != "claude-sonnet-5-5" {
		t.Errorf("pages = %+v", pages)
	}
	if len(h.rt.starts) != 1 {
		t.Errorf("live switch should not respawn; starts = %d", len(h.rt.starts))
	}
	if !slices.ContainsFunc(h.rt.lines(), func(l string) bool { return strings.Contains(l, `"subtype":"set_model"`) }) {
		t.Error("set_model not sent")
	}
}

func TestModifiersRenderAndEffortRespawns(t *testing.T) {
	h := newHarness(t, "multi-turn-partial")
	v, _ := h.m.Create("code", t.TempDir(), "opus")
	h.m.ToggleModifier(v.ID, "efficiency", true)
	pages := h.send(v.ID, "one")
	if !strings.Contains(pages[0].Directives, "minimum tokens") || !slices.Equal(pages[0].Modifiers, []string{"efficiency"}) {
		t.Errorf("page 1 = %+v", pages[0])
	}
	h.m.ToggleModifier(v.ID, "thorough", true) // replaces efficiency; needs --effort high
	pages = h.send(v.ID, "two")
	if !strings.Contains(pages[1].Directives, "Verify, check edge cases") || !strings.Contains(pages[1].Directives, `"Efficiency Mode" instruction no longer applies`) {
		t.Errorf("page 2 directives = %q", pages[1].Directives)
	}
	if len(h.rt.starts) != 2 {
		t.Fatalf("effort change should respawn: %d starts", len(h.rt.starts))
	}
	if args := strings.Join(h.rt.starts[1].Args, " "); !strings.Contains(args, "--effort high") || !strings.Contains(args, "--resume") {
		t.Errorf("respawn must resume with --effort high: %s", args)
	}
	sent := 0
	for _, l := range h.rt.lines() {
		var turn struct {
			Type    string
			Message struct{ Content string }
		}
		if json.Unmarshal([]byte(l), &turn) == nil && turn.Type == "user" && strings.HasPrefix(turn.Message.Content, "<session_directives>") {
			sent++
		}
	}
	if sent != 2 {
		t.Errorf("directives sent with %d turns, want 2", sent)
	}
}

func TestUnexpectedExitMidTurn(t *testing.T) {
	h := newHarness(t, "single-turn")
	h.rt.turns = nil
	v, _ := h.m.Create("chat", "", "haiku")
	h.m.Send(context.Background(), v.ID, "hello")
	h.rt.mu.Lock()
	p := h.rt.procs[0]
	h.rt.mu.Unlock()
	p.exit(3)
	pages := h.waitIdle(v.ID)
	if pages[0].Status != "error" || !strings.Contains(pages[0].Error, "stopped unexpectedly") {
		t.Errorf("page = %+v", pages[0])
	}
	s, _ := h.m.get(v.ID)
	if s.View().Running {
		t.Error("process should be gone")
	}
	// the next turn respawns
	h.rt.turns = segments(t, "single-turn")
	pages = h.send(v.ID, "again")
	if pages[1].Status != "done" || len(h.rt.starts) != 2 {
		t.Errorf("respawn failed: %+v, starts %d", pages[1], len(h.rt.starts))
	}
}

func TestSlashCommandPages(t *testing.T) {
	h := newHarness(t, "slash-commands")
	v, _ := h.m.Create("code", t.TempDir(), "haiku")
	h.m.Focus(v.ID)
	for _, q := range []string{"Reply with just: ready", "/cost", "/context", "/compact", "/model sonnet", "/help", "/clear"} {
		h.send(v.ID, q)
	}
	pages := h.send(v.ID, "What did I ask first?")
	if !strings.Contains(pages[1].AnswerMD, "subscription") || pages[3].AnswerMD == "" || pages[6].AnswerMD != "Conversation cleared." {
		t.Errorf("slash pages: cost=%q compact=%q clear=%q", pages[1].AnswerMD, pages[3].AnswerMD, pages[6].AnswerMD)
	}
	s, _ := h.m.get(v.ID)
	if sid := s.View().ProviderSID; sid != "003f62ed-fcb9-4f24-aebe-59d5f3007ea8" {
		t.Errorf("provider sid after /clear = %s", sid)
	}
}

func TestModelsFromHandshakeArePersisted(t *testing.T) {
	dir := t.TempDir()
	h := openHarness(t, dir, "perm-stdio-allow")
	v, _ := h.m.Create("chat", "", "haiku")
	h.send(v.ID, "write hello.txt")
	deadline := time.Now().Add(2 * time.Second)
	for len(h.m.Models()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if len(h.m.Models()) == 0 {
		t.Fatal("models from initialize not recorded")
	}
	h.m.Close()
	h.db.Close()
	if h2 := openHarness(t, dir, "single-turn"); len(h2.m.Models()) == 0 {
		t.Error("models should survive a restart")
	}
}

// A page the UI receives while it is still open must carry lists, not
// null: the frontend reads trace.length straight away.
func TestOpenPageHasNoNullLists(t *testing.T) {
	h := newHarness(t, "single-turn")
	h.rt.turns = nil
	pages := make(chan store.Page, 8)
	h.m.d.Sink = pageSink{h.sink, pages}
	v, _ := h.m.Create("chat", "", "haiku")
	if err := h.m.Send(context.Background(), v.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	p := <-pages
	b, _ := json.Marshal(p)
	for _, field := range []string{`"trace":null`, `"touchedFiles":null`, `"modifiers":null`} {
		if strings.Contains(string(b), field) {
			t.Errorf("open page has %s: %s", field, b)
		}
	}
}

type pageSink struct {
	*recSink
	pages chan store.Page
}

func (s pageSink) PageChanged(p store.Page) { s.pages <- p }

func TestSetSortOrder(t *testing.T) {
	h := newHarness(t, "single-turn")
	a, _ := h.m.Create("chat", "", "haiku")
	b, _ := h.m.Create("chat", "", "haiku")
	if l := h.m.List(); l[0].ID != b.ID {
		t.Fatal("newest first")
	}
	if _, err := h.m.SetSortOrder(a.ID, b.SortOrder+1); err != nil {
		t.Fatal(err)
	}
	if l := h.m.List(); l[0].ID != a.ID {
		t.Errorf("order = %s, %s", l[0].Title, l[1].Title)
	}
	recs, _ := h.db.ListSessions()
	if recs[0].ID != a.ID {
		t.Error("order not saved")
	}
}

// A turn can carry files: they reach the CLI as content blocks and the page
// keeps a record of them (not their content), across a reopen of the store.
func TestSendWithAttachments(t *testing.T) {
	h := newHarness(t, "document-input")
	v, err := h.m.Create("chat", "", "")
	if err != nil {
		t.Fatal(err)
	}
	files := []core.Attachment{
		{Name: "brief.pdf", Path: `C:\docs\brief.pdf`, MediaType: "application/pdf", Data: []byte("%PDF-1.4 tiny")},
		{Name: "notes.txt", Path: `C:\docs\notes.txt`, MediaType: "text/plain", Data: []byte("Second code word: MARMALADE.\n")},
	}
	if err := h.m.Send(context.Background(), v.ID, "", files...); err != nil {
		t.Fatal(err)
	}
	pages := h.waitIdle(v.ID)
	var turn string
	for _, l := range h.rt.lines() {
		if strings.Contains(l, `"type":"user"`) {
			turn = l
		}
	}
	for _, want := range []string{`"type":"document"`, `"title":"brief.pdf"`, `"title":"notes.txt"`, "MARMALADE"} {
		if !strings.Contains(turn, want) {
			t.Errorf("turn is missing %s: %s", want, turn)
		}
	}
	if len(pages) != 1 || !strings.Contains(pages[0].AnswerMD, "PELICAN") {
		t.Fatalf("pages = %+v", pages)
	}
	got := pages[0].Attachments
	if len(got) != 2 || got[0].Name != "brief.pdf" || got[0].Path != `C:\docs\brief.pdf` || got[1].MediaType != "text/plain" || got[1].Size != 29 {
		t.Errorf("attachments = %+v", got)
	}
	if s, _ := h.m.get(v.ID); s.View().Title != "brief.pdf" {
		t.Errorf("title = %q (an attachments-only turn names the session after the first file)", s.View().Title)
	}
	again, err := h.db.ListPages(v.ID)
	if err != nil || len(again[0].Attachments) != 2 {
		t.Errorf("stored attachments = %+v, %v", again, err)
	}
}
