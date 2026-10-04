package app

import (
	"context"
	"fmt"
	"strings"

	"uncli/internal/core"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Judging commands UNCLI can't place: with the "model" setting, the
// decision model says whether a command only reads, is safe work in the
// session's folder, or is unsafe and how, with a one-line reason for
// someone who doesn't use the command line when the model can write one.
// The session layer caches the answer by class and decider
// (session/judging.go).

const judgeSystem = "You judge whether something an AI coding assistant wants to do is safe to let it do without asking " +
	"its user, who does not use the command line and can't read commands. If you are unsure, answer that it is risky. " +
	"Reply only with the requested JSON."

var judgeQuestions = []core.Question{
	{ID: "level", Text: "What does it do?", Choices: []core.Choice{
		{Value: session.RiskLooks, Meaning: "it only reads or shows information"},
		{Value: session.RiskRoutine, Meaning: "it only changes files inside that folder, or builds, tests, formats or runs the project's own scripts; easily undone or redone"},
		{Value: session.RiskRisky, Meaning: "it could do real harm: deletes or overwrites things for good, changes files outside the folder, sends or publishes anything beyond this computer, installs software, changes system settings or needs admin rights, stops programs, touches passwords or keys, runs code fetched from elsewhere, or changes a cloud account"},
	}},
	{ID: "risk", Text: "If risky, the main way it could do harm; otherwise none.", Choices: []core.Choice{
		{Value: "none", Meaning: "not risky"},
		{Value: session.WhyDeletes, Meaning: "deletes or overwrites things for good"},
		{Value: session.WhyDiscards, Meaning: "throws away uncommitted changes"},
		{Value: session.WhyOutside, Meaning: "changes files outside the folder"},
		{Value: session.WhyPublishes, Meaning: "sends or publishes something beyond this computer"},
		{Value: session.WhyInstalls, Meaning: "installs software"},
		{Value: session.WhySystem, Meaning: "changes system settings or needs admin rights"},
		{Value: session.WhyStops, Meaning: "stops programs or services"},
		{Value: session.WhyRemote, Meaning: "connects to another computer"},
		{Value: session.WhySecrets, Meaning: "touches passwords, keys or other secrets"},
		{Value: session.WhyRunsCode, Meaning: "runs code fetched from elsewhere"},
		{Value: session.WhyCloud, Meaning: "changes things in a cloud account"},
	}},
}

const judgeExplain = `one short sentence for a non-technical user saying what it does, without command names, e.g. "Rebuilds the project's documentation." or "Deletes the downloaded packages folder."`

func (s *Service) judgeCommand(ctx context.Context, q session.JudgeQuery) (store.Judgement, error) {
	d, err := s.decider()
	if err != nil {
		return store.Judgement{}, err
	}
	dec, err := d.Decide(ctx, core.Decision{Instructions: judgeSystem, State: judgeState(q), Questions: judgeQuestions, Explain: judgeExplain})
	if err != nil {
		return store.Judgement{}, err
	}
	return judgementFrom(d.Info(), dec, s.Preferences().DecisionModel.threshold())
}

// judgeState is what the decider is told about the command or tool.
func judgeState(q session.JudgeQuery) string {
	var b strings.Builder
	shell := "a POSIX shell"
	if q.Dialect == session.DialectPowerShell {
		shell = "PowerShell"
	}
	if q.Part != "" {
		fmt.Fprintf(&b, "The assistant wants to run this command in %s, in the folder %s:\n\n%s\n\n", shell, q.Workdir, q.Part)
		if q.Code != "" {
			fmt.Fprintf(&b, "It is given this code to run:\n\n%s\n\n", clip(q.Code, 6000))
		}
		if q.Command != "" && q.Command != q.Part {
			fmt.Fprintf(&b, "It is one part of this command line (judge only the part above):\n\n%s\n\n", q.Command)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "The assistant wants to use the tool %q", q.Tool)
	if q.Description != "" {
		fmt.Fprintf(&b, " (%s)", q.Description)
	}
	fmt.Fprintf(&b, ", working in the folder %s. Judge what this tool does in general, not only this call. Input of this call:\n\n%s\n\n", q.Workdir, clip(string(q.Input), 2000))
	return b.String()
}

// judgementFrom turns a decider's answers into a judgement. A calibrated
// decider that says "reads" or "safe" without being sure enough (below the
// threshold) gets "unsafe: not sure", which prompts; an uncalibrated one is
// taken at its word, as it was told to answer "risky" when unsure.
func judgementFrom(info core.DeciderInfo, dec core.Decided, threshold float64) (store.Judgement, error) {
	level, risk := dec.Answers["level"], dec.Answers["risk"]
	j := store.Judgement{Level: level.Choice, Note: clip(strings.TrimSpace(dec.Reason), 200), Model: info.Model,
		Decider: info.Key(), Confidence: level.Probability}
	switch level.Choice {
	case session.RiskLooks, session.RiskRoutine:
		if info.Calibrated && level.Probability < threshold {
			j.Level, j.Risk = session.RiskRisky, session.WhyUnsure
		}
	case session.RiskRisky:
		j.Risk = risk.Choice
		if j.Risk == "none" || j.Risk == "" {
			j.Risk = session.WhySystem
		}
	default:
		return store.Judgement{}, fmt.Errorf("the decision model gave an unknown level %q", level.Choice)
	}
	return j, nil
}
