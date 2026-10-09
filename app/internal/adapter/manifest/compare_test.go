package manifest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"uncli/internal/adapter/antigravity"
	"uncli/internal/core"
)

// The engine is proven against a built-in (decision 0013): the Antigravity
// CLI written as a manifest must behave exactly like UNCLI's Go adapter for
// it, on every recorded fixture.

const (
	agyManifest = "../../../testdata/providers/antigravity"
	agyFixtures = "../../../testdata/streams/antigravity/" + antigravity.PinnedVersion
)

func pair(t *testing.T) (*antigravity.Adapter, *Adapter) {
	t.Helper()
	m, _, err := Load(agyManifest)
	if err != nil {
		t.Fatal(err)
	}
	hookCmd := `"C:\Program Files\UNCLI\uncli.exe" hook`
	return antigravity.New(nil, t.TempDir(), hookCmd), New(m, nil, t.TempDir(), hookCmd)
}

// same compares two values as JSON, so key order and raw bytes don't matter.
func same(t *testing.T, what string, want, got any) {
	t.Helper()
	norm := func(v any) any {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var out any
		_ = json.Unmarshal(b, &out)
		return out
	}
	if w, g := norm(want), norm(got); !reflect.DeepEqual(w, g) {
		wb, _ := json.Marshal(w)
		gb, _ := json.Marshal(g)
		t.Errorf("%s differs:\n  Go adapter: %s\n  manifest:   %s", what, wb, gb)
	}
}

func events(t *testing.T, p core.Parser, lines []string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range lines {
		evs, err := p.Feed([]byte(l))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range evs {
			var data any
			_ = json.Unmarshal(e.Data, &data)
			out = append(out, map[string]any{"kind": e.Kind, "data": data})
		}
	}
	return out
}

func fixtureLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

func TestSameEventsAsTheGoAdapter(t *testing.T) {
	goA, man := pair(t)
	names, _ := filepath.Glob(filepath.Join(agyFixtures, "*.jsonl"))
	n := 0
	for _, f := range names {
		base := filepath.Base(f)
		if strings.HasSuffix(base, ".in.jsonl") || strings.HasSuffix(base, ".hook.jsonl") {
			continue
		}
		n++
		lines := fixtureLines(t, f)
		want, got := events(t, goA.NewParser(), lines), events(t, man.NewParser(), lines)
		if len(want) != len(got) {
			t.Errorf("%s: %d events from the Go adapter, %d from the manifest", base, len(want), len(got))
		}
		for i := range min(len(want), len(got)) {
			same(t, base+" event "+string(rune('0'+i%10)), want[i], got[i])
		}
	}
	if n < 6 {
		t.Fatalf("only %d fixtures", n)
	}
	// Lines it doesn't know come out unknown, from both.
	for _, l := range []string{`not json`, `{"event":"mystery"}`, `[1,2]`} {
		same(t, "unknown line "+l, events(t, goA.NewParser(), []string{l})[0]["kind"], events(t, man.NewParser(), []string{l})[0]["kind"])
	}
}

func TestSameHookCalls(t *testing.T) {
	goA, man := pair(t)
	names, _ := filepath.Glob(filepath.Join(agyFixtures, "*.hook.jsonl"))
	calls := 0
	for _, f := range names {
		for _, l := range fixtureLines(t, f) {
			calls++
			w, werr := goA.ReadHook(antigravity.EventPreTool, []byte(l))
			g, gerr := man.ReadHook(antigravity.EventPreTool, []byte(l))
			if (werr == nil) != (gerr == nil) {
				t.Fatalf("errors differ: %v / %v", werr, gerr)
			}
			same(t, filepath.Base(f)+" call", w, g)
			// And what each tool does, for the safe list and the cards.
			same(t, "action of "+w.Approval.Tool, antigravity.ActionOf(w.Approval.Tool, w.Approval.Input), man.ActionOf(g.Approval.Tool, g.Approval.Input))
		}
	}
	if calls < 5 {
		t.Fatalf("only %d hook calls", calls)
	}
	w, _ := goA.ReadHook(antigravity.EventPreInvocation, []byte(`{"conversationId":"x"}`))
	g, _ := man.ReadHook(antigravity.EventPreInvocation, []byte(`{"conversationId":"x"}`))
	same(t, "instructions call", w, g)
	if _, err := man.ReadHook(antigravity.EventPreTool, []byte(`{"toolCall":{}}`)); err == nil {
		t.Error("a call naming no tool was read")
	}

	// The answers, byte for byte.
	tool, ins := core.HookCall{Kind: core.HookTool}, core.HookCall{Kind: core.HookInstructions}
	for _, c := range []struct {
		call core.HookCall
		ans  core.HookAnswer
	}{
		{tool, core.HookAnswer{Allow: true}}, {tool, core.HookAnswer{Reason: "no"}}, {tool, core.HookAnswer{}},
		{ins, core.HookAnswer{Instructions: "Be brief."}}, {ins, core.HookAnswer{}},
	} {
		wb, _ := goA.AnswerHook(c.call, c.ans)
		gb, _ := man.AnswerHook(c.call, c.ans)
		if string(wb) != string(gb) {
			t.Errorf("answer %+v: %s / %s", c, wb, gb)
		}
	}
}

func TestSameCommand(t *testing.T) {
	goA, man := pair(t)
	for _, spec := range []core.LaunchSpec{
		{Approvals: true, ResumeID: "abc", Model: "gemini-3.8-flash-low", Effort: "high", PermissionMode: "plan", Env: map[string]string{"UNCLI_HOOK_TOKEN": "t"}},
		{SessionID: "new-id", Model: "m"},
		{},
	} {
		w, werr := goA.BuildCommand("agy.exe", spec)
		g, gerr := man.BuildCommand("agy.exe", spec)
		if werr != nil || gerr != nil {
			t.Fatal(werr, gerr)
		}
		// Each keeps its state in its own folder (the test's).
		g.Args = replaceAll(g.Args, man.StateDir, goA.StateDir)
		same(t, "command", w, g)
	}
	// The hook files, byte for byte.
	wb, _ := os.ReadFile(filepath.Join(goA.StateDir, "config", "hooks.json"))
	gb, err := os.ReadFile(filepath.Join(man.StateDir, "config", "hooks.json"))
	if err != nil || len(wb) == 0 || string(wb) != string(gb) {
		t.Errorf("hooks.json:\n%s\n---\n%s (%v)", wb, gb, err)
	}
	same(t, "capabilities", goA.Capabilities(), man.Capabilities())
	same(t, "sign-in command", replaceAll(goA.LoginCommand("agy.exe").Args, goA.StateDir, "S"), replaceAll(man.LoginCommand("agy.exe").Args, man.StateDir, "S"))
	same(t, "sign-in env", goA.LoginCommand("agy.exe").Env, man.LoginCommand("agy.exe").Env)
	same(t, "status command", replaceAll(goA.AuthStatusCommand("agy.exe").Args, goA.StateDir, "S"), replaceAll(man.AuthStatusCommand("agy.exe").Args, man.StateDir, "S"))
	out := []byte("Fetching available models...\ngemini-3.8-flash-high\tGemini 3.8 Flash (High)\r\nclaude-opus-5-5-low\tClaude Opus 5.5 (Low)\n")
	same(t, "models", goA.ParseModels(out), man.ParseModels(out))
	wa, _ := goA.ParseAuthStatus(out)
	ga, _ := man.ParseAuthStatus(out)
	same(t, "sign-in", wa, ga)
	wa, _ = goA.ParseAuthStatus([]byte("error: not signed in"))
	ga, _ = man.ParseAuthStatus([]byte("error: not signed in"))
	same(t, "signed out", wa, ga)
	for _, turn := range []core.UserTurn{{Text: "hi"}, {Text: "hi {text} \"quoted\"", Directives: "Be brief."}} {
		wl, _ := goA.EncodeTurn(turn)
		gl, _ := man.EncodeTurn(turn)
		if string(wl) != string(gl) {
			t.Errorf("turn: %s / %s", wl, gl)
		}
	}
	if _, err := man.EncodeTurn(core.UserTurn{Text: "x", Attachments: []core.Attachment{{Name: "a.png"}}}); err == nil {
		t.Error("an attachment was encoded")
	}
	same(t, "state paths", replaceAll(goA.StatePaths(), goA.StateDir, "S"), replaceAll(man.StatePaths(), man.StateDir, "S"))
}

func replaceAll(in []string, from, to string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ReplaceAll(s, from, to)
	}
	return out
}

// The comparison would notice a manifest that differs: a wrong path for the
// answer's text, and a tool given the wrong kind.
func TestComparisonNoticesDifferences(t *testing.T) {
	goA, man := pair(t)
	lines := fixtureLines(t, filepath.Join(agyFixtures, "multi-turn.jsonl"))
	if !reflect.DeepEqual(events(t, goA.NewParser(), lines), events(t, man.NewParser(), lines)) {
		t.Fatal("the fixture already differs")
	}
	for i := range man.M.Events {
		if r := man.M.Events[i].Result; r != nil {
			r.Text = "result.status"
		}
	}
	if reflect.DeepEqual(events(t, goA.NewParser(), lines), events(t, man.NewParser(), lines)) {
		t.Error("a wrong text path went unnoticed")
	}
	man.M.Tools.Kinds["run_command"] = "read"
	if reflect.DeepEqual(antigravity.ActionOf("run_command", nil), man.ActionOf("run_command", nil)) {
		t.Error("a wrong tool kind went unnoticed")
	}
}
