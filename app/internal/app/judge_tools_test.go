package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"uncli/internal/core"
	"uncli/internal/session"
)

// toolDecider answers by tool: "read_*" reads, "delete_*" is risky, the
// rest safe. It records each decision it was asked.
type toolDecider struct {
	calibrated bool
	asked      []core.Decision
}

func (d *toolDecider) Info() core.DeciderInfo {
	return core.DeciderInfo{ID: "fake", Model: "m", Version: "1", Calibrated: d.calibrated}
}

func (d *toolDecider) Decide(_ context.Context, dec core.Decision) (core.Decided, error) {
	d.asked = append(d.asked, dec)
	out := core.Decided{Answers: map[string]core.Answer{}}
	for _, q := range dec.Questions {
		kind, i, _ := strings.Cut(q.ID, ".")
		var name string
		_, _ = fmt.Sscanf(q.Text, "What does the tool %q do?", &name)
		if kind == "risk" {
			_, _ = fmt.Sscanf(q.Text, "If %q is risky", &name)
		}
		level, risk := session.RiskRoutine, "none"
		switch {
		case strings.HasPrefix(name, "read_"):
			level = session.RiskLooks
		case strings.HasPrefix(name, "delete_"):
			level, risk = session.RiskRisky, session.WhyDeletes
		}
		if kind == "level" {
			out.Answers[q.ID] = core.Answer{Choice: level, Probability: 0.97}
		} else {
			out.Answers[q.ID] = core.Answer{Choice: risk, Probability: 0.9}
		}
		_ = i
	}
	return out, nil
}

func TestJudgeToolsInOneDecisionPerBatch(t *testing.T) {
	d := &toolDecider{}
	tools := []string{"mcp__notes__read_note", "mcp__notes__delete_note", "mcp__notes__touch_note"}
	got, err := judgeToolsWith(context.Background(), d, 0.9, session.ToolsQuery{Server: "notes", Version: "1.0.0", Tools: tools, Workdir: "/w"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.asked) != 1 {
		t.Fatalf("asked %d times, want once for the server", len(d.asked))
	}
	state := d.asked[0].State
	if !strings.Contains(state, `"notes" (version 1.0.0)`) || !strings.Contains(state, "- touch_note") || strings.Contains(state, "mcp__") {
		t.Errorf("state = %q", state)
	}
	if got["mcp__notes__read_note"].Level != session.RiskLooks || got["mcp__notes__touch_note"].Level != session.RiskRoutine ||
		got["mcp__notes__delete_note"].Level != session.RiskRisky || got["mcp__notes__delete_note"].Risk != session.WhyDeletes {
		t.Errorf("judgements = %+v", got)
	}
	// A calibrated decider not sure enough that a tool is safe: unsafe, "not sure".
	cal := &toolDecider{calibrated: true}
	strict, _ := judgeToolsWith(context.Background(), cal, 0.99, session.ToolsQuery{Server: "notes", Tools: tools[2:]})
	if j := strict["mcp__notes__touch_note"]; j.Level != session.RiskRisky || j.Risk != session.WhyUnsure {
		t.Errorf("below threshold = %+v", j)
	}
	// Many tools: several rounds of at most maxToolsPerDecision.
	many := make([]string, 45)
	for i := range many {
		many[i] = fmt.Sprintf("mcp__big__t%02d", i)
	}
	big := &toolDecider{}
	all, _ := judgeToolsWith(context.Background(), big, 0.9, session.ToolsQuery{Server: "big", Tools: many})
	if len(big.asked) != 3 || len(all) != 45 {
		t.Errorf("rounds = %d, judged %d", len(big.asked), len(all))
	}
}
