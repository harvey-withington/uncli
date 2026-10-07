package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"uncli/internal/core"
)

const fixtures = "../../../testdata/streams/claude/" + PinnedVersion

func parseFixture(t *testing.T, name string) []core.Event {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtures, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p := newParser()
	var out []core.Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		evs, err := p.Feed(sc.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, ev := range evs {
			if len(ev.Raw) == 0 {
				t.Fatalf("%s: %s event without its raw line", name, ev.Kind)
			}
		}
		out = append(out, evs...)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func ofKind(evs []core.Event, k core.EventKind) []core.Event {
	var out []core.Event
	for _, ev := range evs {
		if ev.Kind == k {
			out = append(out, ev)
		}
	}
	return out
}

func decode[T any](t *testing.T, ev core.Event) T {
	t.Helper()
	v, err := core.Decode[T](ev)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func results(t *testing.T, evs []core.Event) []core.TurnResult {
	var out []core.TurnResult
	for _, ev := range ofKind(evs, core.EvTurnResult) {
		out = append(out, decode[core.TurnResult](t, ev))
	}
	return out
}

func TestEveryFixtureParses(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(fixtures, "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		if strings.HasSuffix(name, ".in") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			evs := parseFixture(t, name)
			if len(ofKind(evs, core.EvTurnResult)) == 0 {
				t.Error("no turn result")
			}
		})
	}
}

func TestMultiTurnPartial(t *testing.T) {
	evs := parseFixture(t, "multi-turn-partial")
	if n := len(ofKind(evs, core.EvSessionReady)); n != 1 {
		t.Errorf("session_ready %d times, want once (init repeats every turn)", n)
	}
	rs := results(t, evs)
	if len(rs) != 3 {
		t.Fatalf("results = %d, want 3", len(rs))
	}
	if rs[2].Text != "Forty-two." || rs[2].IsError {
		t.Errorf("last result = %+v", rs[2])
	}
	if !(rs[0].TotalCost < rs[1].TotalCost && rs[1].TotalCost < rs[2].TotalCost) || !rs[0].CostIsTotal {
		t.Errorf("cost should be a growing running total: %v %v %v", rs[0].TotalCost, rs[1].TotalCost, rs[2].TotalCost)
	}
	if rs[0].Usage.OutputTokens == 0 || rs[0].Usage.CacheWrite == 0 {
		t.Errorf("usage missing: %+v", rs[0].Usage)
	}
	var streamed strings.Builder
	for _, ev := range ofKind(evs, core.EvTextDelta) {
		streamed.WriteString(decode[core.TextDelta](t, ev).Text)
	}
	if !strings.Contains(streamed.String(), "console.log(42)") {
		t.Errorf("deltas didn't stream the code block: %q", streamed.String())
	}
	if len(ofKind(evs, core.EvTurnStarted)) < 3 {
		t.Error("expected a turn_started per turn")
	}
}

func TestInterruptThenContinue(t *testing.T) {
	rs := results(t, parseFixture(t, "interrupt-then-continue"))
	if len(rs) != 2 {
		t.Fatalf("results = %d", len(rs))
	}
	if !rs[0].Interrupted || rs[0].IsError {
		t.Errorf("first turn should be interrupted, not an error: %+v", rs[0])
	}
	if rs[1].Interrupted || rs[1].IsError || !strings.Contains(rs[1].Text, "Last Light") {
		t.Errorf("second turn should continue the conversation: %+v", rs[1])
	}
}

func TestResumeKeepsSessionID(t *testing.T) {
	multi := decode[core.SessionReady](t, ofKind(parseFixture(t, "multi-turn-partial"), core.EvSessionReady)[0])
	resumed := decode[core.SessionReady](t, ofKind(parseFixture(t, "resume-partial"), core.EvSessionReady)[0])
	if multi.ProviderSID != resumed.ProviderSID {
		t.Errorf("resume changed the session id: %s -> %s", multi.ProviderSID, resumed.ProviderSID)
	}
	if resumed.CLIVersion != PinnedVersion {
		t.Errorf("cli version = %q", resumed.CLIVersion)
	}
}

func TestSetModelLive(t *testing.T) {
	evs := parseFixture(t, "set-model-live")
	ready := ofKind(evs, core.EvSessionReady)
	if len(ready) != 2 {
		t.Fatalf("session_ready %d times, want 2 (model changed)", len(ready))
	}
	if m := decode[core.SessionReady](t, ready[1]).Model; m != "claude-sonnet-5-5" {
		t.Errorf("model after switch = %s", m)
	}
	notices := ofKind(evs, core.EvNotice)
	if len(notices) == 0 || decode[core.Notice](t, notices[0]).Kind != core.NoticeModelChanged {
		t.Errorf("expected a model_changed notice")
	}
}

func TestSlashCommands(t *testing.T) {
	evs := parseFixture(t, "slash-commands")
	kinds := map[string]bool{}
	for _, ev := range ofKind(evs, core.EvNotice) {
		kinds[decode[core.Notice](t, ev).Kind] = true
	}
	for _, k := range []string{core.NoticeCompacted, core.NoticeConversationReset} {
		if !kinds[k] {
			t.Errorf("missing %s notice", k)
		}
	}
	synthetic := 0
	for _, ev := range ofKind(evs, core.EvTextBlock) {
		if decode[core.TextBlock](t, ev).Synthetic {
			synthetic++
		}
	}
	if synthetic < 4 {
		t.Errorf("synthetic text blocks = %d", synthetic)
	}
	ready := ofKind(evs, core.EvSessionReady)
	first := decode[core.SessionReady](t, ready[0]).ProviderSID
	last := decode[core.SessionReady](t, ready[len(ready)-1]).ProviderSID
	if first == last {
		t.Error("/clear should surface a new session id")
	}
}

func TestPermissionDeniedReportedOnce(t *testing.T) {
	for _, name := range []string{"perm-prompts-none", "perm-mode-dontask", "perm-host-no-handler"} {
		evs := parseFixture(t, name)
		byID := map[string]int{}
		denied := 0
		for _, ev := range ofKind(evs, core.EvToolFinished) {
			tf := decode[core.ToolFinished](t, ev)
			byID[tf.ID]++
			if tf.Denied {
				denied++
			}
		}
		if denied == 0 {
			t.Errorf("%s: no denial reported", name)
		}
		for id, n := range byID {
			if n != 1 {
				t.Errorf("%s: tool %s finished %d times", name, id, n)
			}
		}
	}
}

func TestToolUseAndFileTouched(t *testing.T) {
	evs := parseFixture(t, "perm-accept-edits-bash-allowlist")
	started := ofKind(evs, core.EvToolStarted)
	if len(started) != 3 {
		t.Fatalf("tool_started = %d", len(started))
	}
	if s := decode[core.ToolStarted](t, started[0]); s.Name != "Write" || s.Summary != "Write notes.md" {
		t.Errorf("first tool = %+v", s)
	}
	if s := decode[core.ToolStarted](t, started[1]).Summary; s != "Bash git --version" {
		t.Errorf("bash summary = %q", s)
	}
	ft := ofKind(evs, core.EvFileTouched)
	if len(ft) != 1 || decode[core.FileTouched](t, ft[0]).How != "write" {
		t.Errorf("file_touched = %v", ft)
	}
	fin := ofKind(evs, core.EvToolFinished)
	if !decode[core.ToolFinished](t, fin[0]).OK {
		t.Error("Write should have succeeded under acceptEdits")
	}
}

// An Edit is a touched file once it has succeeded, at the first line it
// changed (from its structuredPatch); the Read before it isn't.
func TestEditTouchesFileAtChangedLine(t *testing.T) {
	evs := parseFixture(t, "edit-tool-result")
	ft := ofKind(evs, core.EvFileTouched)
	if len(ft) != 1 {
		t.Fatalf("file_touched = %d events", len(ft))
	}
	f := decode[core.FileTouched](t, ft[0])
	if f.How != "edit" || f.Line != 6 || f.Added != 1 || f.Removed != 1 || !strings.HasSuffix(f.Path, "notes.txt") {
		t.Errorf("file_touched = %+v", f)
	}
	// It comes after the tool's finish, not with its start.
	var finishedAt, touchedAt int
	for i, ev := range evs {
		switch ev.Kind {
		case core.EvToolFinished:
			finishedAt = i
		case core.EvFileTouched:
			touchedAt = i
		}
	}
	if touchedAt < finishedAt {
		t.Error("file_touched came before the edit finished")
	}
}

// A write that was denied touched nothing.
func TestDeniedWriteTouchesNothing(t *testing.T) {
	for _, name := range []string{"perm-stdio-deny", "perm-prompts-none", "perm-mode-dontask"} {
		if ft := ofKind(parseFixture(t, name), core.EvFileTouched); len(ft) != 0 {
			t.Errorf("%s: file_touched = %v", name, ft)
		}
	}
}

func TestPatchStats(t *testing.T) {
	cases := map[string][3]int{ // line, added, removed
		`{"structuredPatch":[{"newStart":3,"lines":[" a"," b"," c","-x","+X"]}]}`:                                          {6, 1, 1},
		`{"structuredPatch":[{"newStart":1,"lines":["+new first line"," a"]}]}`:                                            {1, 1, 0},
		`{"structuredPatch":[{"newStart":2,"lines":[" a","-b","-c"," d"]},{"newStart":40,"lines":[" x","+y","+z","+w"]}]}`: {3, 3, 2},
		`{"type":"create","content":"one\ntwo\nthree\n","structuredPatch":[]}`:                                             {0, 3, 0},
		`{"type":"create","content":"no newline"}`:                                                                         {0, 1, 0},
		`{"structuredPatch":[]}`: {0, 0, 0},
		`not json`:               {0, 0, 0},
	}
	for in, want := range cases {
		line, added, removed := patchStats(json.RawMessage(in))
		if got := [3]int{line, added, removed}; got != want {
			t.Errorf("patchStats(%s) = %v, want %v", in, got, want)
		}
	}
}

// A file a tool created counts every line as added (fixture: Write "hi").
func TestCreatedFileCountsItsLines(t *testing.T) {
	ft := ofKind(parseFixture(t, "perm-stdio-allow"), core.EvFileTouched)
	if len(ft) != 1 {
		t.Fatalf("file_touched = %d", len(ft))
	}
	if f := decode[core.FileTouched](t, ft[0]); f.How != "write" || f.Added != 1 || f.Removed != 0 {
		t.Errorf("file_touched = %+v", f)
	}
}

// mcp_status, sent mid-turn, comes back with each MCP tool's
// annotations; the CLI asks about every MCP tool call, read-only or not.
func TestToolHints(t *testing.T) {
	evs := parseFixture(t, "mcp-status-tool-hints")
	hints := ofKind(evs, core.EvToolHints)
	if len(hints) != 1 {
		t.Fatalf("tool_hints = %d", len(hints))
	}
	got := decode[core.ToolHints](t, hints[0]).Tools
	want := map[string]core.ToolHint{
		"mcp__notes__read_note":   {ReadOnly: true, Server: "notes", ServerVersion: "1.0.0"},
		"mcp__notes__delete_note": {Destructive: true, Server: "notes", ServerVersion: "1.0.0"},
		"mcp__notes__lookup_web":  {ReadOnly: true, OpenWorld: true, Server: "notes", ServerVersion: "1.0.0"},
		"mcp__notes__touch_note":  {Server: "notes", ServerVersion: "1.0.0"},
	}
	if len(got) != len(want) {
		t.Fatalf("hints = %+v", got)
	}
	for name, h := range want {
		if g, ok := got[name]; !ok || g != h {
			t.Errorf("%s = %+v (%v), want %+v", name, g, ok, h)
		}
	}
	var asked []string
	for _, e := range ofKind(evs, core.EvApprovalAsked) {
		asked = append(asked, decode[core.ApprovalAsked](t, e).Tool)
	}
	if strings.Join(asked, ",") != "mcp__notes__read_note,mcp__notes__delete_note" {
		t.Errorf("asked about %v", asked)
	}
	if hints := toolHints([]mcpServer{{Name: "claude.ai Gmail"}}); hints.Tools == nil {
		t.Error("no tools should still give a map")
	}
}

// set_permission_mode switches acceptEdits to default on a live process:
// the first Write runs without asking, the second is asked about.
func TestSetPermissionModeLive(t *testing.T) {
	evs := parseFixture(t, "set-permission-mode-live")
	asks := ofKind(evs, core.EvApprovalAsked)
	if len(asks) != 3 {
		t.Fatalf("approval_asked = %d, want 3 (Write after the switch, two MCP tools)", len(asks))
	}
	if a := decode[core.ApprovalAsked](t, asks[0]); a.Tool != "Write" || !strings.Contains(string(a.Input), "b.txt") {
		t.Errorf("first ask = %+v", a)
	}
	if len(ofKind(evs, core.EvError)) != 0 {
		t.Error("the mode switch was answered with an error")
	}
}

func TestApprovalAndAccount(t *testing.T) {
	evs := parseFixture(t, "perm-stdio-allow")
	asks := ofKind(evs, core.EvApprovalAsked)
	if len(asks) != 1 {
		t.Fatalf("approval_asked = %d", len(asks))
	}
	if a := decode[core.ApprovalAsked](t, asks[0]); a.Tool != "Write" || a.RequestID == "" || !strings.HasPrefix(a.ToolUseID, "toolu_") ||
		a.Action.Kind != core.ActWrite || !strings.HasSuffix(a.Action.Path, "hello.txt") {
		t.Errorf("approval = %+v", a)
	}
	acc := ofKind(evs, core.EvAccount)
	if len(acc) != 1 {
		t.Fatalf("account = %d", len(acc))
	}
	a := decode[core.Account](t, acc[0])
	found := false
	for _, m := range a.Models {
		if m.Value == "sonnet" && m.DisplayName != "" && len(m.EffortLevels) > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("sonnet missing from models: %+v", a.Models)
	}
}

func TestErrors(t *testing.T) {
	for name, code := range map[string]string{"error-not-logged-in": "authentication_failed", "error-bad-model": "model_not_found"} {
		rs := results(t, parseFixture(t, name))
		if len(rs) != 1 || !rs[0].IsError || rs[0].ErrorCode != code {
			t.Errorf("%s: result = %+v, want error %s", name, rs, code)
		}
	}
}

func TestUsageLimit(t *testing.T) {
	evs := ofKind(parseFixture(t, "single-turn"), core.EvUsageLimit)
	if len(evs) != 1 {
		t.Fatalf("usage_limit = %d", len(evs))
	}
	u := decode[core.UsageLimit](t, evs[0])
	if _, ok := u.Windows["five_hour"]; !ok || u.Status != "allowed" {
		t.Errorf("usage = %+v", u)
	}
}

func TestUnknownAndGarbageLines(t *testing.T) {
	p := newParser()
	for _, l := range []string{`{"type":"brand_new_thing","x":1}`, `not json at all`, `{"type":"system","subtype":"mystery"}`} {
		evs, err := p.Feed([]byte(l))
		if err != nil || len(evs) != 1 || evs[0].Kind != core.EvUnknown || string(evs[0].Raw) != l {
			t.Errorf("%q -> %v, %v", l, evs, err)
		}
	}
	if evs, err := p.Feed([]byte("  ")); err != nil || len(evs) != 0 {
		t.Errorf("blank line -> %v, %v", evs, err)
	}
}

func TestTruncateKeepsUTF8(t *testing.T) {
	s := strings.Repeat("é", maxToolOutput)
	out := truncate(s)
	if !strings.HasSuffix(out, "…") || strings.ContainsRune(strings.TrimSuffix(out, "…"), '�') {
		t.Errorf("bad truncation")
	}
	for _, r := range strings.TrimSuffix(out, "…") {
		if r != 'é' {
			t.Fatalf("split rune: %q", r)
		}
	}
}

func TestActionOf(t *testing.T) {
	cases := []struct {
		tool, input string
		want        core.ToolAction
	}{
		{"PowerShell", `{"command":"npm test"}`, core.ToolAction{Kind: core.ActShell, Tool: "PowerShell", Dialect: "powershell", Command: "npm test"}},
		{"Bash", `{"command":"ls"}`, core.ToolAction{Kind: core.ActShell, Tool: "Bash", Dialect: "bash", Command: "ls"}},
		{"Edit", `{"file_path":"a.go"}`, core.ToolAction{Kind: core.ActEdit, Tool: "Edit", Path: "a.go"}},
		{"NotebookEdit", `{"notebook_path":"n.ipynb"}`, core.ToolAction{Kind: core.ActEdit, Tool: "NotebookEdit", Path: "n.ipynb"}},
		{"mcp__notes__read_note", `{}`, core.ToolAction{Kind: core.ActMCP, Tool: "mcp__notes__read_note"}},
		{"AskUserQuestion", `{}`, core.ToolAction{Kind: core.ActQuestion, Tool: "AskUserQuestion"}},
		{"SubagentHandback", `{}`, core.ToolAction{Kind: core.ActInternal, Tool: "SubagentHandback"}},
		{"CronCreate", `{}`, core.ToolAction{Kind: core.ActOther, Tool: "CronCreate"}},
	}
	for _, c := range cases {
		got := ActionOf(c.tool, json.RawMessage(c.input))
		got.Input = nil
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %+v, want %+v", c.tool, got, c.want)
		}
	}
}

// A background sub-agent shows as a background task until it finishes;
// the task lines themselves aren't unknown (fixture subagent-background).
func TestBackgroundTasks(t *testing.T) {
	evs := parseFixture(t, "subagent-background")
	bg := ofKind(evs, core.EvBackground)
	if len(bg) != 2 {
		t.Fatalf("background events = %d", len(bg))
	}
	if first := decode[core.BackgroundTasks](t, bg[0]); len(first.Tasks) != 1 || first.Tasks[0].Description != "Simple ping response" || first.Tasks[0].Kind != "local_agent" {
		t.Errorf("first = %+v", first)
	}
	if last := decode[core.BackgroundTasks](t, bg[1]); len(last.Tasks) != 0 {
		t.Errorf("after it finished = %+v", last)
	}
	for _, ev := range ofKind(evs, core.EvUnknown) {
		if u := decode[core.Unknown](t, ev); strings.HasPrefix(u.Type, "system/task_") {
			t.Errorf("%s reported as unknown", u.Type)
		}
	}
	if n := len(results(t, evs)); n != 2 {
		t.Errorf("results = %d, want the user's turn and the one the CLI started", n)
	}
}
