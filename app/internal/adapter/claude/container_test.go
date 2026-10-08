package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProjectDir(t *testing.T) {
	for in, want := range map[string]string{
		`s:\Local\Code\Projects\UNCLI\UNCLI-1.0`: "s--Local-Code-Projects-UNCLI-UNCLI-1-0", // as found in ~/.claude/projects
		"/workspace/uncli-1.0-ab12cd34":          "-workspace-uncli-1-0-ab12cd34",
	} {
		if got := ProjectDir(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestSharedBrain(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".claude")
	// The user's own skills, and claude.ai ones synced for their account; a
	// folder without a SKILL.md isn't a skill.
	for _, d := range []string{"skills/mine", "skills/docs", "skills/synced/org_acct/docs", "skills/synced/org_acct/pdf", "skills/synced/org_acct/broken"} {
		if err := os.MkdirAll(filepath.Join(cfg, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(d, "broken") {
			_ = os.WriteFile(filepath.Join(cfg, filepath.FromSlash(d), "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644)
		}
	}
	sh := SharedBrain(cfg, `S:\Code\App`, "/workspace/app-1234", "/home/uncli", true)
	if sh[0].Host != filepath.Join(cfg, "projects", "S--Code-App", "memory") || sh[0].Path != "/home/uncli/.claude/projects/-workspace-app-1234/memory" {
		t.Errorf("memory = %+v", sh[0])
	}
	skills := map[string]string{}
	for _, s := range sh {
		if strings.HasPrefix(s.Path, "/home/uncli/.claude/skills/") {
			skills[strings.TrimPrefix(s.Path, "/home/uncli/.claude/skills/")] = s.Host
		}
	}
	if len(skills) != 3 || skills["mine"] == "" || skills["pdf"] != filepath.Join(cfg, "skills", "synced", "org_acct", "pdf") ||
		skills["docs"] != filepath.Join(cfg, "skills", "docs") { // the user's own wins
		t.Errorf("skills = %v", skills)
	}
	// Signed in to the account, the CLI syncs claude.ai skills itself.
	own := 0
	for _, s := range SharedBrain(cfg, `S:\Code\App`, "/workspace/app-1234", "/home/uncli", false) {
		if strings.HasPrefix(s.Path, "/home/uncli/.claude/skills/") {
			own++
			if strings.Contains(s.Host, "synced") {
				t.Errorf("synced skill given to an account container: %s", s.Host)
			}
		}
	}
	if own != 2 {
		t.Errorf("own skills = %d", own)
	}
	last := sh[len(sh)-1]
	if !last.File || last.Path != "/home/uncli/.claude/CLAUDE.md" {
		t.Errorf("CLAUDE.md = %+v", last)
	}
	for _, s := range sh {
		if strings.Contains(strings.ToLower(s.Host), "credentials") || strings.HasSuffix(s.Host, "settings.json") {
			t.Errorf("%s must never be shared", s.Host)
		}
	}
}

func TestContainerMCP(t *testing.T) {
	cfg := `{
	  "mcpServers": {
	    "web": {"type": "http", "url": "http://example.com/mcp"},
	    "files": {"command": "npx", "args": ["-y", "@scope/files-mcp"]},
	    "wrapped": {"command": "cmd", "args": ["/c", "npx", "-y", "@scope/thing"]},
	    "local-exe": {"command": "C:\\tools\\server.exe", "args": []},
	    "win-path": {"command": "node", "args": ["C:\\code\\server.js"]},
	    "py": {"command": "uvx", "args": ["some-mcp"], "env": {"HOME_DIR": "D:\\data"}}
	  },
	  "projects": {
	    "s:/code/app": {"mcpServers": {"proj": {"type": "sse", "url": "http://localhost:9000/sse"}}},
	    "s:/other": {"mcpServers": {"elsewhere": {"type": "http", "url": "http://x"}}}
	  }
	}`
	servers, skipped := ContainerMCP([]byte(cfg), `S:\Code\App`)
	var names []string
	for n := range servers {
		names = append(names, n)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"files", "proj", "web", "wrapped"}) {
		t.Errorf("kept %v", names)
	}
	if !slices.Equal(skipped, []string{"local-exe", "py", "win-path"}) {
		t.Errorf("skipped %v", skipped)
	}
	var w struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	_ = json.Unmarshal(servers["wrapped"], &w)
	if w.Command != "npx" || !slices.Equal(w.Args, []string{"-y", "@scope/thing"}) {
		t.Errorf("unwrapped = %+v", w)
	}
}

func TestWithMCPKeepsTheRest(t *testing.T) {
	out, err := WithMCP([]byte(`{"numStartups": 3, "mcpServers": {"old": {}}}`), map[string]json.RawMessage{"web": json.RawMessage(`{"type":"http","url":"http://x"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(out, &doc)
	if string(doc["numStartups"]) != "3" || !strings.Contains(string(doc["mcpServers"]), `"web"`) || strings.Contains(string(doc["mcpServers"]), "old") {
		t.Errorf("doc = %s", out)
	}
	if out, _ := WithMCP(nil, map[string]json.RawMessage{}); !strings.Contains(string(out), `"mcpServers": {}`) {
		t.Errorf("from nothing = %s", out)
	}
}

func TestLostConversation(t *testing.T) {
	a := New(nil)
	if !a.LostConversation("No conversation found with session ID: 0b0e8d7e-1111-4222-8333-944455556666\n") {
		t.Error("the CLI's own words weren't recognised")
	}
	if a.LostConversation("Error: rate limited") {
		t.Error("another error taken for a lost conversation")
	}
}
