package session

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uncli/internal/core"
)

// written: a line containing all of want was written to a CLI.
func (h *harness) written(want ...string) bool {
	h.rt.mu.Lock()
	defer h.rt.mu.Unlock()
	for _, l := range h.rt.stdin {
		all := true
		for _, w := range want {
			all = all && strings.Contains(l, w)
		}
		if all {
			return true
		}
	}
	return false
}

// answerAs feeds a session an event as if its CLI had said it.
func answerAs(s *Session, kind core.EventKind, payload any) {
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	s.handle(gen, core.NewEvent(kind, payload, nil))
}

func mcpOf(t *testing.T, h *harness, id string) map[string]MCPServerView {
	t.Helper()
	v, err := h.m.View(id)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]MCPServerView{}
	for _, sv := range v.MCP {
		out[sv.Name] = sv
	}
	return out
}

// A local sign-in: the CLI's link comes back to open, the server is
// watched until it connects, and other sessions waiting for it reconnect.
func TestMCPSignInLocal(t *testing.T) {
	h := newHarness(t, "single-turn")
	a, _ := h.m.Create("chat", "", "haiku")
	b, _ := h.m.Create("chat", "", "haiku")
	sa, _ := h.m.get(a.ID)
	sb, _ := h.m.get(b.ID)
	_ = sb.warm(context.Background())
	answerAs(sb, core.EvMCPServers, core.MCPServers{List: []core.MCPServer{{Name: "BRUV", Status: core.MCPNeedsAuth}}})

	type result struct {
		in  MCPSignIn
		err error
	}
	done := make(chan result, 1)
	go func() {
		in, err := h.m.MCPSignIn(context.Background(), a.ID, "BRUV")
		done <- result{in, err}
	}()
	// It starts A's CLI and asks it, with no redirect of UNCLI's own.
	waitUntil(t, "the sign-in request", func() bool { return h.written(`"subtype":"mcp_authenticate"`, `"serverName":"BRUV"`) })
	if h.written("redirectUri") {
		t.Error("a local CLI takes the browser back itself")
	}
	answerAs(sa, core.EvMCPAuth, core.MCPAuth{Server: "BRUV", Step: "sign_in", URL: "https://auth.example/authorize?x=1", Callback: true, Port: 4567})
	r := <-done
	if r.err != nil || r.in.URL != "https://auth.example/authorize?x=1" || r.in.Connector {
		t.Fatalf("sign-in = %+v, %v", r.in, r.err)
	}
	answerAs(sa, core.EvMCPServers, core.MCPServers{List: []core.MCPServer{{Name: "BRUV", Status: core.MCPNeedsAuth}}})
	if sv := mcpOf(t, h, a.ID)["BRUV"]; !sv.Signing {
		t.Errorf("waiting for the browser: %+v", sv)
	}
	// It's watched: the status is asked for again.
	waitUntil(t, "a status check", func() bool {
		h.rt.mu.Lock()
		defer h.rt.mu.Unlock()
		n := 0
		for _, l := range h.rt.stdin {
			if strings.Contains(l, `"subtype":"mcp_status"`) {
				n++
			}
		}
		return n >= 1
	})

	// Connected: no longer signing, and B (same CLI, same place, waiting) reconnects.
	answerAs(sa, core.EvMCPServers, core.MCPServers{List: []core.MCPServer{{Name: "BRUV", Status: core.MCPConnected, Tools: 12}}})
	if sv := mcpOf(t, h, a.ID)["BRUV"]; sv.Signing || sv.Tools != 12 || sv.Status != core.MCPConnected {
		t.Errorf("connected: %+v", sv)
	}
	waitUntil(t, "B reconnecting", func() bool { return h.written(`"subtype":"mcp_reconnect"`, `"serverName":"BRUV"`) })
	// B's refusal (say it keeps its own sign-ins) isn't shown as a failure.
	answerAs(sb, core.EvMCPAuth, core.MCPAuth{Server: "BRUV", Step: "reconnect", Error: "Server status: needs-auth"})
	if sv := mcpOf(t, h, b.ID)["BRUV"]; sv.Error != "" {
		t.Errorf("a quiet reconnect's refusal showed: %+v", sv)
	}
}

// A CLI elsewhere (WSL, a container) is asked to send the browser back to
// UNCLI, which hands the address on and reconnects.
func TestMCPSignInElsewhere(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	s, _ := h.m.get(v.ID)
	_ = s.warm(context.Background())
	s.mu.Lock()
	s.rec.Runtime = RuntimeWSL // the CLI is running; now it's "elsewhere"
	s.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := h.m.MCPSignIn(context.Background(), v.ID, "BRUV")
		done <- err
	}()
	var redirect string
	waitUntil(t, "a sign-in asking for UNCLI's redirect", func() bool {
		h.rt.mu.Lock()
		defer h.rt.mu.Unlock()
		for _, l := range h.rt.stdin {
			if i := strings.Index(l, `"redirectUri":"`); i >= 0 {
				redirect = strings.SplitN(l[i+len(`"redirectUri":"`):], `"`, 2)[0]
				return true
			}
		}
		return false
	})
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") {
		t.Fatalf("redirect = %q", redirect)
	}
	answerAs(s, core.EvMCPAuth, core.MCPAuth{Server: "BRUV", Step: "sign_in", URL: "https://auth.example/a?redirect_uri=" + url.QueryEscape(redirect), Callback: true, Custom: true})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The browser comes back to UNCLI: the address goes to the CLI, then a reconnect.
	go func() { _, _ = http.Get(redirect + "?code=abc&state=xyz") }()
	waitUntil(t, "the callback", func() bool { return h.written(`"subtype":"mcp_oauth_callback_url"`, `code=abc`) })
	answerAs(s, core.EvMCPAuth, core.MCPAuth{Server: "BRUV", Step: "callback"})
	waitUntil(t, "the reconnect", func() bool { return h.written(`"subtype":"mcp_reconnect"`) })
}

// Failures are the server's, shown on it; a session whose CLI can't sign
// in says so.
func TestMCPSignInFails(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	s, _ := h.m.get(v.ID)
	_ = s.warm(context.Background())
	answerAs(s, core.EvMCPServers, core.MCPServers{List: []core.MCPServer{{Name: "BRUV", Status: core.MCPNeedsAuth}}})
	done := make(chan error, 1)
	go func() {
		_, err := h.m.MCPSignIn(context.Background(), v.ID, "BRUV")
		done <- err
	}()
	waitUntil(t, "the sign-in request", func() bool { return h.written(`"subtype":"mcp_authenticate"`) })
	answerAs(s, core.EvMCPAuth, core.MCPAuth{Server: "BRUV", Step: "sign_in", Error: "Server not found: BRUV"})
	if err := <-done; err == nil || err.Error() != "Server not found: BRUV" {
		t.Errorf("err = %v", err)
	}
	if sv := mcpOf(t, h, v.ID)["BRUV"]; sv.Error != "Server not found: BRUV" || sv.Signing {
		t.Errorf("view = %+v", sv)
	}
	if v2, _ := h.m.View(v.ID); v2.Error != "" {
		t.Errorf("not the session's error: %q", v2.Error)
	}
	// Reconnect and sign-out are sent as asked.
	if err := h.m.MCPReconnect(context.Background(), v.ID, "BRUV"); err != nil || !h.written(`"subtype":"mcp_reconnect"`) {
		t.Errorf("reconnect: %v", err)
	}
	if err := h.m.MCPSignOut(v.ID, "BRUV"); err != nil || !h.written(`"subtype":"mcp_clear_auth"`) {
		t.Errorf("sign-out: %v", err)
	}
	if _, err := h.m.MCPSignIn(context.Background(), "nope", "BRUV"); err == nil {
		t.Error("an unknown session")
	}
}
