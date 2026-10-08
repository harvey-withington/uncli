package session

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"uncli/config"
	"uncli/internal/adapter/antigravity"
	"uncli/internal/adapter/claude"
	"uncli/internal/core"
	"uncli/internal/hook"
	"uncli/internal/profile"
	"uncli/internal/store"
)

const agyFixtures = "../../testdata/streams/antigravity/" + antigravity.PinnedVersion

// agyHarness runs an Antigravity session on the fake runtime with the real
// hook server. Its one turn is the recorded tools-hook-allow stream cut
// off when the shell command starts, so the turn stays open while the
// hook waits.
type agyHarness struct {
	*harness
	hooks *hook.Server
	calls []json.RawMessage // what the recorded hook was sent, in order
	id    string
}

func newAgyHarness(t *testing.T, profileID string) *agyHarness {
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
	b, err := os.ReadFile(filepath.Join(agyFixtures, "tools-hook-allow.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var turn bytes.Buffer
	for l := range bytes.SplitSeq(b, []byte("\n")) {
		turn.Write(l)
		turn.WriteByte('\n')
		if bytes.Contains(l, []byte(`"tool_name":"run_command"`)) && bytes.Contains(l, []byte(`"ACTIVE"`)) {
			break
		}
	}
	hooks, err := hook.Listen()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hooks.Close() })
	rt := &fakeRuntime{turns: [][]byte{turn.Bytes()}}
	sink := &recSink{}
	agy := antigravity.New(nil, filepath.Join(dir, "agy"), "uncli hook")
	m, err := NewManager(Deps{
		Store: db, Adapter: claude.New(nil), Others: []core.Adapter{agy}, Hooks: hooks, Runtime: rt, Profiles: set, Sink: sink,
		ScratchDir: filepath.Join(dir, "scratch"),
		Binary:     func(_ context.Context, p string) (string, string, error) { return p + ".exe", "test", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	h := &agyHarness{harness: &harness{t: t, m: m, rt: rt, sink: sink, db: db, dir: dir}, hooks: hooks}
	raw, _ := os.ReadFile(filepath.Join(agyFixtures, "tools-hook-allow.hook.jsonl"))
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		h.calls = append(h.calls, json.RawMessage(l))
	}
	v, err := m.CreateWith(NewSession{Profile: profileID, Workdir: `C:\uncli-spike`, Provider: "antigravity"})
	if err != nil {
		t.Fatal(err)
	}
	h.id = v.ID
	return h
}

// start sends the turn and waits until its shell command is running.
func (h *agyHarness) start() {
	h.t.Helper()
	if err := h.m.Send(context.Background(), h.id, "go"); err != nil {
		h.t.Fatal(err)
	}
	h.until(func(v View) bool { return v.State == RunningTools }, "the tool never started")
}

func (h *agyHarness) until(ok func(View) bool, msg string) View {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if v, _ := h.m.View(h.id); ok(v) {
			return v
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.t.Fatal(msg)
	return View{}
}

// ask runs the hook as the CLI would, for one recorded call, and returns
// what it prints and its exit code, once it ends.
func (h *agyHarness) ask(proc int, event string, payload []byte) <-chan [2]string {
	env := h.rt.starts[proc].Env
	out := make(chan [2]string, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		code := hook.Run(event, bytes.NewReader(payload), &stdout, &stderr, func(k string) string { return env[k] })
		out <- [2]string{stdout.String(), map[bool]string{true: "0", false: "1"}[code == 0]}
	}()
	return out
}

func wait(t *testing.T, ch <-chan [2]string) (string, string) {
	t.Helper()
	select {
	case r := <-ch:
		return r[0], r[1]
	case <-time.After(5 * time.Second):
		t.Fatal("the hook never got an answer")
		return "", ""
	}
}

func TestHookStartsTheCLIGated(t *testing.T) {
	h := newAgyHarness(t, "code")
	h.start()
	cmd := h.rt.starts[0]
	if cmd.Path != "antigravity.exe" || !slices.Contains(cmd.Args, "--dangerously-skip-permissions") {
		t.Errorf("command = %+v", cmd)
	}
	if cmd.Env[hook.AddrEnv] == "" || cmd.Env[hook.TokenEnv] == "" {
		t.Errorf("env = %v", cmd.Env)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "agy", "config", "hooks.json")); err != nil {
		t.Error(err)
	}
}

// Under "Always", writing a file waits for the user's card; the answer
// goes back to the waiting hook, and the trace says who allowed it.
func TestHookAskedOnACard(t *testing.T) {
	h := newAgyHarness(t, "code")
	if _, err := h.m.SetMode(h.id, ModeAlways); err != nil {
		t.Fatal(err)
	}
	h.start()
	res := h.ask(0, antigravity.EventPreTool, h.calls[1])
	v := h.until(func(v View) bool { return len(v.Approvals) == 1 }, "no card")
	a := v.Approvals[0]
	if a.Tool != "write_to_file" || a.Action.Path != `C:\uncli-spike\colour.txt` || !strings.HasSuffix(a.ToolUseID, ":4") {
		t.Errorf("card = %+v", a)
	}
	if err := h.m.Answer(h.id, a.RequestID, Allow, ""); err != nil {
		t.Fatal(err)
	}
	out, code := wait(t, res)
	if code != "0" || out != `{"decision":"allow"}` {
		t.Errorf("hook printed %q, exit %s", out, code)
	}
	pages, _ := h.m.Pages(h.id)
	approved := false
	for _, it := range pages[0].Trace {
		approved = approved || it.ID == a.ToolUseID && it.Approved == "you"
	}
	if !approved {
		t.Errorf("trace = %+v", pages[0].Trace)
	}
}

func TestHookDeniedOnACard(t *testing.T) {
	h := newAgyHarness(t, "code")
	if _, err := h.m.SetMode(h.id, ModeAlways); err != nil {
		t.Fatal(err)
	}
	h.start()
	res := h.ask(0, antigravity.EventPreTool, h.calls[1])
	v := h.until(func(v View) bool { return len(v.Approvals) == 1 }, "no card")
	if err := h.m.Answer(h.id, v.Approvals[0].RequestID, Deny, ""); err != nil {
		t.Fatal(err)
	}
	out, _ := wait(t, res)
	if out != `{"decision":"deny","reason":"`+deniedOnCard+`"}` {
		t.Errorf("hook printed %q", out)
	}
}

// Reading runs at every level without a card: a file, and a command that
// only reads.
func TestHookSafeRunsUnasked(t *testing.T) {
	h := newAgyHarness(t, "code")
	if _, err := h.m.SetMode(h.id, ModeAlways); err != nil {
		t.Fatal(err)
	}
	h.start()
	for _, call := range []json.RawMessage{h.calls[0], h.calls[2]} {
		out, code := wait(t, h.ask(0, antigravity.EventPreTool, call))
		if code != "0" || out != `{"decision":"allow"}` {
			t.Errorf("hook printed %q, exit %s", out, code)
		}
	}
	if v, _ := h.m.View(h.id); len(v.Approvals) != 0 {
		t.Errorf("cards = %+v", v.Approvals)
	}
}

// Stopping the turn refuses what waits, and the stopped process's hooks are
// refused from then on.
func TestHookReleasedWhenStopped(t *testing.T) {
	h := newAgyHarness(t, "code")
	if _, err := h.m.SetMode(h.id, ModeAlways); err != nil {
		t.Fatal(err)
	}
	h.start()
	res := h.ask(0, antigravity.EventPreTool, h.calls[1])
	h.until(func(v View) bool { return len(v.Approvals) == 1 }, "no card")
	if err := h.m.Interrupt(h.id); err != nil {
		t.Fatal(err)
	}
	if out, _ := wait(t, res); !strings.Contains(out, `"decision":"deny"`) {
		t.Errorf("hook printed %q", out)
	}
	if out, code := wait(t, h.ask(0, antigravity.EventPreTool, h.calls[0])); code != "1" || out != "" {
		t.Errorf("after stopping: %q, exit %s", out, code)
	}
}

// Before each model call the hook gives the model the session's
// instructions, which the CLI has no flag for.
func TestHookGivesInstructions(t *testing.T) {
	h := newAgyHarness(t, "chat")
	h.start()
	p, _ := h.m.d.Profiles.Profile("chat")
	out, code := wait(t, h.ask(0, antigravity.EventPreInvocation, []byte(`{"conversationId":"x","invocationNum":0}`)))
	if code != "0" {
		t.Fatalf("exit %s", code)
	}
	var ans struct {
		InjectSteps []struct {
			EphemeralMessage string `json:"ephemeralMessage"`
		} `json:"injectSteps"`
	}
	if err := json.Unmarshal([]byte(out), &ans); err != nil || len(ans.InjectSteps) != 1 || ans.InjectSteps[0].EphemeralMessage == "" {
		t.Fatalf("hook printed %q (%v)", out, err)
	}
	if p.SystemPrompt != "" && !strings.Contains(ans.InjectSteps[0].EphemeralMessage, firstLineOf(p.SystemPrompt)) {
		t.Errorf("instructions = %q", ans.InjectSteps[0].EphemeralMessage)
	}
}

func firstLineOf(s string) string { l, _, _ := strings.Cut(strings.TrimSpace(s), "\n"); return l }

// A session on a provider this UNCLI doesn't have can't start.
func TestUnknownProvider(t *testing.T) {
	h := newHarness(t, "multi-turn-partial")
	if _, err := h.m.CreateWith(NewSession{Profile: "chat", Provider: "nope"}); err == nil {
		t.Error("a session on an unknown provider was created")
	}
}
