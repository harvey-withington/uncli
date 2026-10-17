package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// UserMCP is the MCP servers the user set up for the CLI everywhere (user
// scope, `claude mcp add -s user`), from its ~/.claude.json. An isolated
// session gets them with --mcp-config when its profile shares them, since
// it reads none of the user's settings.
func UserMCP(claudeJSON []byte) map[string]json.RawMessage {
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(claudeJSON, &cfg) != nil {
		return nil
	}
	return cfg.MCPServers
}

// claudeJSON reads the CLI's ~/.claude.json (in CLAUDE_CONFIG_DIR when
// that is set); nil when there is none.
func (a *Adapter) claudeJSON() []byte {
	if a.userConfig != nil {
		return a.userConfig()
	}
	path := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json")
	if os.Getenv("CLAUDE_CONFIG_DIR") == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		path = filepath.Join(home, ".claude.json")
	}
	b, _ := os.ReadFile(path)
	return b
}

// configDir is the CLI's config folder: CLAUDE_CONFIG_DIR, else ~/.claude.
func configDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}
