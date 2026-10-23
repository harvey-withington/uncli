//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"uncli/internal/app"
	"uncli/internal/core"
)

// MCP sign-in against the real CLI (decision 0020): a local MCP server that
// needs OAuth (testdata/streams/claude/mcp-oauth-server.mjs, which approves
// at once), signed in through UNCLI with no restart, then signed out. The
// CLI gets a config folder of its own, so no token reaches the user's, and
// no turn is sent (no usage).
func TestMCPSignIn(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	port := freePort(t)
	server := exec.Command("node", filepath.Join("..", "..", "testdata", "streams", "claude", "mcp-oauth-server.mjs"), port)
	if err := server.Start(); err != nil {
		t.Skipf("node: %v", err)
	}
	t.Cleanup(func() { _ = server.Process.Kill() })
	waitTil(t, "the fake server", 10*time.Second, func() bool {
		r, err := http.Get("http://127.0.0.1:" + port + "/.well-known/oauth-authorization-server")
		return err == nil && r.StatusCode == 200
	})

	config, dir := t.TempDir(), t.TempDir()
	cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"locked": map[string]any{"type": "http", "url": "http://127.0.0.1:" + port + "/mcp"}}})
	os.WriteFile(filepath.Join(dir, "mcp.json"), cfg, 0o644)
	os.WriteFile(filepath.Join(config, "profiles.yaml"), []byte(`profiles:
  - id: locked
    label: Locked
    folder: pick
    model: haiku
    tools: [Read]
    isolated: true
    permission_mode: default
    mcp_config: ["`+filepath.ToSlash(filepath.Join(dir, "mcp.json"))+`"]
`), 0o644)
	svc, _ := open(t, config)
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("locked", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}

	in, err := svc.Sessions.MCPSignIn(ctx, v.ID, "locked")
	if err != nil {
		t.Fatal(err)
	}
	if in.URL == "" || in.Connector {
		t.Fatalf("sign-in = %+v", in)
	}
	// The status comes in once the CLI has started: needs signing in, waiting.
	waitTil(t, "signing", 20*time.Second, func() bool { return mcpStatus(t, svc, v.ID).Signing })

	// The browser: the server approves and sends it back to the CLI's listener.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, err := client.Get(in.URL)
	if err != nil {
		t.Fatal(err)
	}
	back := r.Header.Get("Location")
	if _, err := http.Get(back); err != nil {
		t.Fatalf("the CLI's listener at %s: %v", back, err)
	}
	waitTil(t, "connected", 20*time.Second, func() bool {
		s := mcpStatus(t, svc, v.ID)
		return s.Status == core.MCPConnected && s.Tools == 1 && !s.Signing
	})

	if err := svc.Sessions.MCPSignOut(v.ID, "locked"); err != nil {
		t.Fatal(err)
	}
	waitTil(t, "signed out", 20*time.Second, func() bool { return mcpStatus(t, svc, v.ID).Status == core.MCPNeedsAuth })
	// Reconnecting without a sign-in is refused, on the server, not the session.
	if err := svc.Sessions.MCPReconnect(ctx, v.ID, "locked"); err != nil {
		t.Fatal(err)
	}
	waitTil(t, "the refusal", 20*time.Second, func() bool { return mcpStatus(t, svc, v.ID).Error != "" })
	if view, _ := svc.Sessions.View(v.ID); view.Error != "" {
		t.Errorf("session error: %s", view.Error)
	}
}

func mcpStatus(t *testing.T, svc *app.Service, id string) (out struct {
	Status, Error string
	Tools         int
	Signing       bool
}) {
	t.Helper()
	v, err := svc.Sessions.View(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range v.MCP {
		if s.Name == "locked" {
			out.Status, out.Error, out.Tools, out.Signing = s.Status, s.Error, s.Tools, s.Signing
		}
	}
	return out
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func waitTil(t *testing.T, what string, d time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
