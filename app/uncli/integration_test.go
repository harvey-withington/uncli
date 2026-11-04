//go:build integration

package uncli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/session"
)

// board is a host's own MCP server over HTTP, as BRUV's board tools are:
// one read-only tool, card_title, that needs the host's token.
type board struct {
	mu     sync.Mutex
	tokens []string // the Authorization of each tool call
	asked  []string // the card ids asked for
}

func (b *board) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ID == nil { // a notification
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		result = map[string]any{"protocolVersion": p.ProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": "board", "version": "1.0.0"}}
	case "tools/list":
		result = map[string]any{"tools": []any{map[string]any{
			"name": "card_title", "description": "The title of a card on the board, by its id.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}, "required": []string{"id"}},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "openWorldHint": false},
		}}}
	case "tools/call":
		var p struct {
			Arguments struct {
				ID string `json:"id"`
			} `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		b.mu.Lock()
		b.tokens = append(b.tokens, r.Header.Get("Authorization"))
		b.asked = append(b.asked, p.Arguments.ID)
		b.mu.Unlock()
		title := "No such card"
		if p.Arguments.ID == "42" {
			title = "Lisbon banjo parade"
		}
		result = map[string]any{"content": []any{map[string]any{"type": "text", "text": title}}}
	default:
		result = map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

// A host's own MCP servers reach its sessions' CLI (BRUV's board tools):
// a chat asked about a card calls the host's tool, with the host's token,
// and answers from it. Against the real CLI, with the user's sign-in
// (UserCLIConfig) and the app's downloaded binary; one short turn.
func TestHostMCPServersReachTheCLI(t *testing.T) {
	def, err := app.DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	bins, _ := filepath.Glob(filepath.Join(def.Cache, "cli", "claude", "*", "claude.exe"))
	if len(bins) == 0 {
		t.Skip("no downloaded Claude CLI in the app's cache")
	}
	t.Setenv("UNCLI_CLAUDE_BIN", bins[len(bins)-1])

	b := &board{}
	srv := httptest.NewServer(b)
	defer srv.Close()
	// The user's own sign-in: the variable unset, as it is for them.
	if prev, ok := os.LookupEnv("CLAUDE_CONFIG_DIR"); ok {
		os.Unsetenv("CLAUDE_CONFIG_DIR")
		t.Cleanup(func() { os.Setenv("CLAUDE_CONFIG_DIR", prev) })
	}
	var askedFor []string
	var mu sync.Mutex
	c, err := Start(Options{DataDir: t.TempDir(), UserCLIConfig: true, MCPServers: func(id string) map[string]MCPServer {
		mu.Lock()
		askedFor = append(askedFor, id)
		mu.Unlock()
		return map[string]MCPServer{"board": {URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer host-token"}}}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	sessions := c.svc.Sessions
	v, err := sessions.Create("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.SetMode(v.ID, session.ModeUnsafe); err != nil {
		t.Fatal(err)
	}
	if err := sessions.Send(context.Background(), v.ID, "Use the board's card_title tool to look up card 42, then reply with just its title."); err != nil {
		t.Fatal(err)
	}
	// A read-only tool shouldn't need asking; if it is asked, allow it and say so.
	deadline := time.Now().Add(3 * time.Minute)
	for {
		if time.Now().After(deadline) {
			t.Fatal("the turn never finished")
		}
		var cur session.View
		for _, s := range sessions.List() {
			if s.ID == v.ID {
				cur = s
			}
		}
		for _, a := range cur.Approvals {
			t.Logf("asked about %s; allowing", a.Tool)
			_ = sessions.Answer(v.ID, a.RequestID, session.Allow, "")
		}
		if !cur.Busy && len(cur.Approvals) == 0 && time.Since(deadline.Add(-3*time.Minute)) > 2*time.Second {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	pages, err := sessions.Pages(v.ID)
	if err != nil || len(pages) == 0 {
		t.Fatalf("pages: %v", err)
	}
	answer := pages[len(pages)-1].AnswerMD
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.asked) == 0 || b.asked[0] != "42" {
		t.Fatalf("the board's tool was called for %v; the answer: %q", b.asked, answer)
	}
	for _, tok := range b.tokens {
		if tok != "Bearer host-token" {
			t.Errorf("a tool call came with %q, not the host's token", tok)
		}
	}
	if !strings.Contains(strings.ToLower(answer), "lisbon banjo parade") {
		t.Errorf("the answer %q isn't the card's title", answer)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(askedFor) == 0 || askedFor[0] != v.ID {
		t.Errorf("the host was asked for servers for %v, not session %s", askedFor, v.ID)
	}
}
