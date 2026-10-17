package session

import (
	"errors"
	"fmt"
	"slices"

	"uncli/internal/core"
	"uncli/internal/profile"
	"uncli/internal/store"
)

// What an isolated session shares of the user's own: their MCP servers and
// connectors (tools) and their skills. Each session starts with its type's
// defaults and can change them; a change applies from the next turn, which
// restarts the CLI on the conversation (decision 0016).

// shareable is what a session could share: only an isolated type's, only
// what its CLI can be given, and only on this machine (a session in a
// container gets what its container profile shares).
func (m *Manager) shareable(rec store.Session, caps core.Capabilities) []string {
	out := []string{}
	p, ok := m.d.Profiles.Profile(rec.ProfileID)
	if !ok || !p.Isolated || rec.Runtime != "" && rec.Runtime != m.d.Runtime.ID() {
		return out
	}
	if caps.SharedTools {
		out = append(out, profile.ShareTools)
	}
	if caps.SharedSkills {
		out = append(out, profile.ShareSkills)
	}
	return out
}

// sharesLocked is what the session shares now: its own choice, else its
// type's defaults, of what it can. Caller holds s.mu.
func (s *Session) sharesLocked() []string {
	can := s.m.shareable(s.rec, s.ad().Capabilities())
	want := s.rec.ShareChoice
	if want == nil {
		p, _ := s.m.d.Profiles.Profile(s.rec.ProfileID)
		want = p.DefaultShares()
	}
	out := []string{}
	for _, x := range can {
		if slices.Contains(want, x) {
			out = append(out, x)
		}
	}
	return out
}

// checkShares refuses a share the session can't have.
func checkShares(shares, can []string) error {
	for _, x := range shares {
		if !slices.Contains(can, x) {
			return fmt.Errorf("this session can't share %q", x)
		}
	}
	return nil
}

// SetShares changes what a session shares, from its next turn.
func (m *Manager) SetShares(id string, shares []string) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.SetShares(shares)
}

func (s *Session) SetShares(shares []string) (View, error) {
	if shares == nil {
		return View{}, errors.New("say what the session shares")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := checkShares(shares, s.m.shareable(s.rec, s.ad().Capabilities())); err != nil {
		return View{}, err
	}
	s.rec.ShareChoice = slices.Clone(shares)
	if err := s.m.d.Store.UpdateSession(&s.rec); err != nil {
		return View{}, err
	}
	s.changed()
	return s.viewLocked(), nil
}

// sharesKey says what a launch shares, to tell when the running CLI was
// started with something else.
func sharesKey(spec core.LaunchSpec) string {
	return fmt.Sprint(spec.ShareMCP, spec.ShareConnectors, spec.ShareSkills)
}
