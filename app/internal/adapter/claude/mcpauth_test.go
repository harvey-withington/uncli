package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/core"
)

// fixtureLines is a recording's output lines.
func fixtureLines(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtures, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// answer is the recording's answer to a request id, re-tagged as UNCLI tags
// its own (the recordings used plain ids).
func answer(t *testing.T, lines []string, requestID, tag string) []byte {
	t.Helper()
	for _, l := range lines {
		if strings.Contains(l, `"request_id":"`+requestID+`"`) && strings.Contains(l, `"control_response"`) {
			return []byte(strings.Replace(l, `"request_id":"`+requestID+`"`, `"request_id":"`+tag+`"`, 1))
		}
	}
	t.Fatalf("no answer to %s", requestID)
	return nil
}

func one[T any](t *testing.T, evs []core.Event, kind core.EventKind) T {
	t.Helper()
	for _, e := range evs {
		if e.Kind == kind {
			v, err := core.Decode[T](e)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
	}
	t.Fatalf("no %s in %v", kind, evs)
	var zero T
	return zero
}

func TestMCPControls(t *testing.T) {
	a := New(nil)
	if !a.Capabilities().MCPSignIn {
		t.Error("MCPSignIn should be on")
	}
	for kind, want := range map[core.ControlKind][]string{
		core.CtlMCPSignIn:    {`"subtype":"mcp_authenticate"`, `"serverName":"BRUV"`, `"request_id":"mcp.sign_in.`},
		core.CtlMCPReconnect: {`"subtype":"mcp_reconnect"`, `"request_id":"mcp.reconnect.`},
		core.CtlMCPSignOut:   {`"subtype":"mcp_clear_auth"`, `"request_id":"mcp.sign_out.`},
	} {
		b, ok := a.EncodeControl(core.Control{Kind: kind, Server: "BRUV"})
		if !ok {
			t.Fatalf("%s: not encoded", kind)
		}
		for _, w := range want {
			if !strings.Contains(string(b), w) {
				t.Errorf("%s: %s lacks %s", kind, b, w)
			}
		}
		if strings.Contains(string(b), "redirectUri") {
			t.Errorf("%s: no redirect asked for: %s", kind, b)
		}
	}
	b, _ := a.EncodeControl(core.Control{Kind: core.CtlMCPSignIn, Server: "x", URL: "http://127.0.0.1:5000/cb"})
	if !strings.Contains(string(b), `"redirectUri":"http://127.0.0.1:5000/cb"`) {
		t.Errorf("redirect: %s", b)
	}
	b, _ = a.EncodeControl(core.Control{Kind: core.CtlMCPCallback, Server: "x", URL: "http://localhost:1/callback?code=c&state=s"})
	var sent struct {
		Request struct{ Subtype, CallbackURL string } `json:"request"`
	}
	_ = json.Unmarshal(b, &sent)
	if sent.Request.Subtype != "mcp_oauth_callback_url" || sent.Request.CallbackURL != "http://localhost:1/callback?code=c&state=s" {
		t.Errorf("callback: %s", b)
	}
	for _, c := range []core.Control{{Kind: core.CtlMCPSignIn}, {Kind: core.CtlMCPCallback, Server: "x"}} {
		if _, ok := a.EncodeControl(c); ok {
			t.Errorf("%+v should be refused", c)
		}
	}
	// A server name with dots survives the tag.
	if step, server, ok := mcpRequest(mcpRequestID("sign_in", "my.server.v2")); !ok || step != "sign_in" || server != "my.server.v2" {
		t.Errorf("tag: %s %s %v", step, server, ok)
	}
}

// The recorded answers (CLI 2.1.285), as UNCLI reads them.
func TestMCPAnswers(t *testing.T) {
	// Status: needs-auth before, connected with its tool after.
	lines := fixtureLines(t, "mcp-auth-local")
	p := newParser()
	before := one[core.MCPServers](t, mustFeed(t, p, answer(t, lines, "req_2", "req_2")), core.EvMCPServers)
	if len(before.List) != 1 || before.List[0] != (core.MCPServer{Name: "locked", Status: core.MCPNeedsAuth, Tools: 0}) {
		t.Errorf("before = %+v", before)
	}
	after := one[core.MCPServers](t, mustFeed(t, p, answer(t, lines, "req_4", "req_4")), core.EvMCPServers)
	if after.List[0] != (core.MCPServer{Name: "locked", Status: core.MCPConnected, Tools: 1}) {
		t.Errorf("after = %+v", after)
	}

	// Sign in through the CLI's own listener: the link, and its port.
	auth := one[core.MCPAuth](t, mustFeed(t, p, answer(t, lines, "req_3", "mcp.sign_in.ab12.locked")), core.EvMCPAuth)
	if auth.Server != "locked" || auth.Step != "sign_in" || !strings.HasPrefix(auth.URL, "http://127.0.0.1:47011/authorize?") ||
		!auth.Callback || auth.Custom || auth.Port == 0 || auth.Done || auth.Error != "" {
		t.Errorf("local sign-in = %+v", auth)
	}

	// The redirect asked for was taken.
	custom := fixtureLines(t, "mcp-auth-custom")
	auth = one[core.MCPAuth](t, mustFeed(t, p, answer(t, custom, "req_3", "mcp.sign_in.ab12.locked")), core.EvMCPAuth)
	if !auth.Custom || !strings.Contains(auth.URL, "redirect_uri=http%3A%2F%2F127.0.0.1%3A47099%2Fcb") || auth.Port != 0 {
		t.Errorf("custom sign-in = %+v", auth)
	}

	// The callback and reconnect answers carry no data; an error is the
	// step's, never the session's.
	callback := fixtureLines(t, "mcp-auth-callback")
	auth = one[core.MCPAuth](t, mustFeed(t, p, answer(t, callback, "req_4", "mcp.callback.ab12.locked")), core.EvMCPAuth)
	if auth.Step != "callback" || auth.Error != "" {
		t.Errorf("callback = %+v", auth)
	}
	clear := fixtureLines(t, "mcp-auth-clear")
	evs := mustFeed(t, p, answer(t, clear, "req_7", "mcp.reconnect.ab12.locked"))
	auth = one[core.MCPAuth](t, evs, core.EvMCPAuth)
	if auth.Step != "reconnect" || auth.Error != "Server status: needs-auth" {
		t.Errorf("refused reconnect = %+v", auth)
	}
	for _, e := range evs {
		if e.Kind == core.EvError {
			t.Error("a refused MCP request isn't a session error")
		}
	}
	auth = one[core.MCPAuth](t, mustFeed(t, p, answer(t, clear, "req_5", "mcp.sign_out.ab12.locked")), core.EvMCPAuth)
	if auth.Step != "sign_out" || auth.Error != "" {
		t.Errorf("sign-out = %+v", auth)
	}
	// Statuses the CLI might add later count as failed.
	if s := mcpServers([]mcpServer{{Name: "x", Status: "exploded"}}); s.List[0].Status != core.MCPFailed {
		t.Errorf("unknown status = %+v", s)
	}

}

func mustFeed(t *testing.T, p core.Parser, line []byte) []core.Event {
	t.Helper()
	evs, err := p.Feed(line)
	if err != nil {
		t.Fatal(err)
	}
	return evs
}
