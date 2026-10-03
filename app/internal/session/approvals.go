package session

import (
	"encoding/json"
	"errors"
	"time"

	"uncli/internal/core"
	"uncli/internal/store"
)

// Approvals: with the adapter's Approvals capability, the CLI asks before
// any tool use its allowlist doesn't cover, and waits for the answer. The
// project's rules (rules.go) answer it when one matches (allow or deny);
// otherwise the session keeps it until the user answers on its card
// (Allow, Always allow…, Deny). Whatever is still waiting when the turn is
// stopped is denied.
//
// Unattended mode is for when the user walks away: rules still answer
// first, but anything that would wait for the user is declined at once,
// with a message telling the model why and how to carry on without it.

// Approval is a tool use waiting for the user's answer.
type Approval struct {
	RequestID   string           `json:"requestId"`
	Tool        string           `json:"tool"`
	Input       json.RawMessage  `json:"input,omitempty"`
	Description string           `json:"description,omitempty"`
	ToolUseID   string           `json:"toolUseId,omitempty"`
	Suggestions []store.ToolRule `json:"suggestions"` // what "Always allow" can add, most specific first
	AskedAt     int64            `json:"askedAt"`
}

// Decisions the user can make on an approval card.
const (
	Allow        = "allow"   // this once
	AllowSession = "session" // and add a rule for the rest of this session
	Always       = "always"  // and add a rule to the project
	Deny         = "deny"
)

const (
	deniedByUser = "Denied by the user in UNCLI."
	deniedByRule = "Denied by a rule for this project in UNCLI."

	unattendedDeclined = "Declined automatically: the user is away (UNCLI unattended mode) and no rule allows this. " +
		"Don't retry it as it is. Use tools and commands that are already allowed; run chained or piped commands " +
		"as separate commands, since each part needs a rule of its own. If the task can't be finished without " +
		"this, finish what you can and list what still needs the user at the end of your answer."
	unattendedQuestion = "The user is away (UNCLI unattended mode) and can't answer questions. Make the most " +
		"reasonable choice yourself, carry on, and say at the end of your answer which choices you made."
	unattendedPlan = "The user is away (UNCLI unattended mode) and can't approve a plan. Give the plan in your " +
		"answer and stop there; the user will review it when they're back."

	// Told to the model at the start of a turn when the mode has changed.
	unattendedOn = "Unattended mode is on: the user is away. Any tool use or command that would need their " +
		"approval is declined automatically, and questions to them can't be answered. Prefer tools and commands " +
		"that are already allowed, run chained or piped commands as separate commands, make reasonable choices " +
		"yourself, and end your answer with what you decided and anything that still needs the user."
	unattendedOff = "Unattended mode is off: the user is back and can approve tool uses and answer questions again."
)

// unattendedMessage is the reason given for a request declined in unattended mode.
func unattendedMessage(tool string) string {
	switch tool {
	case "AskUserQuestion":
		return unattendedQuestion
	case "ExitPlanMode":
		return unattendedPlan
	}
	return unattendedDeclined
}

func bashCommand(input json.RawMessage) string {
	var in struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(input, &in)
	return in.Command
}

// askLocked handles a permission request from the CLI. Caller holds s.mu.
// It reports whether the request now waits for the user.
func (s *Session) askLocked(a core.ApprovalAsked) bool {
	if s.page == nil || s.proc == nil {
		_ = s.replyLocked(a.RequestID, false, nil, "There is no turn to approve this for.")
		return false
	}
	rules, _ := s.m.d.Store.Rules(s.rec.Workdir)
	action, from := decide(s.sessionRules, rules, a.Tool, a.Input)
	switch action {
	case store.RuleAllow:
		_ = s.replyLocked(a.RequestID, true, a.Input, "")
		s.markApprovedLocked(a.ToolUseID, from)
		return false
	case store.RuleDeny:
		_ = s.replyLocked(a.RequestID, false, nil, deniedByRule)
		s.markDeniedLocked(a.ToolUseID)
		return false
	}
	if s.unattended {
		_ = s.replyLocked(a.RequestID, false, nil, unattendedMessage(a.Tool))
		s.markDeniedLocked(a.ToolUseID)
		return false
	}
	sug := suggestions(a.Tool, a.Input)
	if sug == nil {
		sug = []store.ToolRule{}
	}
	s.pending = append(s.pending, Approval{
		RequestID: a.RequestID, Tool: a.Tool, Input: a.Input, Description: a.Description, ToolUseID: a.ToolUseID,
		Suggestions: sug, AskedAt: time.Now().UnixMilli(),
	})
	return true
}

// Answer gives the user's decision on a waiting request. For Always, rule
// is the rule to add (one of the request's suggestions, possibly edited).
func (s *Session) Answer(requestID, decision string, rule *store.ToolRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := -1
	for k, a := range s.pending {
		if a.RequestID == requestID {
			i = k
		}
	}
	if i < 0 {
		return errors.New("that request is no longer waiting for an answer")
	}
	a := s.pending[i]
	allow := decision == Allow || decision == AllowSession || decision == Always
	if !allow && decision != Deny {
		return errors.New("unknown decision " + decision)
	}
	if decision == Always || decision == AllowSession {
		if rule == nil || rule.Tool != a.Tool {
			return errors.New("choose what to always allow")
		}
		r := *rule
		r.Action = store.RuleAllow
		if decision == Always {
			if err := s.m.d.Store.SetRule(s.rec.Workdir, r); err != nil {
				return err
			}
		} else {
			s.sessionRules = addRule(s.sessionRules, r)
		}
	}
	if err := s.replyLocked(a.RequestID, allow, a.Input, deniedByUser); err != nil {
		return err
	}
	s.pending = append(s.pending[:i:i], s.pending[i+1:]...)
	if allow {
		s.markApprovedLocked(a.ToolUseID, "you")
	} else {
		s.markDeniedLocked(a.ToolUseID)
	}
	s.resumeLocked()
	s.changed()
	return nil
}

// resumeLocked goes back to work once nothing waits for the user.
func (s *Session) resumeLocked() {
	if len(s.pending) == 0 && s.state == NeedsApproval {
		s.state = Thinking
		if s.runningTools > 0 {
			s.state = RunningTools
		}
	}
}

// SetUnattended turns unattended mode on or off. Turning it on declines
// whatever is already waiting, so the user can walk away from a card.
func (s *Session) SetUnattended(on bool) View {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unattended = on
	if on {
		for _, a := range s.pending {
			_ = s.replyLocked(a.RequestID, false, nil, unattendedMessage(a.Tool))
			s.markDeniedLocked(a.ToolUseID)
		}
		s.pending = nil
		s.resumeLocked()
	}
	s.changed()
	return s.viewLocked()
}

// unattendedDirective is what the next turn tells the model about the
// mode, when it has changed since the model was last told. Caller holds s.mu.
func (s *Session) unattendedDirective() []string {
	if s.unattended == s.toldUnattended {
		return nil
	}
	if s.unattended {
		return []string{unattendedOn}
	}
	return []string{unattendedOff}
}

// SessionRules are the rules for this session only, kept until UNCLI quits.
func (s *Session) SessionRules() []store.ToolRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.ToolRule{}, s.sessionRules...)
}

// RemoveSessionRule drops a session rule.
func (s *Session) RemoveSessionRule(r store.ToolRule) []store.ToolRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.sessionRules[:0:0]
	for _, x := range s.sessionRules {
		if x.Tool != r.Tool || x.Prefix != r.Prefix {
			out = append(out, x)
		}
	}
	s.sessionRules = out
	return append([]store.ToolRule{}, out...)
}

// addRule adds a rule, replacing one for the same tool and prefix.
func addRule(rules []store.ToolRule, r store.ToolRule) []store.ToolRule {
	for i, x := range rules {
		if x.Tool == r.Tool && x.Prefix == r.Prefix {
			rules[i] = r
			return rules
		}
	}
	return append(rules, r)
}

// markApprovedLocked records on the trace who let a tool use run.
func (s *Session) markApprovedLocked(toolUseID, by string) {
	if s.page == nil || toolUseID == "" {
		return
	}
	for k := range s.page.Trace {
		if s.page.Trace[k].ID == toolUseID {
			s.page.Trace[k].Approved = by
		}
	}
	s.savePage()
}

func (s *Session) markDeniedLocked(toolUseID string) {
	if s.page == nil || toolUseID == "" {
		return
	}
	for k := range s.page.Trace {
		if s.page.Trace[k].ID == toolUseID {
			s.page.Trace[k].Denied = true
		}
	}
	s.savePage()
}

// denyPendingLocked answers every waiting request with a denial (the turn
// is being stopped). Caller holds s.mu.
func (s *Session) denyPendingLocked(reason string) {
	for _, a := range s.pending {
		_ = s.replyLocked(a.RequestID, false, nil, reason)
	}
	s.pending = nil
}

func (s *Session) replyLocked(requestID string, allow bool, input json.RawMessage, message string) error {
	b, ok := s.m.d.Adapter.EncodeControl(core.Control{Kind: core.CtlApprove, RequestID: requestID, Allow: allow, UpdatedInput: input, Message: message})
	if !ok || s.proc == nil {
		return errors.New("the CLI isn't running")
	}
	_, err := s.proc.Stdin().Write(b)
	return err
}
