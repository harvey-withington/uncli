//go:build integration

package uncli

import (
	"context"
	"encoding/json"
	"fmt"
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

// isolated is a chat that shares nothing of the user's own: no MCP servers,
// connectors or skills of theirs, so a test can only reach its stand-in
// board, never the user's real tools.
var isolated = session.NewSession{Profile: "chat", Model: "haiku", Shares: []string{}}

// answerFor allows only the stand-in board's tools; anything else is
// denied, so nothing of the user's can run.
func answerFor(tool string) string {
	if strings.HasPrefix(tool, "mcp__board__") {
		return session.Allow
	}
	return session.Deny
}

// board is a host's own MCP server over HTTP, as BRUV's board tools are:
// a read-only tool, card_title, that needs the host's token, and a write,
// set_card_title.
type board struct {
	mu      sync.Mutex
	tokens  []string // the Authorization of each tool call
	asked   []string // the card ids asked for
	renamed []string // the titles set
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
		}, map[string]any{
			"name": "set_card_title", "description": "Renames a card on the board.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"}}, "required": []string{"id", "title"}},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "openWorldHint": false},
		}}}
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.Name == "set_card_title" {
			b.mu.Lock()
			b.renamed = append(b.renamed, p.Arguments.Title)
			b.mu.Unlock()
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "Renamed."}}}
			break
		}
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
	v, err := sessions.CreateWith(isolated)
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
			_ = sessions.Answer(v.ID, a.RequestID, answerFor(a.Tool), "")
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

// One turn on a host's UNCLI: the answer, and the tools it asked the user
// about (allowed, so the turn can finish).
func turn(t *testing.T, c *Core, id, text string) (string, []session.Approval) {
	t.Helper()
	sessions := c.svc.Sessions
	if err := sessions.Send(context.Background(), id, text); err != nil {
		t.Fatal(err)
	}
	var asked []session.Approval
	start := time.Now()
	for {
		if time.Since(start) > 3*time.Minute {
			t.Fatal("the turn never finished")
		}
		var cur session.View
		for _, s := range sessions.List() {
			if s.ID == id {
				cur = s
			}
		}
		for _, a := range cur.Approvals {
			asked = append(asked, a)
			_ = sessions.Answer(id, a.RequestID, answerFor(a.Tool), "")
		}
		if !cur.Busy && len(cur.Approvals) == 0 && time.Since(start) > 2*time.Second {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	pages, err := sessions.Pages(id)
	if err != nil || len(pages) == 0 {
		t.Fatalf("pages: %v", err)
	}
	return pages[len(pages)-1].AnswerMD, asked
}

// The host decides approvals for its own tools (HostToolPolicy), seeing
// the call's arguments: at the strictest prompt level, where a write always
// asks, a host that allows it lets it run unasked, a host that refuses it
// keeps it from running at all, and a host that asks puts its own words on
// UNCLI's card. Against the real CLI, three short turns.
func TestHostToolPolicyDecidesForTheCLI(t *testing.T) {
	def, err := app.DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	bins, _ := filepath.Glob(filepath.Join(def.Cache, "cli", "claude", "*", "claude.exe"))
	if len(bins) == 0 {
		t.Skip("no downloaded Claude CLI in the app's cache")
	}
	t.Setenv("UNCLI_CLAUDE_BIN", bins[len(bins)-1])
	if prev, ok := os.LookupEnv("CLAUDE_CONFIG_DIR"); ok {
		os.Unsetenv("CLAUDE_CONFIG_DIR")
		t.Cleanup(func() { os.Setenv("CLAUDE_CONFIG_DIR", prev) })
	}
	for _, mode := range []string{PolicyAllow, PolicyDeny, PolicyAsk} {
		t.Run(mode, func(t *testing.T) {
			b := &board{}
			srv := httptest.NewServer(b)
			defer srv.Close()
			c, err := Start(Options{DataDir: t.TempDir(), UserCLIConfig: true,
				MCPServers: func(string) map[string]MCPServer { return map[string]MCPServer{"board": {URL: srv.URL}} },
				HostToolPolicy: func(c HostToolCall) HostDecision {
					if c.Tool != "set_card_title" {
						return HostDecision{}
					}
					d := HostDecision{Decision: mode, Reason: "Chat mode: the board can't be changed."}
					if mode == PolicyAsk {
						d.Reason = "Suggest mode"
						d.Prompt = &HostPrompt{Title: fmt.Sprintf("Rename card %v", c.Args["id"]), Detail: fmt.Sprintf("New title: %v", c.Args["title"])}
					}
					return d
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			v, err := c.svc.Sessions.CreateWith(isolated)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.svc.Sessions.SetMode(v.ID, session.ModeAlways); err != nil {
				t.Fatal(err)
			}
			answer, asked := turn(t, c, v.ID, "Use the board's set_card_title tool to rename card 42 to \"Melbourne in December\". Try it once only, then reply DONE, or say why it failed.")
			var card *session.Approval
			for i, a := range asked {
				if strings.Contains(a.Tool, "set_card_title") {
					card = &asked[i]
				}
			}
			if mode != PolicyAsk && card != nil {
				t.Errorf("the user was asked about %s; the host decided it", card.Tool)
			}
			b.mu.Lock()
			renamed := append([]string(nil), b.renamed...)
			b.mu.Unlock()
			switch mode {
			case PolicyAllow:
				if len(renamed) == 0 || renamed[0] != "Melbourne in December" {
					t.Errorf("allowed, but the board got %v; the answer: %q", renamed, answer)
				}
			case PolicyDeny:
				if len(renamed) != 0 {
					t.Errorf("refused, but the board was renamed: %v", renamed)
				}
				t.Logf("refused; the answer: %q", answer)
			case PolicyAsk:
				// The card had the host's words, made from the call's arguments;
				// allowed on it, the rename ran.
				if card == nil || card.Host == nil {
					t.Fatalf("no card with the host's words: %+v; the answer: %q", card, answer)
				}
				if card.Host.Title != "Rename card 42" || card.Host.Detail != "New title: Melbourne in December" {
					t.Errorf("the card said %+v", card.Host)
				}
				if len(renamed) == 0 || renamed[0] != "Melbourne in December" {
					t.Errorf("allowed on the card, but the board got %v", renamed)
				}
			}
		})
	}
}
