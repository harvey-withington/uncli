package session

import (
	"bufio"
	"context"
	"encoding/json"

	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"uncli/config"
	"uncli/internal/adapter/claude"
	"uncli/internal/adapter/manifest"
	"uncli/internal/core"
	"uncli/internal/profile"
	"uncli/internal/store"
)

// An ACP provider (decision 0014) through the session: a fake agent that
// answers initialize with Grok's recorded answer and otherwise speaks ACP
// as its specification and the measurements of Grok describe.

const grokFixtures = "../../testdata/streams/grok/1.0.50"

type fakeACP struct {
	mu       sync.Mutex
	starts   []core.Command
	received []map[string]any // what UNCLI sent, every process
	signedIn bool
	loaded   []string // sessions loaded
}

func (f *fakeACP) ID() string { return "local" }

func (f *fakeACP) Start(_ context.Context, cmd core.Command, workdir string) (core.Proc, error) {
	f.mu.Lock()
	f.starts = append(f.starts, cmd)
	f.mu.Unlock()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	p := &acpProc{in: inW, out: outR, done: make(chan struct{})}
	go f.serve(p, inR, outW, workdir)
	return p, nil
}

type acpProc struct {
	in   *io.PipeWriter
	out  *io.PipeReader
	once sync.Once
	done chan struct{}
}

func (p *acpProc) Stdin() io.Writer  { return p.in }
func (p *acpProc) Stdout() io.Reader { return p.out }
func (p *acpProc) Stderr() io.Reader { return strings.NewReader("") }
func (p *acpProc) Wait() error       { <-p.done; return nil }
func (p *acpProc) Kill() error {
	p.once.Do(func() { close(p.done); p.in.Close(); p.out.Close() })
	return nil
}

// serve is the agent: one JSON-RPC message a line each way.
func (f *fakeACP) serve(p *acpProc, in *io.PipeReader, out *io.PipeWriter, workdir string) {
	defer out.Close()
	send := func(v any) { b, _ := json.Marshal(v); _, _ = out.Write(append(b, '\n')) }
	initAnswer, _ := os.ReadFile(filepath.Join(grokFixtures, "initialize-signed-out.jsonl"))
	recorded := strings.SplitN(string(initAnswer), "\n", 2)[0]
	sc := bufio.NewScanner(in)
	answers := make(chan map[string]any, 8) // UNCLI's answers to the agent's requests
	var prompt json.RawMessage              // the prompt waiting on a permission or cancel
	cancelled := make(chan struct{}, 1)
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		f.mu.Lock()
		f.received = append(f.received, m)
		signedIn := f.signedIn
		f.mu.Unlock()
		id, _ := json.Marshal(m["id"])
		method, _ := m["method"].(string)
		params, _ := m["params"].(map[string]any)
		switch {
		case method == "" && m["id"] != nil:
			answers <- m
		case method == "initialize":
			var rec map[string]any
			_ = json.Unmarshal([]byte(recorded), &rec)
			rec["id"] = m["id"]
			send(rec)
		case method == "session/new":
			if !signedIn {
				send(map[string]any{"jsonrpc": "2.0", "id": m["id"], "error": map[string]any{"code": -32000, "message": "Authentication required", "data": "no auth method id provided"}})
				continue
			}
			send(map[string]any{"jsonrpc": "2.0", "method": "_x.ai/session/setup", "params": map[string]any{"phase": "auth"}})
			send(map[string]any{"jsonrpc": "2.0", "id": m["id"], "result": map[string]any{"sessionId": "grok-session-1"}})
		case method == "session/load":
			f.mu.Lock()
			f.loaded = append(f.loaded, params["sessionId"].(string))
			f.mu.Unlock()
			// The conversation so far, replayed before the answer.
			for _, u := range []map[string]any{
				{"sessionUpdate": "user_message_chunk", "content": map[string]any{"type": "text", "text": "hi"}},
				{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "OLD ANSWER"}},
			} {
				send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "grok-session-1", "update": u}})
			}
			send(map[string]any{"jsonrpc": "2.0", "id": m["id"], "result": map[string]any{}})
		case method == "session/cancel":
			cancelled <- struct{}{}
		case method == "session/prompt":
			prompt = id
			text := ""
			if l, ok := params["prompt"].([]any); ok && len(l) > 0 {
				text, _ = l[0].(map[string]any)["text"].(string)
			}
			go f.turn(send, prompt, text, workdir, answers, cancelled)
		}
	}
	_ = p
}

func update(sid string, u map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": sid, "update": u}}
}

// turn answers one prompt.
func (f *fakeACP) turn(send func(any), id json.RawMessage, text, workdir string, answers chan map[string]any, cancelled chan struct{}) {
	end := func(stop string) {
		send(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": map[string]any{"stopReason": stop}})
	}
	sid := "grok-session-1"
	switch {
	case strings.Contains(text, "write"):
		path := filepath.Join(workdir, "hello.txt")
		send(update(sid, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "call-1", "title": "Write hello.txt", "kind": "edit",
			"status": "pending", "rawInput": map[string]any{"path": path, "content": "hi"}, "locations": []any{map[string]any{"path": path}}}))
		// An agent request UNCLI doesn't serve: it must be refused, not left waiting.
		send(map[string]any{"jsonrpc": "2.0", "id": 99, "method": "fs/read_text_file", "params": map[string]any{"path": path}})
		send(map[string]any{"jsonrpc": "2.0", "id": 100, "method": "session/request_permission", "params": map[string]any{
			"sessionId": sid,
			"toolCall":  map[string]any{"toolCallId": "call-1", "title": "Write hello.txt", "kind": "edit", "rawInput": map[string]any{"path": path, "content": "hi"}, "locations": []any{map[string]any{"path": path}}},
			"options": []any{
				map[string]any{"optionId": "yes", "name": "Allow", "kind": "allow_once"},
				map[string]any{"optionId": "always", "name": "Always", "kind": "allow_always"},
				map[string]any{"optionId": "no", "name": "Reject", "kind": "reject_once"},
			}}})
		for a := range answers {
			if text, _ := json.Marshal(a["id"]); string(text) != "100" {
				continue
			}
			res, _ := a["result"].(map[string]any)
			outcome, _ := res["outcome"].(map[string]any)
			if outcome["optionId"] != "yes" {
				end("cancelled") // as Grok does on reject_once
				return
			}
			_ = os.WriteFile(path, []byte("hi"), 0o644)
			send(update(sid, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "call-1", "status": "completed",
				"content": []any{map[string]any{"type": "diff", "path": path, "oldText": nil, "newText": "hi"}}}))
			send(update(sid, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "Done."}}))
			end("end_turn")
			return
		}
	case strings.Contains(text, "slow"):
		send(update(sid, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "Thinking about it"}}))
		select {
		case <-cancelled:
			end("cancelled")
		case <-time.After(10 * time.Second):
			end("end_turn")
		}
	default:
		send(update(sid, map[string]any{"sessionUpdate": "agent_thought_chunk", "content": map[string]any{"type": "text", "text": "hmm"}}))
		for _, s := range []string{"Hel", "lo!"} {
			send(update(sid, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": s}}))
		}
		send(update(sid, map[string]any{"sessionUpdate": "usage_update", "used": 100, "size": 1000}))
		end("end_turn")
	}
}

func (f *fakeACP) sent(method string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, m := range f.received {
		if m["method"] == method {
			out = append(out, m)
		}
	}
	return out
}

func newACPHarness(t *testing.T, signedIn bool) (*Manager, *fakeACP, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "uncli.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	set, err := profile.Load(defaults, "")
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := manifest.Load("../../testdata/providers/grok")
	if err != nil {
		t.Fatal(err)
	}
	grok := manifest.New(m, nil, filepath.Join(dir, "grok-home"), "")
	rt := &fakeACP{signedIn: signedIn}
	mgr, err := NewManager(Deps{
		Store: db, Adapter: claude.New(nil), Others: []core.Adapter{grok}, Runtime: rt, Profiles: set, Sink: &recSink{},
		ScratchDir: filepath.Join(dir, "scratch"),
		Binary:     func(_ context.Context, p string) (string, string, error) { return p + ".exe", "test", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Close)
	return mgr, rt, dir
}

func waitView(t *testing.T, m *Manager, id string, ok func(View) bool, what string) View {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if v, _ := m.View(id); ok(v) {
			return v
		}
		time.Sleep(5 * time.Millisecond)
	}
	v, _ := m.View(id)
	t.Fatalf("waited in vain for %s: %+v", what, v)
	return View{}
}

func lastPage(t *testing.T, m *Manager, id string) store.Page {
	t.Helper()
	pages, _ := m.Pages(id)
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	return pages[len(pages)-1]
}

func TestACPSession(t *testing.T) {
	m, rt, dir := newACPHarness(t, true)
	work := filepath.Join(dir, "work")
	_ = os.MkdirAll(work, 0o755)
	v, err := m.CreateWith(NewSession{Profile: "code", Workdir: work, Provider: "grok"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetMode(v.ID, ModeAlways); err != nil {
		t.Fatal(err)
	}
	idle := func(v View) bool { return !v.Busy }

	// The handshake, then a streamed answer.
	if err := m.Send(context.Background(), v.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, idle, "the first answer")
	if p := lastPage(t, m, v.ID); p.AnswerMD != "Hello!" || p.Status != "done" {
		t.Fatalf("page = %q (%s)", p.AnswerMD, p.Status)
	}
	init := rt.sent("initialize")
	caps, _ := init[0]["params"].(map[string]any)["clientCapabilities"].(map[string]any)
	if caps["terminal"] != false || caps["fs"].(map[string]any)["writeTextFile"] != false {
		t.Errorf("UNCLI offered the agent files or a terminal: %v", caps)
	}
	if got, _ := m.View(v.ID); got.ProviderSID != "grok-session-1" {
		t.Errorf("provider session = %q", got.ProviderSID)
	}

	// A write waits on a card; allowed, it's done and listed.
	if err := m.Send(context.Background(), v.ID, "write a file"); err != nil {
		t.Fatal(err)
	}
	card := waitView(t, m, v.ID, func(v View) bool { return len(v.Approvals) == 1 }, "a card").Approvals[0]
	if card.Action.Kind != core.ActEdit || !strings.HasSuffix(card.Action.Path, "hello.txt") || card.ToolUseID != "call-1" {
		t.Errorf("card = %+v", card)
	}
	if err := m.Answer(v.ID, card.RequestID, Allow, ""); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, idle, "the write's turn")
	p := lastPage(t, m, v.ID)
	if p.AnswerMD != "Done." || len(p.Trace) != 1 || p.Trace[0].Approved != "you" || !p.Trace[0].OK {
		t.Errorf("page = %q, trace %+v", p.AnswerMD, p.Trace)
	}
	if len(p.TouchedFiles) != 1 || p.TouchedFiles[0].How != "write" {
		t.Errorf("touched = %+v", p.TouchedFiles)
	}
	// The agent's request for a file was refused, not left waiting.
	var refused bool
	for _, a := range rt.received {
		if id, _ := json.Marshal(a["id"]); string(id) == "99" && a["error"] != nil {
			refused = true
		}
	}
	if !refused {
		t.Error("the agent's fs request wasn't answered")
	}
	// UNCLI answered with "once", never "always".
	for _, a := range rt.received {
		if id, _ := json.Marshal(a["id"]); string(id) == "100" {
			if o := a["result"].(map[string]any)["outcome"].(map[string]any); o["optionId"] != "yes" {
				t.Errorf("answer = %v", o)
			}
		}
	}

	// Denied: the agent ends the turn (as Grok does).
	_ = os.Remove(filepath.Join(work, "hello.txt"))
	if err := m.Send(context.Background(), v.ID, "write again"); err != nil {
		t.Fatal(err)
	}
	card = waitView(t, m, v.ID, func(v View) bool { return len(v.Approvals) == 1 }, "a second card").Approvals[0]
	if err := m.Answer(v.ID, card.RequestID, Deny, ""); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, idle, "the denied turn")
	if _, err := os.Stat(filepath.Join(work, "hello.txt")); err == nil {
		t.Error("the file was written after a denial")
	}
	if p := lastPage(t, m, v.ID); p.Status != "interrupted" {
		t.Errorf("denied turn = %s", p.Status)
	}

	// Stop mid-turn is ACP's cancel, not a kill.
	if err := m.Send(context.Background(), v.ID, "slow one"); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, func(v View) bool { return v.State == Writing || v.State == Thinking }, "the slow turn")
	time.Sleep(100 * time.Millisecond)
	if err := m.Interrupt(v.ID); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, idle, "the stop")
	if len(rt.sent("session/cancel")) != 1 || len(rt.starts) != 1 {
		t.Errorf("cancels %d, processes %d", len(rt.sent("session/cancel")), len(rt.starts))
	}
	if p := lastPage(t, m, v.ID); p.Status != "interrupted" {
		t.Errorf("stopped turn = %s", p.Status)
	}
}

// After a restart the conversation is loaded, and its replay isn't taken
// for a new answer.
func TestACPResume(t *testing.T) {
	m, rt, _ := newACPHarness(t, true)
	v, err := m.CreateWith(NewSession{Profile: "chat", Provider: "grok"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Send(context.Background(), v.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, func(v View) bool { return !v.Busy }, "the first answer")
	s, _ := m.get(v.ID)
	s.stop()
	if err := m.Send(context.Background(), v.ID, "hi again"); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, func(v View) bool { return !v.Busy }, "the resumed answer")
	if len(rt.loaded) != 1 || rt.loaded[0] != "grok-session-1" || len(rt.sent("session/new")) != 1 {
		t.Errorf("loaded %v, new %d", rt.loaded, len(rt.sent("session/new")))
	}
	if p := lastPage(t, m, v.ID); p.AnswerMD != "Hello!" {
		t.Errorf("answer = %q (the replay leaked in?)", p.AnswerMD)
	}
}

// Signed out, the turn fails with what to do, and the next is refused at once.
func TestACPSignedOut(t *testing.T) {
	m, _, _ := newACPHarness(t, false)
	v, err := m.CreateWith(NewSession{Profile: "chat", Provider: "grok"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Send(context.Background(), v.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	waitView(t, m, v.ID, func(v View) bool { return !v.Busy }, "the failed turn")
	if p := lastPage(t, m, v.ID); p.Status != "error" || !strings.Contains(p.Error, "isn't signed in") {
		t.Errorf("page = %s %q", p.Status, p.Error)
	}
	if err := m.Send(context.Background(), v.ID, "hi"); err == nil || !strings.Contains(err.Error(), "signed in") {
		t.Errorf("second send: %v", err)
	}
}
