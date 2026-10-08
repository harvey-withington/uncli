package session

import (
	"sort"
	"strings"

	"uncli/internal/core"
	"uncli/internal/store"
)

// When to prompt and the safe list (modes.go, class.go), as the UI sees them.

// SetMode changes when a session prompts: always, unsafe or never.
func (m *Manager) SetMode(id, mode string) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.SetMode(mode)
}

// SafeList lists the safe-list entries that apply in a session's project:
// its own, then those for all projects.
func (m *Manager) SafeList(id string) ([]store.SafeEntry, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	return m.d.Store.SafeList(s.View().Workdir)
}

// SetSafeEntry adds or changes an entry, in scope ("project": the
// session's folder, or "all"), and looks again at every waiting request.
func (m *Manager) SetSafeEntry(id string, e store.SafeEntry, scope string) ([]store.SafeEntry, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	if e.Folder, err = s.scopeFolder(scope); err != nil {
		return nil, err
	}
	if err := m.d.Store.SetSafeEntry(e); err != nil {
		return nil, err
	}
	m.rejudgeAll()
	return m.SafeList(id)
}

// DeleteSafeEntry removes an entry (its Folder says which scope).
func (m *Manager) DeleteSafeEntry(id string, e store.SafeEntry) ([]store.SafeEntry, error) {
	if err := m.d.Store.DeleteSafeEntry(e); err != nil {
		return nil, err
	}
	m.rejudgeAll()
	return m.SafeList(id)
}

// SetSafeLabel gives an entry the user's own name for it; empty clears it.
func (m *Manager) SetSafeLabel(id string, e store.SafeEntry, label string) ([]store.SafeEntry, error) {
	if err := m.d.Store.SetSafeLabel(e, label); err != nil {
		return nil, err
	}
	return m.SafeList(id)
}

// MoveSafeEntry gives an entry another scope: "project" (the session's
// folder) or "all". An entry already there for the same class takes the
// moved entry's verdict.
func (m *Manager) MoveSafeEntry(id string, e store.SafeEntry, scope string) ([]store.SafeEntry, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	folder, err := s.scopeFolder(scope)
	if err != nil {
		return nil, err
	}
	if err := m.d.Store.MoveSafeEntry(e, folder); err != nil {
		return nil, err
	}
	m.rejudgeAll()
	return m.SafeList(id)
}

// Teach remembers classes with a verdict, in scope: "This should prompt"
// on a trace row teaches unsafe.
func (m *Manager) Teach(id string, classes []store.SafeClass, verdict, scope string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	folder, err := s.scopeFolder(scope)
	if err != nil {
		return err
	}
	for _, c := range classes {
		if err := m.d.Store.SetSafeEntry(store.SafeEntry{Kind: c.Kind, Words: c.Words, Flags: c.Flags, Verdict: verdict, Folder: folder}); err != nil {
			return err
		}
	}
	m.rejudgeAll()
	return nil
}

// ClassPreview is what a command's part would be remembered as, or why it
// can't be.
type ClassPreview struct {
	Part  string           `json:"part"`
	Class *store.SafeClass `json:"class,omitempty"`
	Fixed string           `json:"fixed,omitempty"` // inline, complex or context
}

// PreviewClasses says what each part of an example command would be
// remembered as (Settings: add an entry from an example).
func (m *Manager) PreviewClasses(id, command string) ([]ClassPreview, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	workdir := s.View().Workdir
	parts, ok := workingParts(command, s.dialect(), workdir)
	if !ok {
		return []ClassPreview{{Part: strings.TrimSpace(command), Fixed: FixedComplex}}, nil
	}
	out := []ClassPreview{}
	for _, p := range parts {
		c, fixed := classOf(p)
		if _, ctx := partRisk(p, place{workdir: workdir, state: m.statePaths()}); fixed == "" && ctx.Level == RiskRisky {
			fixed = FixedContext
		}
		pv := ClassPreview{Part: strings.TrimSpace(p.text), Fixed: fixed}
		if fixed == "" {
			pv.Class = &c
		}
		out = append(out, pv)
	}
	return out, nil
}

// KnownTools lists the MCP tools a session has, for adding to the safe list.
func (m *Manager) KnownTools(id string) ([]string, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	out := make([]string, 0, len(s.hints))
	for t := range s.hints {
		out = append(out, t)
	}
	s.mu.Unlock()
	sort.Strings(out)
	return out, nil
}

// statePaths are the folders the CLI keeps its own files in, if its
// adapter says.
func (m *Manager) statePaths() []string {
	var out []string
	for _, a := range append([]core.Adapter{m.d.Adapter}, m.d.Others...) {
		if sp, ok := a.(core.StatePather); ok {
			out = append(out, sp.StatePaths()...)
		}
	}
	return out
}

// rejudgeAll looks again at every session's waiting requests.
func (m *Manager) rejudgeAll() {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		s.Rejudge()
	}
}

// Explanation is what would happen to a command in a session now, and why
// for each of its parts.
type Explanation struct {
	Action string              `json:"action"` // allow (runs) | ask (prompts) | deny (blocked or, unattended, declined)
	Why    []store.TraceReason `json:"why"`
}

// ExplainCommand says what a session would do with a command if Claude ran
// it now, without running anything. A command UNCLI doesn't recognise is
// judged by the quick-task model first, when that is the user's setting.
func (m *Manager) ExplainCommand(id, command string) (Explanation, error) {
	s, err := m.get(id)
	if err != nil {
		return Explanation{}, err
	}
	s.mu.Lock()
	p := s.policyLocked()
	unattended := s.unattended
	s.mu.Unlock()
	action := core.ToolAction{Kind: core.ActShell, Tool: "shell", Dialect: s.dialect(), Command: command}
	v := p.judge(action)
	if v.action == actionJudge { // judge it now: the user is waiting for the answer
		for _, q := range v.judge {
			m.judgeNow(q)
		}
		v = p.judge(action)
	}
	if unattended && v.action == actionPrompt {
		v.action = actionBlock
	}
	return Explanation{Action: v.action, Why: v.why}, nil
}

// Allowlist is a session type's list of what's safe in it (allowed_tools),
// as written. UNCLI treats it as safe when prompting only when unsafe.
func (m *Manager) Allowlist(profileID string) []string {
	p, ok := m.d.Profiles.Profile(profileID)
	if !ok || p.AllowedTools == nil {
		return []string{}
	}
	return append([]string{}, p.AllowedTools...)
}

// SessionAllowlist is the list of a session's type.
func (m *Manager) SessionAllowlist(id string) ([]string, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	return m.Allowlist(s.View().ProfileID), nil
}
