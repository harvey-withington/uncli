package antigravity

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"uncli/internal/core"
)

const fixtures = "../../../testdata/streams/antigravity/" + PinnedVersion

func parseFixture(t *testing.T, name string) []core.Event {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtures, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p := newParser()
	var evs []core.Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		out, err := p.Feed(sc.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		evs = append(evs, out...)
	}
	return evs
}

func of[T any](t *testing.T, evs []core.Event, kind core.EventKind) []T {
	t.Helper()
	var out []T
	for _, e := range evs {
		if e.Kind == kind {
			v, err := core.Decode[T](e)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
	}
	return out
}

// Every recorded line is understood: nothing comes out unknown.
func TestFixturesParse(t *testing.T) {
	names, _ := filepath.Glob(filepath.Join(fixtures, "*.jsonl"))
	n := 0
	for _, f := range names {
		base := filepath.Base(f)
		if strings.HasSuffix(base, ".in.jsonl") || strings.HasSuffix(base, ".hook.jsonl") {
			continue
		}
		n++
		for _, e := range parseFixture(t, strings.TrimSuffix(base, ".jsonl")) {
			if e.Kind == core.EvUnknown {
				t.Errorf("%s: unknown line %s", base, e.Raw)
			}
		}
	}
	if n < 6 {
		t.Fatalf("only %d fixtures", n)
	}
}

func TestMultiTurn(t *testing.T) {
	evs := parseFixture(t, "multi-turn")
	ready := of[core.SessionReady](t, evs, core.EvSessionReady)
	if len(ready) != 1 || ready[0].ProviderSID == "" || ready[0].Model != "gemini-3.8-flash-low" {
		t.Fatalf("ready = %+v", ready)
	}
	if n := len(of[struct{}](t, evs, core.EvTurnStarted)); n != 2 {
		t.Errorf("%d turns started", n)
	}
	blocks := of[core.TextBlock](t, evs, core.EvTextBlock)
	if len(blocks) != 2 || blocks[0].Text != "OK" || !strings.Contains(blocks[1].Text, "console.log(42);") {
		t.Errorf("blocks = %+v", blocks)
	}
	res := of[core.TurnResult](t, evs, core.EvTurnResult)
	if len(res) != 2 || res[0].IsError || res[1].IsError {
		t.Fatalf("results = %+v", res)
	}
	// Each turn's own usage, not the process's running total.
	if res[1].Usage.InputTokens >= res[0].Usage.InputTokens*2 || res[1].Usage.InputTokens == 0 {
		t.Errorf("second turn's usage = %+v (first %+v)", res[1].Usage, res[0].Usage)
	}
	if res[0].DurationMS <= 0 || res[1].DurationMS <= 0 {
		t.Errorf("durations = %d, %d", res[0].DurationMS, res[1].DurationMS)
	}
}

func TestBadModel(t *testing.T) {
	evs := parseFixture(t, "error-bad-model")
	res := of[core.TurnResult](t, evs, core.EvTurnResult)
	if len(res) != 1 || !res[0].IsError || !strings.Contains(res[0].Text, "no-such-model") {
		t.Errorf("result = %+v", res)
	}
	if len(of[core.ErrorInfo](t, evs, core.EvError)) != 1 {
		t.Error("no error event")
	}
}

func TestToolsThroughTheHook(t *testing.T) {
	evs := parseFixture(t, "tools-hook-allow")
	conv := of[core.SessionReady](t, evs, core.EvSessionReady)[0].ProviderSID
	started := of[core.ToolStarted](t, evs, core.EvToolStarted)
	names := []string{}
	for _, s := range started {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "view_file,write_to_file,run_command" || started[2].ID != ToolID(conv, 6) {
		t.Fatalf("started = %+v", started)
	}
	if started[2].Summary != "Get-Content colour.txt" {
		t.Errorf("summary = %q", started[2].Summary)
	}
	fin := of[core.ToolFinished](t, evs, core.EvToolFinished)
	if len(fin) != 3 || !fin[0].OK || !fin[2].OK || !strings.Contains(fin[2].Output, "teal") {
		t.Errorf("finished = %+v", fin)
	}
	files := of[core.FileTouched](t, evs, core.EvFileTouched)
	if len(files) != 1 || files[0].Path != `C:\uncli-spike\colour.txt` || files[0].How != "write" {
		t.Errorf("files = %+v", files)
	}

	// The hook's calls name the same steps, so a card belongs to its tool.
	calls := readHookCalls(t, "tools-hook-allow")
	a := &Adapter{}
	want := map[string]string{"view_file": core.ActRead, "write_to_file": core.ActWrite, "run_command": core.ActShell}
	for i, raw := range calls {
		c, err := a.ReadHook(EventPreTool, raw)
		if err != nil {
			t.Fatal(err)
		}
		ap := c.Approval
		if c.Kind != core.HookTool || ap.ToolUseID != started[i].ID || ap.Action.Kind != want[ap.Tool] {
			t.Errorf("call %d = %+v (stream id %s)", i, ap, started[i].ID)
		}
	}
	shell, _ := a.ReadHook(EventPreTool, calls[2])
	if shell.Approval.Action.Command != "Get-Content colour.txt" || (runtime.GOOS == "windows" && shell.Approval.Action.Dialect != "powershell") {
		t.Errorf("shell = %+v", shell.Approval.Action)
	}
	if write, _ := a.ReadHook(EventPreTool, calls[1]); write.Approval.Action.Path != `C:\uncli-spike\colour.txt` {
		t.Errorf("write = %+v", write.Approval.Action)
	}
}

func TestDeniedByTheHook(t *testing.T) {
	evs := parseFixture(t, "tools-hook-deny")
	fin := of[core.ToolFinished](t, evs, core.EvToolFinished)
	if len(fin) != 1 || fin[0].OK || !fin[0].Denied {
		t.Errorf("finished = %+v", fin)
	}
}

func TestSubagent(t *testing.T) {
	evs := parseFixture(t, "subagent")
	started := of[core.ToolStarted](t, evs, core.EvToolStarted)
	fin := of[core.ToolFinished](t, evs, core.EvToolFinished)
	if len(started) != 1 || started[0].Name != "invoke_subagent" || len(fin) != 1 || !fin[0].OK {
		t.Errorf("started %+v, finished %+v", started, fin)
	}
	res := of[core.TurnResult](t, evs, core.EvTurnResult)
	if len(res) != 1 || !strings.Contains(strings.ToLower(res[0].Text), "teal") {
		t.Errorf("result = %+v", res)
	}
	// The sub-agent's own calls reach the hook under its own conversation.
	for _, raw := range readHookCalls(t, "subagent") {
		var c hookCall
		_ = json.Unmarshal(raw, &c)
		if c.ConversationID == "" || c.ToolCall.Name == "" {
			t.Errorf("call = %s", raw)
		}
	}
}

func readHookCalls(t *testing.T, name string) []json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtures, name+".hook.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []json.RawMessage
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		out = append(out, json.RawMessage(l))
	}
	return out
}
