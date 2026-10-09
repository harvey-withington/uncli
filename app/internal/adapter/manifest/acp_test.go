package manifest

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/core"
)

// The ACP driver against Grok's recorded conversations (testdata/streams/
// grok/1.0.50, recorded 2026-10-09 with UNCLI's answers): it must send what
// was sent and read what came back.

const grokStreams = "../../../testdata/streams/grok/1.0.50"

type replay struct {
	events   []core.Event
	sent     []map[string]any // what the driver wrote
	approved map[string]bool  // tool calls answered
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// play feeds a recording through a driver, sending the recorded prompts
// when the driver is ready for them and answering approvals as decide says.
func play(t *testing.T, name string, spec core.LaunchSpec, decide func(core.ApprovalAsked) bool, interruptAfter int) replay {
	t.Helper()
	a := grokAdapter(t)
	d := a.NewDriver(spec)
	var r replay
	r.approved = map[string]bool{}
	write := func(lines ...[]byte) {
		for _, l := range lines {
			var m map[string]any
			if err := json.Unmarshal(l, &m); err != nil {
				t.Fatalf("driver wrote %q", l)
			}
			r.sent = append(r.sent, m)
		}
	}
	var prompts []string
	for _, l := range readLines(t, filepath.Join(grokStreams, name+".in.jsonl")) {
		var m map[string]any
		_ = json.Unmarshal([]byte(l), &m)
		if m["method"] == "session/prompt" {
			p := m["params"].(map[string]any)["prompt"].([]any)[0].(map[string]any)
			prompts = append(prompts, p["text"].(string))
		}
	}
	turn := 0
	send := func() {
		if turn < len(prompts) {
			b, err := d.Turn(core.UserTurn{Text: prompts[turn]})
			if err != nil {
				t.Fatal(err)
			}
			turn++
			if b != nil {
				write(b)
			}
		}
	}
	write(d.Start()...)
	deltas := 0
	for _, l := range readLines(t, filepath.Join(grokStreams, name+".jsonl")) {
		evs, replies := d.Feed([]byte(l))
		write(replies...)
		for _, e := range evs {
			r.events = append(r.events, e)
			switch e.Kind {
			case core.EvSessionReady, core.EvTurnResult:
				send()
			case core.EvTextDelta, core.EvThinking:
				if deltas++; interruptAfter > 0 && deltas == interruptAfter {
					b, ok := d.Control(core.Control{Kind: core.CtlInterrupt})
					if !ok {
						t.Fatal("no interrupt")
					}
					write(b)
				}
			case core.EvApprovalAsked:
				ap, _ := core.Decode[core.ApprovalAsked](e)
				allow := decide(ap)
				r.approved[ap.ToolUseID] = allow
				b, ok := d.Control(core.Control{Kind: core.CtlApprove, RequestID: ap.RequestID, Allow: allow})
				if !ok {
					t.Fatalf("no answer for %s", ap.RequestID)
				}
				write(b)
			}
		}
	}
	return r
}

func eventsOf[T any](t *testing.T, evs []core.Event, k core.EventKind) []T {
	t.Helper()
	var out []T
	for _, e := range evs {
		if e.Kind == k {
			v, _ := core.Decode[T](e)
			out = append(out, v)
		}
	}
	return out
}

func methods(sent []map[string]any) []string {
	var out []string
	for _, m := range sent {
		if s, ok := m["method"].(string); ok {
			out = append(out, s)
		} else {
			out = append(out, "answer")
		}
	}
	return out
}

// No line goes unread.
func TestGrokStreamsKnown(t *testing.T) {
	for _, name := range []string{"tools-allow", "tools-deny", "cancel", "resume", "initialize-signed-out"} {
		r := play(t, name, core.LaunchSpec{Workdir: `C:\uncli-spike`, ResumeID: map[bool]string{true: "01a12053-acda-7900-8713-41501e0ebff8"}[name == "resume"]},
			func(core.ApprovalAsked) bool { return name != "tools-deny" }, map[bool]int{true: 3}[name == "cancel"])
		for _, e := range r.events {
			if e.Kind == core.EvUnknown {
				t.Errorf("%s: unknown line %s", name, e.Raw)
			}
		}
	}
}

func TestGrokTools(t *testing.T) {
	r := play(t, "tools-allow", core.LaunchSpec{Workdir: `C:\uncli-spike`}, func(core.ApprovalAsked) bool { return true }, 0)
	if got := strings.Join(methods(r.sent), ","); got != "initialize,session/new,session/prompt,answer,answer,answer,session/prompt" &&
		got != "initialize,session/new,session/prompt,session/prompt,answer,answer,answer" {
		t.Errorf("sent %s", got)
	}
	// Each answer picks the agent's "once" option.
	for _, m := range r.sent {
		if res, ok := m["result"].(map[string]any); ok {
			if o := res["outcome"].(map[string]any); o["optionId"] != "allow-once" {
				t.Errorf("answer %v", o)
			}
		}
	}
	res := eventsOf[core.TurnResult](t, r.events, core.EvTurnResult)
	if len(res) != 2 || res[0].IsError || res[1].IsError {
		t.Fatalf("results = %+v", res)
	}
	// The turn's usage and cost, from the result's _meta.usage.
	if u := res[0].Usage; u.InputTokens != 14191 || u.OutputTokens != 24 || u.CacheRead != 1536 {
		t.Errorf("usage = %+v", u)
	}
	if math.Abs(res[0].TotalCost-0.026222) > 1e-9 || res[0].CostIsTotal {
		t.Errorf("cost = %v (total %v)", res[0].TotalCost, res[0].CostIsTotal)
	}
	blocks := eventsOf[core.TextBlock](t, r.events, core.EvTextBlock)
	if blocks[0].Text != "OK" || !strings.Contains(blocks[len(blocks)-1].Text, "teal") {
		t.Errorf("blocks = %+v", blocks)
	}
	started := eventsOf[core.ToolStarted](t, r.events, core.EvToolStarted)
	var names []string
	for _, s := range started {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "run_terminal_command,read_file,write,run_terminal_command" {
		t.Errorf("tools = %v", names)
	}
	if started[3].Summary != "Get-Content colour.txt" {
		t.Errorf("summary = %q", started[3].Summary)
	}
	asked := eventsOf[core.ApprovalAsked](t, r.events, core.EvApprovalAsked)
	if len(asked) != 3 {
		t.Fatalf("asked %d times (reading isn't asked about)", len(asked))
	}
	if a := asked[0].Action; a.Kind != core.ActShell || a.Dialect == "" || !strings.HasPrefix(a.Command, "Get-ChildItem") {
		t.Errorf("first ask = %+v", a)
	}
	if a := asked[1].Action; (a.Kind != core.ActEdit && a.Kind != core.ActWrite) || !strings.HasSuffix(a.Path, `colour.txt`) {
		t.Errorf("write ask = %+v", a)
	}
	files := eventsOf[core.FileTouched](t, r.events, core.EvFileTouched)
	if len(files) != 1 || files[0].How != "write" || !strings.HasSuffix(files[0].Path, "colour.txt") {
		t.Errorf("files = %+v", files)
	}
	fin := eventsOf[core.ToolFinished](t, r.events, core.EvToolFinished)
	if len(fin) != 4 || !fin[3].OK || !strings.Contains(fin[3].Output, "teal") {
		t.Errorf("finished = %+v", fin)
	}
}

// Refused: the tool fails as refused, and Grok ends the turn.
func TestGrokDenied(t *testing.T) {
	r := play(t, "tools-deny", core.LaunchSpec{Workdir: `C:\uncli-spike`}, func(core.ApprovalAsked) bool { return false }, 0)
	for _, m := range r.sent {
		if res, ok := m["result"].(map[string]any); ok {
			if o := res["outcome"].(map[string]any); o["optionId"] != "reject-once" {
				t.Errorf("answer %v", o)
			}
		}
	}
	fin := eventsOf[core.ToolFinished](t, r.events, core.EvToolFinished)
	if len(fin) != 1 || fin[0].OK || !fin[0].Denied {
		t.Errorf("finished = %+v", fin)
	}
	res := eventsOf[core.TurnResult](t, r.events, core.EvTurnResult)
	if len(res) != 1 || !res[0].Interrupted {
		t.Errorf("result = %+v", res)
	}
}

// Stopped mid-answer with session/cancel; the same process answers next.
func TestGrokCancel(t *testing.T) {
	r := play(t, "cancel", core.LaunchSpec{Workdir: `C:\uncli-spike`}, func(core.ApprovalAsked) bool { return true }, 3)
	if !strings.Contains(strings.Join(methods(r.sent), ","), "session/cancel") {
		t.Errorf("sent %v", methods(r.sent))
	}
	res := eventsOf[core.TurnResult](t, r.events, core.EvTurnResult)
	if len(res) != 2 || !res[0].Interrupted || res[1].Interrupted || res[1].IsError {
		t.Fatalf("results = %+v", res)
	}
	if blocks := eventsOf[core.TextBlock](t, r.events, core.EvTextBlock); blocks[len(blocks)-1].Text != "AFTER" {
		t.Errorf("after the stop: %+v", blocks)
	}
}

// Resumed with session/load: the replay isn't an answer.
func TestGrokResume(t *testing.T) {
	r := play(t, "resume", core.LaunchSpec{Workdir: `C:\uncli-spike`, ResumeID: "01a12053-acda-7900-8713-41501e0ebff8"}, func(core.ApprovalAsked) bool { return true }, 0)
	if got := strings.Join(methods(r.sent), ","); got != "initialize,session/load,session/prompt" {
		t.Errorf("sent %s", got)
	}
	ready := eventsOf[core.SessionReady](t, r.events, core.EvSessionReady)
	if len(ready) != 1 || ready[0].ProviderSID != "01a12053-acda-7900-8713-41501e0ebff8" {
		t.Errorf("ready = %+v", ready)
	}
	if blocks := eventsOf[core.TextBlock](t, r.events, core.EvTextBlock); len(blocks) != 1 || blocks[0].Text != "teal" {
		t.Errorf("blocks = %+v (the replay leaked in?)", blocks)
	}
	if n := len(eventsOf[core.ToolStarted](t, r.events, core.EvToolStarted)); n != 0 {
		t.Errorf("%d replayed tools", n)
	}
}

// Signed out: the first turn fails saying so.
func TestGrokSignedOut(t *testing.T) {
	a := grokAdapter(t)
	d := a.NewDriver(core.LaunchSpec{Workdir: `C:\uncli-spike`})
	d.Start()
	if b, err := d.Turn(core.UserTurn{Text: "hi"}); b != nil || err != nil {
		t.Fatalf("turn before ready = %s, %v", b, err)
	}
	var evs []core.Event
	for _, l := range readLines(t, filepath.Join(grokStreams, "initialize-signed-out.jsonl")) {
		e, replies := d.Feed([]byte(l))
		evs = append(evs, e...)
		for _, r := range replies {
			if !strings.Contains(string(r), "session/new") {
				t.Errorf("replied %s", r)
			}
		}
	}
	res := eventsOf[core.TurnResult](t, evs, core.EvTurnResult)
	if len(res) != 1 || !res[0].IsError || !strings.Contains(res[0].Text, "isn't signed in") {
		t.Errorf("result = %+v", res)
	}
	if _, err := d.Turn(core.UserTurn{Text: "again"}); err == nil {
		t.Error("a turn after a failed start was taken")
	}
	// The models it offers, from its model state.
	acc := eventsOf[core.Account](t, evs, core.EvAccount)
	if len(acc) != 1 || len(acc[0].Models) == 0 || acc[0].Models[0].Value == "" {
		t.Errorf("models = %+v", acc)
	}
}

func TestGrokModels(t *testing.T) {
	a := grokAdapter(t)
	in, _ := os.ReadFile(filepath.Join(grokStreams, "models-signed-in.txt"))
	out, _ := os.ReadFile(filepath.Join(grokStreams, "models-signed-out.txt"))
	if st, _ := a.ParseAuthStatus(in); !st.LoggedIn {
		t.Error("signed in read as out")
	}
	if st, _ := a.ParseAuthStatus(out); st.LoggedIn {
		t.Error("signed out read as in")
	}
	if ms := a.ParseModels(in); len(ms) != 1 || ms[0].Value != "grok-4.7" {
		t.Errorf("models = %+v", ms)
	}
}
