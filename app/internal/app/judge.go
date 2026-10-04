package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"uncli/internal/core"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Judging commands UNCLI doesn't recognise: in Ask mode, with the
// "model" setting, the quick-task model says whether a command only looks,
// is routine work in the session's folder, or is risky, with a one-line
// reason for someone who doesn't use the command line. The session layer
// caches the answer (session/judging.go).

var judgeSchema = json.RawMessage(`{"type":"object","properties":{` +
	`"level":{"type":"string","enum":["looks","routine","risky"]},` +
	`"risk":{"type":"string","enum":["none","deletes","discards","outside","publishes","installs","system","stops","remote","secrets","runs-code","cloud"]},` +
	`"reason":{"type":"string"}},"required":["level","risk","reason"]}`)

const judgeSystem = "You judge whether something an AI coding assistant wants to do is safe to let it do without asking " +
	"its user, who does not use the command line and can't read commands. Reply only with the requested JSON."

func (s *Service) judgeCommand(ctx context.Context, q session.JudgeQuery) (store.Judgement, error) {
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
	} else {
		fmt.Fprintf(&b, "The assistant wants to use the tool %q", q.Tool)
		if q.Description != "" {
			fmt.Fprintf(&b, " (%s)", q.Description)
		}
		fmt.Fprintf(&b, ", working in the folder %s. Judge what this tool does in general, not only this call. Input of this call:\n\n%s\n\n", q.Workdir, clip(string(q.Input), 2000))
	}
	b.WriteString(`Answer "level":
- "looks": it only reads or shows information.
- "routine": it only changes files inside that folder, or builds, tests, formats or runs the project's own scripts; easily undone or redone.
- "risky": it could do real harm: deletes or overwrites things for good, changes files outside the folder, sends or publishes anything beyond this computer, installs software, changes system settings or needs admin rights, stops programs, touches passwords or keys, runs code fetched from elsewhere, or changes a cloud account.
If you are unsure, answer "risky".
"risk": for risky, the main way it could do harm (deletes, discards = throws away uncommitted changes, outside, publishes, installs, system, stops, remote = connects to another computer, secrets, runs-code, cloud); otherwise "none".
"reason": one short sentence for a non-technical user saying what it does, without command names, e.g. "Rebuilds the project's documentation." or "Deletes the downloaded packages folder."`)
	res, ref, err := s.RunTextTask(ctx, core.TextTask{System: judgeSystem, Prompt: b.String(), Schema: judgeSchema})
	if err != nil {
		return store.Judgement{}, err
	}
	raw := res.Structured
	if len(raw) == 0 {
		raw = json.RawMessage(res.Text)
	}
	var v struct {
		Level, Risk, Reason string
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return store.Judgement{}, errors.New("the model's answer wasn't the expected JSON")
	}
	j := store.Judgement{Level: v.Level, Note: clip(strings.TrimSpace(v.Reason), 200), Model: ref.Model}
	switch v.Level {
	case session.RiskLooks, session.RiskRoutine:
	case session.RiskRisky:
		j.Risk = v.Risk
		if j.Risk == "none" || j.Risk == "" {
			j.Risk = session.WhySystem
		}
	default:
		return store.Judgement{}, fmt.Errorf("the model gave an unknown level %q", v.Level)
	}
	return j, nil
}
