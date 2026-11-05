package session

import (
	"encoding/json"
	"slices"

	"github.com/harvey-withington/uncli/app/internal/core"
)

// MCP servers the user turned off (Turn off, decision 0020) stay off: the
// choice is saved for this UNCLI, by provider, and applied to every session
// whenever its CLI reports its servers, so a server the user won't sign in
// to stops asking in every chat and after every restart. The CLI can only
// turn a server off once it's running (mcp_toggle), so the view says Off
// from the start, beside the CLI's own status. Turn on undoes it everywhere.

const mcpOffSetting = "mcp.off" // JSON: provider id → server names

// mcpOffLocked is the saved list, read once. Caller holds m.mcpOffMu
// (its own lock, so sessions can ask while holding theirs).
func (m *Manager) mcpOffLocked() map[string][]string {
	if m.mcpOff == nil {
		m.mcpOff = map[string][]string{}
		if v, err := m.d.Store.Setting(mcpOffSetting); err == nil && v != "" {
			_ = json.Unmarshal([]byte(v), &m.mcpOff)
		}
	}
	return m.mcpOff
}

// MCPOff reports whether the user turned a provider's server off.
func (m *Manager) MCPOff(adapter, server string) bool {
	m.mcpOffMu.Lock()
	defer m.mcpOffMu.Unlock()
	return slices.Contains(m.mcpOffLocked()[adapter], server)
}

// setMCPOff saves a server as turned off, or on again.
func (m *Manager) setMCPOff(adapter, server string, off bool) error {
	m.mcpOffMu.Lock()
	defer m.mcpOffMu.Unlock()
	all := m.mcpOffLocked()
	names := slices.DeleteFunc(slices.Clone(all[adapter]), func(n string) bool { return n == server })
	if off {
		names = append(names, server)
	}
	if len(names) == 0 {
		delete(all, adapter)
	} else {
		all[adapter] = names
	}
	b, _ := json.Marshal(all)
	return m.d.Store.SetSetting(mcpOffSetting, string(b))
}

// keepOffLocked turns off, in the CLI, the servers it reports on that the
// user turned off; once per server for each run of the CLI, and quietly
// (an error there isn't the user's). Caller holds s.mu.
func (s *Session) keepOffLocked() {
	if s.proc == nil || !s.ad().Capabilities().MCPSignIn {
		return
	}
	id := s.ad().ID()
	for _, sv := range s.mcp {
		// A server still connecting can't be turned off yet: wait until it settles.
		if sv.Status == core.MCPDisabled || sv.Status == core.MCPPending || !s.m.MCPOff(id, sv.Name) || s.mcpOffSent[sv.Name] == s.proc {
			continue
		}
		b, ok := s.encodeControl(core.Control{Kind: core.CtlMCPToggle, Server: sv.Name, Enabled: false})
		if !ok {
			return
		}
		if _, err := s.proc.Stdin().Write(b); err != nil {
			return
		}
		s.mcpOffSent[sv.Name] = s.proc
		s.mcpQuiet[sv.Name] = true
		// The CLI says nothing when a status changes: ask until it's off.
		s.watchLocked(sv.Name, mcpSettle)
	}
}

// shownOffLocked: the server is off for the user, whatever the CLI says
// while it catches up. Caller holds s.mu.
func (s *Session) shownOffLocked(server string) bool {
	return s.m.MCPOff(s.ad().ID(), server)
}

// toggleElsewhere turns a server off, or on again, in the other running
// sessions of the same provider that have it.
func (m *Manager) toggleElsewhere(from *Session, server string, enabled bool) {
	adapter := from.ad().ID()
	m.mu.Lock()
	others := make([]*Session, 0, len(m.sessions))
	for _, o := range m.sessions {
		if o != from {
			others = append(others, o)
		}
	}
	m.mu.Unlock()
	for _, o := range others {
		o.mu.Lock()
		has := slices.ContainsFunc(o.mcp, func(sv core.MCPServer) bool { return sv.Name == server })
		running := o.proc != nil && o.ad().ID() == adapter && o.ad().Capabilities().MCPSignIn
		o.mu.Unlock()
		if has && running {
			_ = o.mcpSend(core.Control{Kind: core.CtlMCPToggle, Server: server, Enabled: enabled}, mcpSettle)
		}
	}
}
