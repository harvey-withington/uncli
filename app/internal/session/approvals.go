package session

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"uncli/internal/core"
	"uncli/internal/store"
)

// Approvals: with the adapter's Approvals capability, the CLI asks before
// any tool use it wouldn't run on its own, and waits for the answer. The
// session's prompt level and the safe list (modes.go) answer it when they
// can; otherwise the session keeps it until the user answers on its card:
// Allow once, This is safe (remembered for this project or all projects),
// or Deny. Whatever is still waiting when the turn is stopped is denied.
//
// Unattended mode is for when the user walks away: anything that would
// prompt is declined at once, with a message telling the model why and
// how to carry on without it.

// Approval is a tool use waiting for the user's answer.
type Approval struct {
	RequestID   string              `json:"requestId"`
	Tool        string              `json:"tool"`
	Input       json.RawMessage     `json:"input,omitempty"`
	Description string              `json:"description,omitempty"`
	ToolUseID   string              `json:"toolUseId,omitempty"`
	Action      core.ToolAction     `json:"-"`     // what it does, in provider-neutral terms
	Class       Class               `json:"class"` // what UNCLI takes it to do
	Why         []store.TraceReason `json:"why"`   // what each part does, and why it prompts
	// Learn is what "This is safe" would remember: a class for each part
	// that prompts. Empty when it can't be learned (the card says why).
	Learn []store.SafeClass `json:"learn"`
	// Judging: the quick-task model is judging it; it waits without a card.
	Judging bool  `json:"judging,omitempty"`
	AskedAt int64 `json:"askedAt"`
	// Marked: the user toggled "This is safe" on the card, in MarkedScope.
	// The classes are on the safe list, and the card waits for Allow once or
	// Deny instead of answering itself; toggling it off puts back what was
	// there before (prior).
	Marked      bool   `json:"marked,omitempty"`
	MarkedScope string `json:"markedScope,omitempty"`
	prior       []priorEntry
}

// priorEntry is what the safe list held for a class before a card marked
// it safe: an entry (had) to restore, or none, so the mark is removed.
type priorEntry struct {
	at  store.SafeEntry // the class and scope that were marked
	old store.SafeEntry
	had bool
}

// Decisions the user can make on a card.
const (
	Allow = "allow" // this once
	Safe  = "safe"  // and remember it as safe (with a scope)
	Deny  = "deny"
)

// Scopes of a safe-list entry, as the UI names them.
const (
	ScopeProject = "project"
	ScopeAll     = "all"
)

const (
	deniedOnCard = "Denied by the user in UNCLI."

	unattendedDeclined = "Declined automatically: the user is away (UNCLI unattended mode), and this would have needed " +
		"their approval. Don't retry it as it is. Carry on with what doesn't need them; run chained or piped commands " +
		"as separate commands, since the safe parts can then run on their own. If the task can't be finished without " +
		"this, finish what you can and list what still needs the user at the end of your answer."
	unattendedQuestion = "The user is away (UNCLI unattended mode) and can't answer questions. Make the most " +
		"reasonable choice yourself, carry on, and say at the end of your answer which choices you made."
	unattendedPlan = "The user is away (UNCLI unattended mode) and can't approve a plan. Give the plan in your " +
		"answer and stop there; the user will review it when they're back."

	// Told to the model at the start of a turn when the mode has changed.
	unattendedOn = "Unattended mode is on: the user is away. Anything that would need their approval is declined " +
		"automatically, and questions to them can't be answered. Prefer safe work, run chained or piped commands as " +
		"separate commands, make reasonable choices yourself, and end your answer with what you decided and anything " +
		"that still needs the user."
	unattendedOff = "Unattended mode is off: the user is back and can approve tool uses and answer questions again."

	// What the model is told about the prompt level when it changes (or
	// once after a restore). Each line also retires the old read-only mode,
	// so a session told it was read-only stops refusing to try tools.
	levelAlways = "Prompt level: Always. Use tools as you normally would; anything except reading is put to the user " +
		"for approval first. There is no read-only restriction: any earlier read-only instruction no longer applies."
	levelUnsafe = "Prompt level: When unsafe. Use tools as you normally would; safe work runs, and anything unsafe is put " +
		"to the user for approval first. There is no read-only restriction: any earlier read-only instruction no longer applies."
	levelNever = "Prompt level: Never. Tools run without asking the user, except what the user has blocked. There is no " +
		"read-only restriction: any earlier read-only instruction no longer applies."
)

// unattendedMessage is the reason given for a request declined in unattended mode.
func unattendedMessage(a core.ToolAction) string {
	switch a.Kind {
	case core.ActQuestion:
		return unattendedQuestion
	case core.ActPlan:
		return unattendedPlan
	}
	return unattendedDeclined
}

// askLocked handles a permission request from the CLI. Caller holds s.mu.
// It reports whether the request now waits for the user.
func (s *Session) askLocked(a core.ApprovalAsked) bool {
	if s.page == nil || s.proc == nil {
		_ = s.replyLocked(a.RequestID, false, nil, "There is no turn to approve this for.")
		return false
	}
	action := a.Action
	if action.Kind == "" { // an adapter that doesn't say: nothing known about it
		action = core.ToolAction{Kind: core.ActOther, Tool: a.Tool, Input: a.Input}
	}
	v := s.policyLocked().judge(action)
	if s.settleLocked(a.RequestID, a.ToolUseID, action, a.Input, v) {
		return false
	}
	ap := Approval{RequestID: a.RequestID, Tool: a.Tool, Input: a.Input, Description: a.Description, ToolUseID: a.ToolUseID,
		Action: action, AskedAt: time.Now().UnixMilli()}
	ap.update(v)
	s.pending = append(s.pending, ap)
	if v.action == actionJudge {
		s.m.startJudging(v.judge)
	}
	return s.waitingForUserLocked()
}

// update takes a fresh verdict for a waiting request.
func (a *Approval) update(v verdict) {
	a.Class, a.Why, a.Judging = v.class, v.why, v.action == actionJudge
	a.Learn, _ = learnable(v.why)
	if a.Learn == nil {
		a.Learn = []store.SafeClass{}
	}
}

// settleLocked answers a request without the user when the verdict (or
// unattended mode) allows it. It reports whether it did.
func (s *Session) settleLocked(requestID, toolUseID string, a core.ToolAction, input json.RawMessage, v verdict) bool {
	switch {
	case v.action == actionRun:
		_ = s.replyLocked(requestID, true, input, "")
		s.markApprovedLocked(toolUseID, v.by, v.why)
		s.followShellLocked(a)
	case v.action == actionBlock:
		_ = s.replyLocked(requestID, false, nil, v.reason)
		s.markDeniedLocked(toolUseID, v.why)
	case v.action == actionJudge:
		return false // the model's answer decides
	case s.unattended:
		_ = s.replyLocked(requestID, false, nil, unattendedMessage(a))
		s.markDeniedLocked(toolUseID, v.why)
	default:
		return false
	}
	return true
}

// followShellLocked keeps track of the shell a command ran in: its
// dialect, and the folder a cd left it in (the CLI keeps that between
// commands, and goes back to the session folder if it leaves it).
func (s *Session) followShellLocked(a core.ToolAction) {
	if a.Kind != core.ActShell {
		return
	}
	s.shellDialect = a.Dialect
	base := s.shellCwd
	if base == "" {
		base = s.rec.Workdir
	}
	if _, cwd, ok := readScript(a.Command, s.shellDialect, base); ok {
		s.shellCwd = ""
		if p := filepath.ToSlash(filepath.Clean(cwd)); inside(p, s.rec.Workdir) && cwd != s.rec.Workdir {
			s.shellCwd = cwd
		}
	}
}

// policyLocked gathers what a decision depends on. Caller holds s.mu.
func (s *Session) policyLocked() policy {
	safe, _ := s.m.d.Store.SafeList(s.rec.Workdir)
	allowlist := s.allowlist
	if allowlist == nil { // not started yet: the list it will use
		allowlist = parseAllowlist(s.m.Allowlist(s.rec.ProfileID), s.rec.Workdir)
	}
	return policy{mode: s.rec.Mode, unknown: s.m.unknownSetting(), safe: safe, profile: allowlist,
		workdir: s.rec.Workdir, cwd: s.shellCwd, state: s.m.statePaths(), hints: s.hints, judged: s.m.judged, failed: s.m.judge.isFailed}
}

// rejudgeLocked looks again at the requests waiting for the user after
// something they depend on changed (the level, the safe list, unattended
// mode, a model's judgement), and answers the ones that no longer need
// the user.
func (s *Session) rejudgeLocked() {
	if len(s.pending) == 0 {
		return
	}
	p := s.policyLocked()
	still := s.pending[:0:0]
	for _, a := range s.pending {
		if a.Marked { // the user is answering this one on its card
			still = append(still, a)
			continue
		}
		v := p.judge(a.Action)
		if !s.settleLocked(a.RequestID, a.ToolUseID, a.Action, a.Input, v) {
			a.update(v)
			if a.Judging {
				s.m.startJudging(v.judge)
			}
			still = append(still, a)
		}
	}
	s.pending = still
	s.resumeLocked()
	if s.waitingForUserLocked() && s.page != nil {
		s.state = NeedsApproval // a request the model was judging now needs the user
	}
}

// MarkSafe toggles "This is safe" on a waiting card. On, each part that
// prompted is marked safe in scope, remembering what was there; off puts
// that back (an entry's old verdict, or no entry). The card itself keeps
// waiting for Allow once or Deny. The caller looks again at the other
// waiting requests.
func (s *Session) MarkSafe(requestID string, on bool, scope string) error {
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
	a := &s.pending[i]
	st := s.m.d.Store
	if !on {
		for _, pr := range a.prior {
			var err error
			if pr.had {
				err = st.SetSafeEntry(pr.old)
			} else {
				err = st.DeleteSafeEntry(pr.at)
			}
			if err != nil {
				return err
			}
		}
		a.Marked, a.MarkedScope, a.prior = false, "", nil
		s.changed()
		return nil
	}
	if a.Marked {
		return nil
	}
	classes, ok := learnable(a.Why)
	if !ok {
		return errors.New("this can only be allowed once")
	}
	folder, err := s.scopeFolder(scope)
	if err != nil {
		return err
	}
	a.prior = nil
	for _, c := range classes {
		at := store.SafeEntry{Kind: c.Kind, Words: c.Words, Flags: c.Flags, Folder: folder}
		old, had := st.SafeEntryAt(at)
		a.prior = append(a.prior, priorEntry{at: at, old: old, had: had})
		mark := at
		mark.Verdict = store.Safe
		if err := st.SetSafeEntry(mark); err != nil {
			return err
		}
	}
	a.Marked, a.MarkedScope = true, scope
	s.changed()
	return nil
}

// Answer gives the user's decision on a waiting request. Safe remembers
// each part that prompted as safe, in scope (ScopeProject or ScopeAll),
// then allows it; every other waiting request is looked at again.
func (s *Session) Answer(requestID, decision, scope string) error {
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
	switch decision {
	case Allow, Deny:
	case Safe:
		classes, ok := learnable(a.Why)
		if !ok {
			return errors.New("this can only be allowed once")
		}
		folder, err := s.scopeFolder(scope)
		if err != nil {
			return err
		}
		for _, c := range classes {
			if err := s.m.d.Store.SetSafeEntry(store.SafeEntry{Kind: c.Kind, Words: c.Words, Flags: c.Flags, Verdict: store.Safe, Folder: folder}); err != nil {
				return err
			}
		}
	default:
		return errors.New("unknown decision " + decision)
	}
	allow := decision != Deny
	if err := s.replyLocked(a.RequestID, allow, a.Input, deniedOnCard); err != nil {
		return err
	}
	s.pending = append(s.pending[:i:i], s.pending[i+1:]...)
	if allow {
		s.markApprovedLocked(a.ToolUseID, "you", nil)
		s.followShellLocked(a.Action)
	} else {
		s.markDeniedLocked(a.ToolUseID, nil)
	}
	s.resumeLocked()
	s.changed()
	if decision == Safe {
		go s.m.rejudgeAll() // what was just learned may cover other waiting requests, here and elsewhere
	}
	return nil
}

// scopeFolder is the folder a safe-list entry belongs to: this session's
// for ScopeProject, none for ScopeAll.
func (s *Session) scopeFolder(scope string) (string, error) {
	switch scope {
	case ScopeProject, "":
		return s.rec.Workdir, nil
	case ScopeAll:
		return "", nil
	}
	return "", errors.New("unknown scope " + scope)
}

// resumeLocked goes back to work once nothing waits for the user.
func (s *Session) resumeLocked() {
	if !s.waitingForUserLocked() && s.state == NeedsApproval {
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
	s.rejudgeLocked()
	s.changed()
	return s.viewLocked()
}

// SetMode changes when the session prompts (modes.go). It applies to the
// next tool use the CLI asks about, and to requests already waiting.
// "Always" also needs the CLI to ask about edits it would otherwise make
// on its own, so the CLI's permission mode follows (cliModeLocked).
func (s *Session) SetMode(mode string) (View, error) {
	if !ValidMode(mode) {
		return View{}, errors.New("unknown prompt level " + mode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.Mode = mode
	if err := s.m.d.Store.UpdateSession(&s.rec); err != nil {
		return View{}, err
	}
	s.applyCLIModeLocked()
	s.rejudgeLocked()
	s.changed()
	return s.viewLocked(), nil
}

// Rejudge looks again at waiting requests after a change made elsewhere
// (the safe list, a judgement).
func (s *Session) Rejudge() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) > 0 {
		s.rejudgeLocked()
		s.changed()
	}
}

// cliModeLocked is the permission mode the CLI should run in: the
// profile's, except under "Always prompt me", where it must ask about the
// edits acceptEdits would approve on its own.
func (s *Session) cliModeLocked(profileMode string) string {
	if modeOf(s.rec.Mode) == ModeAlways && profileMode != "" && profileMode != "default" {
		return "default"
	}
	return profileMode
}

// applyCLIModeLocked switches a running CLI to the mode it should be in,
// live when the adapter can; otherwise the next turn respawns it (Send
// compares the modes).
func (s *Session) applyCLIModeLocked() {
	if s.proc == nil || !s.ad().Capabilities().LivePermissionMode {
		return
	}
	want := s.cliModeLocked(s.profileMode)
	if want == s.cliMode {
		return
	}
	if want == "" {
		want = "default"
	}
	if b, ok := s.ad().EncodeControl(core.Control{Kind: core.CtlSetPermissionMode, Mode: want}); ok {
		if _, err := s.proc.Stdin().Write(b); err == nil {
			s.cliMode = s.cliModeLocked(s.profileMode)
		}
	}
}

// modeDirectives is what the next turn tells the model about the prompt
// level and unattended mode, each when it changed since the model was last
// told. Caller holds s.mu.
func (s *Session) modeDirectives() []string {
	var out []string
	if mode := modeOf(s.rec.Mode); mode != s.toldMode {
		switch mode {
		case ModeAlways:
			out = append(out, levelAlways)
		case ModeNever:
			out = append(out, levelNever)
		default:
			out = append(out, levelUnsafe)
		}
	}
	if s.unattended != s.toldUnattended {
		if s.unattended {
			out = append(out, unattendedOn)
		} else {
			out = append(out, unattendedOff)
		}
	}
	return out
}

// toldLocked records that the model now knows the current modes.
func (s *Session) toldLocked() {
	s.toldUnattended = s.unattended
	s.toldMode = modeOf(s.rec.Mode)
}

// markApprovedLocked records on the trace who let a tool use run, and
// why for each part.
func (s *Session) markApprovedLocked(toolUseID, by string, why []store.TraceReason) {
	if s.page == nil || toolUseID == "" {
		return
	}
	for k := range s.page.Trace {
		if s.page.Trace[k].ID == toolUseID {
			s.page.Trace[k].Approved, s.page.Trace[k].Why = by, why
		}
	}
	s.savePage()
}

func (s *Session) markDeniedLocked(toolUseID string, why []store.TraceReason) {
	if s.page == nil || toolUseID == "" {
		return
	}
	for k := range s.page.Trace {
		if s.page.Trace[k].ID == toolUseID {
			s.page.Trace[k].Denied = true
			if why != nil {
				s.page.Trace[k].Why = why
			}
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
	if s.replyHookLocked(requestID, allow, message) {
		return nil
	}
	b, ok := s.ad().EncodeControl(core.Control{Kind: core.CtlApprove, RequestID: requestID, Allow: allow, UpdatedInput: input, Message: message})
	if !ok || s.proc == nil {
		return errors.New("the CLI isn't running")
	}
	_, err := s.proc.Stdin().Write(b)
	return err
}
