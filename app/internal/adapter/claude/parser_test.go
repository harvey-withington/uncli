package claude

import (
	"bufio"
	"os"
	"path/filepath"
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

func TestApprovalAndAccount(t *testing.T) {
	evs := parseFixture(t, "perm-stdio-allow")
	asks := ofKind(evs, core.EvApprovalAsked)
	if len(asks) != 1 {
		t.Fatalf("approval_asked = %d", len(asks))
	}
	if a := decode[core.ApprovalAsked](t, asks[0]); a.Tool != "Write" || a.RequestID == "" || !strings.HasPrefix(a.ToolUseID, "toolu_") {
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
