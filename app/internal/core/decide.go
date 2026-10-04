package core

import "context"

// Decider is an app-level helper model that answers fixed-answer questions
// about a state, with a probability for each answer: the LLM writes, the
// decider decides, code acts. It belongs to the app, not to a session, and
// may be a CLI or an API (decision record 0006). UNCLI's first question is
// whether a command is safe to run without asking.
type Decider interface {
	Info() DeciderInfo
	Decide(ctx context.Context, d Decision) (Decided, error)
}

// DeciderInfo says which decider answered, for caching and the trace.
type DeciderInfo struct {
	ID      string // the kind: "quick-task", "systemone"…
	Model   string // the model it asked
	Version string // the pinned version its thresholds are tuned for
	// Calibrated: its probabilities mean what they say, so thresholds apply.
	// A language model asked for structured output isn't.
	Calibrated bool
}

// Key identifies a decider: an answer from another one is asked again.
func (i DeciderInfo) Key() string { return i.ID + "/" + i.Model + "@" + i.Version }

// Decision is one round of questions about one state.
type Decision struct {
	Instructions string // how to judge
	State        string // the facts
	Questions    []Question
	// Explain asks for a one-line reason for a person, when the decider can
	// write one (a language model can; a pure decider leaves it empty).
	Explain string
}

// Question is one fixed-answer question.
type Question struct {
	ID      string
	Text    string
	Choices []Choice
}

// Choice is one possible answer and what it means.
type Choice struct {
	Value   string
	Meaning string
}

// Decided holds an answer for each question, by question id.
type Decided struct {
	Answers map[string]Answer
	Reason  string // when Explain was asked and the decider could write one
}

// Answer is the chosen value and how likely the decider thinks it is: 1
// from an uncalibrated decider.
type Answer struct {
	Choice      string
	Probability float64
}
