package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"uncli/internal/core"
	"uncli/internal/oauthcatch"
)

// MCP sign-in (decision 0020). A session keeps what its CLI says about its
// MCP servers (EvMCPServers) in its view, so the UI can offer to sign in
// to one that needs it, reconnect one that failed, or sign out. The CLI
// says nothing when a server's status changes, so while a sign-in waits
// for the browser (or a reconnect settles) the session asks every second.
//
// Signing in: the CLI gives a link to open in the browser. A local CLI
// takes the browser's return on a port of its own and finishes by itself.
// A CLI elsewhere (WSL, a container) is asked to send the browser back to
// UNCLI instead (oauthcatch), which hands the address on and reconnects.
// The user can always paste the address their browser ended up on.

// MCPServerView is a server as the UI shows it.
type MCPServerView struct {
	core.MCPServer
	Signing bool   `json:"signing,omitempty"` // a sign-in is waiting for the browser
	Error   string `json:"error,omitempty"`   // the last sign-in, reconnect or sign-out failed
}

// MCPSignIn is a sign-in under way: the link to open, and whether it's a
// connector of the provider's own (signed in on its site, then reconnected).
type MCPSignIn struct {
	URL       string `json:"url"`
	Connector bool   `json:"connector"`
	Done      bool   `json:"done"` // signed in already, nothing to open
}

const (
	mcpSignInWait = 10 * time.Minute // how long a sign-in waits for the browser
	mcpSettle     = 20 * time.Second // how long a reconnect or a starting CLI is watched
	mcpAnswerWait = 30 * time.Second // how long the CLI has to answer a request
)

// errNoMCP: the session's CLI can't sign in to MCP servers.
var errNoMCP = errors.New("this session's CLI can't sign in to MCP servers from UNCLI")

func (m *Manager) mcpSession(id string) (*Session, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	if !s.ad().Capabilities().MCPSignIn {
		return nil, errNoMCP
	}
	return s, nil
}

// MCPSignIn starts signing in to a server: the link comes back to open in
// the browser. The session's CLI is started if it isn't running.
func (m *Manager) MCPSignIn(ctx context.Context, id, server string) (MCPSignIn, error) {
	s, err := m.mcpSession(id)
	if err != nil {
		return MCPSignIn{}, err
	}
	if err := s.warm(ctx); err != nil {
		return MCPSignIn{}, err
	}
	var catcher *oauthcatch.Catcher
	if !m.local(s) {
		if catcher, err = oauthcatch.Start(); err != nil {
			return MCPSignIn{}, fmt.Errorf("couldn't wait for the browser: %w", err)
		}
	}
	redirect := ""
	if catcher != nil {
		redirect = catcher.URL
	}
	ans, err := s.mcpAsk(core.Control{Kind: core.CtlMCPSignIn, Server: server, URL: redirect})
	if err == nil && ans.Error != "" {
		err = errors.New(ans.Error)
	}
	if err != nil || !ans.Callback || !ans.Custom {
		// The CLI's own listener (or none): nothing for UNCLI to catch.
		if catcher != nil {
			catcher.Close()
		}
		catcher = nil
	}
	if err != nil {
		s.mcpFailed(server, err.Error())
		return MCPSignIn{}, err
	}
	s.mu.Lock()
	if ans.URL != "" {
		s.mcpSigning[server] = true
		s.mcpErrors[server] = ""
		s.watchLocked(server, mcpSignInWait)
	}
	if catcher != nil {
		if old := s.mcpCatch[server]; old != nil {
			old.Close()
		}
		s.mcpCatch[server] = catcher
	}
	s.changed()
	s.mu.Unlock()
	if catcher != nil {
		go func() {
			wctx, cancel := context.WithTimeout(context.Background(), mcpSignInWait)
			defer cancel()
			u, err := catcher.Wait(wctx)
			catcher.Close()
			if err == nil {
				_ = m.MCPCallback(id, server, u)
			}
		}()
	}
	return MCPSignIn{URL: ans.URL, Connector: ans.URL != "" && !ans.Callback, Done: ans.Done}, nil
}

// MCPCallback hands the CLI the address the browser came back to (caught
// by UNCLI, or pasted by the user); the CLI then reconnects the server.
func (m *Manager) MCPCallback(id, server, url string) error {
	s, err := m.mcpSession(id)
	if err != nil {
		return err
	}
	ans, err := s.mcpAsk(core.Control{Kind: core.CtlMCPCallback, Server: server, URL: url})
	if err == nil && ans.Error != "" {
		err = errors.New(ans.Error)
	}
	if err != nil {
		s.mcpFailed(server, err.Error())
		return err
	}
	return s.mcpSend(core.Control{Kind: core.CtlMCPReconnect, Server: server}, mcpSignInWait)
}

// MCPReconnect connects a server again: after signing in elsewhere (a
// connector on the provider's site), or after it failed.
func (m *Manager) MCPReconnect(ctx context.Context, id, server string) error {
	s, err := m.mcpSession(id)
	if err != nil {
		return err
	}
	if err := s.warm(ctx); err != nil {
		return err
	}
	return s.mcpSend(core.Control{Kind: core.CtlMCPReconnect, Server: server}, mcpSettle)
}

// MCPSignOut forgets a server's sign-in, for every session that shares it.
func (m *Manager) MCPSignOut(id, server string) error {
	s, err := m.mcpSession(id)
	if err != nil {
		return err
	}
	return s.mcpSend(core.Control{Kind: core.CtlMCPSignOut, Server: server}, mcpSettle)
}

// MCPCancel stops waiting for a sign-in.
func (m *Manager) MCPCancel(id, server string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopSigningLocked(server)
	delete(s.mcpWatch, server)
	s.changed()
	return nil
}

// local: the session's CLI runs on this machine, where the browser can
// reach its listener.
func (m *Manager) local(s *Session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Runtime == "" || s.rec.Runtime == m.d.Runtime.ID()
}

// mcpAsk sends a request and waits for the CLI's answer.
func (s *Session) mcpAsk(c core.Control) (core.MCPAuth, error) {
	ch := make(chan core.MCPAuth, 1)
	key := string(c.Kind) + "\x00" + c.Server
	s.mu.Lock()
	if s.proc == nil {
		s.mu.Unlock()
		return core.MCPAuth{}, errors.New("the session's CLI isn't running")
	}
	b, ok := s.encodeControl(c)
	if !ok {
		s.mu.Unlock()
		return core.MCPAuth{}, errNoMCP
	}
	s.mcpAnswers[key] = ch
	_, err := s.proc.Stdin().Write(b)
	s.mu.Unlock()
	if err != nil {
		return core.MCPAuth{}, err
	}
	select {
	case a := <-ch:
		return a, nil
	case <-time.After(mcpAnswerWait):
		s.mu.Lock()
		delete(s.mcpAnswers, key)
		s.mu.Unlock()
		return core.MCPAuth{}, errors.New("the CLI didn't answer")
	}
}

// mcpSend sends a request without waiting (its answer is an EvMCPAuth)
// and watches the server's status for a while.
func (s *Session) mcpSend(c core.Control, watch time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil {
		return errors.New("the session's CLI isn't running")
	}
	b, ok := s.encodeControl(c)
	if !ok {
		return errNoMCP
	}
	if _, err := s.proc.Stdin().Write(b); err != nil {
		return err
	}
	s.mcpErrors[c.Server] = ""
	s.watchLocked(c.Server, watch)
	s.changed()
	return nil
}

func (s *Session) mcpFailed(server, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopSigningLocked(server)
	s.mcpErrors[server] = msg
	s.changed()
}

func (s *Session) stopSigningLocked(server string) {
	delete(s.mcpSigning, server)
	if c := s.mcpCatch[server]; c != nil {
		c.Close()
		delete(s.mcpCatch, server)
	}
}

// watchLocked asks for the servers' status every second until the server
// connects or for d ("" watches until none is pending). Caller holds s.mu.
func (s *Session) watchLocked(server string, d time.Duration) {
	if until := time.Now().Add(d); s.mcpWatch[server].Before(until) {
		s.mcpWatch[server] = until
	}
	if s.mcpTicking {
		return
	}
	s.mcpTicking = true
	go s.watchLoop()
}

func (s *Session) watchLoop() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		now := time.Now()
		for k, until := range s.mcpWatch {
			if now.After(until) {
				delete(s.mcpWatch, k)
				if s.mcpSigning[k] {
					s.stopSigningLocked(k)
					s.mcpErrors[k] = "The sign-in wasn't finished in time."
					s.changed()
				}
			}
		}
		if len(s.mcpWatch) == 0 || s.proc == nil {
			s.mcpTicking = false
			s.mu.Unlock()
			return
		}
		if b, ok := s.encodeControl(core.Control{Kind: core.CtlToolHints}); ok {
			_, _ = s.proc.Stdin().Write(b)
		}
		s.mu.Unlock()
	}
}

// mcpServersLocked takes the CLI's report: servers that connected stop
// being watched, and one a sign-in just connected is reconnected in the
// other sessions that share the sign-in. Caller holds s.mu.
func (s *Session) mcpServersLocked(list []core.MCPServer) {
	s.mcp = list
	pending := false
	for _, sv := range list {
		switch sv.Status {
		case core.MCPConnected:
			delete(s.mcpWatch, sv.Name)
			s.mcpErrors[sv.Name] = ""
			if s.mcpSigning[sv.Name] {
				s.stopSigningLocked(sv.Name)
				go s.m.reconnectOthers(s, sv.Name)
			}
		case core.MCPPending:
			pending = true
		}
	}
	if !pending {
		delete(s.mcpWatch, "")
	}
	s.changed()
}

// mcpAuthLocked takes an answer to a sign-in request. Caller holds s.mu.
func (s *Session) mcpAuthLocked(a core.MCPAuth) {
	kind := map[string]core.ControlKind{"sign_in": core.CtlMCPSignIn, "callback": core.CtlMCPCallback,
		"reconnect": core.CtlMCPReconnect, "sign_out": core.CtlMCPSignOut}[a.Step]
	key := string(kind) + "\x00" + a.Server
	if ch, ok := s.mcpAnswers[key]; ok {
		delete(s.mcpAnswers, key)
		ch <- a
		return
	}
	if a.Error != "" && !s.mcpQuiet[a.Server] {
		s.stopSigningLocked(a.Server)
		s.mcpErrors[a.Server] = a.Error
		s.changed()
	}
	delete(s.mcpQuiet, a.Server)
}

// mcpViewLocked is the servers as the UI shows them.
func (s *Session) mcpViewLocked() []MCPServerView {
	out := []MCPServerView{}
	for _, sv := range s.mcp {
		out = append(out, MCPServerView{MCPServer: sv, Signing: s.mcpSigning[sv.Name], Error: s.mcpErrors[sv.Name]})
	}
	return out
}

// reconnectOthers connects a server that was just signed in to in the
// other running sessions that share the sign-in: the same CLI, in the same
// place, waiting for it. A session that keeps sign-ins of its own (a
// sandboxed one) is refused quietly.
func (m *Manager) reconnectOthers(from *Session, server string) {
	from.mu.Lock()
	ad, rt, ref := from.rec.Adapter, from.rec.Runtime, from.rec.RuntimeRef
	from.mu.Unlock()
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
		if o.proc != nil && o.rec.Adapter == ad && o.rec.Runtime == rt && o.rec.RuntimeRef == ref && o.waitsForLocked(server) {
			if b, ok := o.encodeControl(core.Control{Kind: core.CtlMCPReconnect, Server: server}); ok {
				if _, err := o.proc.Stdin().Write(b); err == nil {
					o.mcpQuiet[server] = true
					o.watchLocked(server, mcpSettle)
				}
			}
		}
		o.mu.Unlock()
	}
}

func (s *Session) waitsForLocked(server string) bool {
	for _, sv := range s.mcp {
		if sv.Name == server {
			return sv.Status == core.MCPNeedsAuth || sv.Status == core.MCPFailed
		}
	}
	return false
}
