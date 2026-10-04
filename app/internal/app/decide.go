package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"uncli/internal/core"
)

// Deciding: an app-level decision model answers fixed-answer questions,
// shared by every session (decision record 0006). The default, and for now
// the only one, is the quick-task model asked for structured output. It is
// uncalibrated, so its answers are taken as given. A calibrated decider
// (Jev, Kev) gives each answer a probability, and the threshold says how
// sure "safe" must be; below it the answer counts as "not sure" and prompts.

// DeciderQuickTask is the quick-task model used as a decider.
const DeciderQuickTask = "quick-task"

// DefaultThreshold is how sure a calibrated decider must be that something
// is safe.
const DefaultThreshold = 0.9

// DecisionModel is the decision model preference.
type DecisionModel struct {
	Provider  string  `json:"provider"`            // quick-task (the default)
	Endpoint  string  `json:"endpoint,omitempty"`  // a decider's base URL
	Model     string  `json:"model,omitempty"`     // the decider's model
	Version   string  `json:"version,omitempty"`   // pinned: thresholds are tuned per version
	LocalOnly bool    `json:"localOnly,omitempty"` // only a decider on this computer or network
	Threshold float64 `json:"threshold,omitempty"` // how sure "safe" must be; 0 is DefaultThreshold
}

func (m DecisionModel) threshold() float64 {
	if m.Threshold <= 0 || m.Threshold > 1 {
		return DefaultThreshold
	}
	return m.Threshold
}

func validDecisionModel(m DecisionModel) error {
	switch m.Provider {
	case "", DeciderQuickTask:
	default:
		return fmt.Errorf("the decision model %q isn't available yet", m.Provider)
	}
	if m.Threshold != 0 && (m.Threshold < 0.5 || m.Threshold > 1) {
		return errors.New("the decision threshold must be between 0.5 and 1")
	}
	return nil
}

// decider is the app's decision model.
func (s *Service) decider() (core.Decider, error) {
	p := s.Preferences()
	if err := validDecisionModel(p.DecisionModel); err != nil {
		return nil, err
	}
	return quickDecider{run: s.RunTextTask, model: p.QuickTaskModel}, nil
}

// deciderKey identifies the current decision model, so answers cached from
// another one are asked again; empty when there is none.
func (s *Service) deciderKey() string {
	d, err := s.decider()
	if err != nil {
		return ""
	}
	return d.Info().Key()
}

// quickDecider asks the quick-task model, one structured answer per round.
type quickDecider struct {
	run   func(context.Context, core.TextTask) (core.TextResult, ModelRef, error)
	model ModelRef
}

func (q quickDecider) Info() core.DeciderInfo {
	return core.DeciderInfo{ID: DeciderQuickTask, Model: q.model.Model}
}

func (q quickDecider) Decide(ctx context.Context, d core.Decision) (core.Decided, error) {
	props := map[string]any{}
	var required []string
	var b strings.Builder
	b.WriteString(strings.TrimSpace(d.State))
	b.WriteString("\n")
	for _, qn := range d.Questions {
		values := make([]string, len(qn.Choices))
		fmt.Fprintf(&b, "\nAnswer %q: %s\n", qn.ID, qn.Text)
		for i, c := range qn.Choices {
			values[i] = c.Value
			fmt.Fprintf(&b, "- %q: %s\n", c.Value, c.Meaning)
		}
		props[qn.ID] = map[string]any{"type": "string", "enum": values}
		required = append(required, qn.ID)
	}
	if d.Explain != "" {
		props["reason"] = map[string]any{"type": "string"}
		required = append(required, "reason")
		fmt.Fprintf(&b, "\nAnswer \"reason\": %s\n", d.Explain)
	}
	schema, err := json.Marshal(map[string]any{"type": "object", "properties": props, "required": required})
	if err != nil {
		return core.Decided{}, err
	}
	res, _, err := q.run(ctx, core.TextTask{System: d.Instructions, Prompt: b.String(), Schema: schema})
	if err != nil {
		return core.Decided{}, err
	}
	raw := res.Structured
	if len(raw) == 0 {
		raw = json.RawMessage(res.Text)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		return core.Decided{}, errors.New("the model's answer wasn't the expected JSON")
	}
	out := core.Decided{Answers: map[string]core.Answer{}}
	for _, qn := range d.Questions {
		v, _ := got[qn.ID].(string)
		if !hasChoice(qn.Choices, v) {
			return core.Decided{}, fmt.Errorf("the model gave an unknown answer %q to %q", v, qn.ID)
		}
		out.Answers[qn.ID] = core.Answer{Choice: v, Probability: 1}
	}
	out.Reason, _ = got["reason"].(string)
	return out, nil
}

func hasChoice(cs []core.Choice, v string) bool {
	for _, c := range cs {
		if c.Value == v {
			return true
		}
	}
	return false
}
