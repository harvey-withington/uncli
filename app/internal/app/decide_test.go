package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/core"
	"uncli/internal/session"
)

// The quick-task model as a decider: one structured call per round, the
// questions' choices as the schema's enums, answers taken as given.
func TestQuickDecider(t *testing.T) {
	var asked core.TextTask
	answer := `{"level":"risky","risk":"publishes","reason":"Uploads the report."}`
	q := quickDecider{model: ModelRef{"claude", "haiku"}, run: func(_ context.Context, task core.TextTask) (core.TextResult, ModelRef, error) {
		asked = task
		return core.TextResult{Structured: json.RawMessage(answer)}, ModelRef{"claude", "haiku"}, nil
	}}
	if info := q.Info(); info.Calibrated || info.Key() != "quick-task/haiku@" {
		t.Errorf("info = %+v, key %s", info, info.Key())
	}
	d := core.Decision{Instructions: judgeSystem, State: "The assistant wants to run: frob sync", Questions: judgeQuestions, Explain: judgeExplain}
	got, err := q.Decide(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if got.Answers["level"] != (core.Answer{Choice: "risky", Probability: 1}) || got.Answers["risk"].Choice != "publishes" || got.Reason != "Uploads the report." {
		t.Errorf("decided = %+v", got)
	}
	var schema struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(asked.Schema, &schema); err != nil || len(schema.Properties["level"].Enum) != 3 || len(schema.Required) != 3 {
		t.Errorf("schema = %s", asked.Schema)
	}
	if asked.System != judgeSystem || !strings.Contains(asked.Prompt, "frob sync") || !strings.Contains(asked.Prompt, `Answer "reason"`) {
		t.Errorf("prompt = %q", asked.Prompt)
	}

	answer = `{"level":"probably fine","risk":"none","reason":"x"}`
	if _, err := q.Decide(context.Background(), d); err == nil {
		t.Error("an answer outside the choices must be refused")
	}
}

// A calibrated decider must be sure enough that something is safe; an
// uncalibrated one is taken at its word.
func TestJudgementFrom(t *testing.T) {
	calibrated := core.DeciderInfo{ID: "systemone", Model: "jev", Version: "1.13", Calibrated: true}
	quick := core.DeciderInfo{ID: DeciderQuickTask, Model: "haiku"}
	answer := func(level string, p float64, risk string) core.Decided {
		return core.Decided{Answers: map[string]core.Answer{"level": {Choice: level, Probability: p}, "risk": {Choice: risk, Probability: p}}}
	}
	cases := []struct {
		info  core.DeciderInfo
		dec   core.Decided
		level string
		risk  string
	}{
		{calibrated, answer(session.RiskRoutine, 0.97, "none"), session.RiskRoutine, ""},
		{calibrated, answer(session.RiskRoutine, 0.7, "none"), session.RiskRisky, session.WhyUnsure}, // the grey zone prompts
		{calibrated, answer(session.RiskLooks, 0.85, "none"), session.RiskRisky, session.WhyUnsure},
		{calibrated, answer(session.RiskRisky, 0.6, "deletes"), session.RiskRisky, session.WhyDeletes},
		{quick, answer(session.RiskRoutine, 1, "none"), session.RiskRoutine, ""},
		{quick, answer(session.RiskRisky, 1, "none"), session.RiskRisky, session.WhySystem},
	}
	for _, c := range cases {
		j, err := judgementFrom(c.info, c.dec, DefaultThreshold)
		if err != nil || j.Level != c.level || j.Risk != c.risk || j.Decider != c.info.Key() {
			t.Errorf("%s %+v: %+v, %v", c.info.ID, c.dec.Answers["level"], j, err)
		}
	}
	if _, err := judgementFrom(quick, answer("maybe", 1, "none"), DefaultThreshold); err == nil {
		t.Error("an unknown level must be refused")
	}
}

// The decision model preference: the quick-task model by default; others
// aren't available yet, and thresholds must make sense.
func TestDecisionModelPreference(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if p := svc.Preferences(); p.DecisionModel.Provider != DeciderQuickTask || p.DecisionModel.threshold() != DefaultThreshold {
		t.Errorf("default = %+v", p.DecisionModel)
	}
	if k := svc.deciderKey(); k != "quick-task/haiku@" {
		t.Errorf("decider key = %q", k)
	}
	base := svc.Preferences()
	for _, m := range []DecisionModel{{Provider: "systemone"}, {Provider: DeciderQuickTask, Threshold: 0.3}} {
		p := base
		p.DecisionModel = m
		if _, err := svc.SetPreferences(p); err == nil {
			t.Errorf("%+v must be refused", m)
		}
	}
	// A stored preference the app can't use falls back to the default.
	if err := svc.Store.SetSetting(settingPrefs, `{"quickTaskModel":{"provider":"claude","model":"haiku"},"decisionModel":{"provider":"systemone"}}`); err != nil {
		t.Fatal(err)
	}
	if p := svc.Preferences(); p.DecisionModel.Provider != DeciderQuickTask {
		t.Errorf("fallback = %+v", p.DecisionModel)
	}
}
