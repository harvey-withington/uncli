package claude

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// What a container (decision 0011) shares of the user's Claude setup. The
// CLI keeps each project's files (transcripts, memory) in
// <config>/projects/<project dir>, so a session's memory in a container is
// the same folder as on Windows, mounted at the name the container's path
// gives it. Credentials are never shared.

// ProjectDir is the folder name the CLI gives a project: its path with
// every character but letters and digits turned into a dash
// (S:\Code\My-App -> S--Code-My-App, /workspace/app -> -workspace-app).
func ProjectDir(p string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, p)
}

// Share is something of the user's to give a container: a folder mounted
// read-write, or a file copied in.
type Share struct {
	Host string // on Windows
	Path string // in the container
	File bool
}

// SharedBrain lists what a container session in host (seen there as cli)
// shares: the project's memory folder, the user's skills, agents and
// commands, and their CLAUDE.md. configDir is the CLI's config folder on
// Windows, home the container user's home. Missing folders are the
// caller's to skip (the memory folder is worth creating).
//
// Skills go in one by one. The CLI keeps the user's claude.ai skills in
// skills/synced/<org>_<account>/<skill> and loads only the folder of the
// account it's signed in to, which a container signed in with a token
// doesn't know; so each synced skill is given as an ordinary skill. The
// user's own skills win a name they share. A container signed in to the
// account (synced false) syncs them itself, so they aren't given twice.
func SharedBrain(configDir, host, cli, home string, synced bool) []Share {
	in := path.Join(home, ".claude")
	out := []Share{{Host: filepath.Join(configDir, "projects", ProjectDir(host), "memory"), Path: path.Join(in, "projects", ProjectDir(cli), "memory")}}
	for name, dir := range skillDirs(filepath.Join(configDir, "skills"), synced) {
		out = append(out, Share{Host: dir, Path: path.Join(in, "skills", name)})
	}
	sort.Slice(out[1:], func(i, j int) bool { return out[1+i].Path < out[1+j].Path })
	return append(out,
		Share{Host: filepath.Join(configDir, "agents"), Path: path.Join(in, "agents")},
		Share{Host: filepath.Join(configDir, "commands"), Path: path.Join(in, "commands")},
		Share{Host: filepath.Join(configDir, "CLAUDE.md"), Path: path.Join(in, "CLAUDE.md"), File: true},
	)
}

// skillDirs finds the skills under dir by name: the user's own (a folder
// with a SKILL.md), then the claude.ai ones synced under synced/<bucket>.
func skillDirs(dir string, synced bool) map[string]string {
	found := map[string]string{}
	add := func(d string) {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "synced" {
				continue
			}
			if _, taken := found[e.Name()]; taken {
				continue
			}
			if _, err := os.Stat(filepath.Join(d, e.Name(), "SKILL.md")); err == nil {
				found[e.Name()] = filepath.Join(d, e.Name())
			}
		}
	}
	add(dir)
	if !synced {
		return found
	}
	buckets, _ := os.ReadDir(filepath.Join(dir, "synced"))
	for _, b := range buckets {
		if b.IsDir() && !strings.HasPrefix(b.Name(), ".") {
			add(filepath.Join(dir, "synced", b.Name()))
		}
	}
	return found
}

// Commands a Linux container can run for an MCP server; anything else
// (a Windows program, a path on Windows) can't start there.
var linuxMCPCommands = map[string]bool{"npx": true, "node": true, "uvx": true, "uv": true, "python": true, "python3": true, "bunx": true, "deno": true, "pipx": true}

var windowsPathRE = regexp.MustCompile(`(?i)(^|[\s"'=])[a-z]:[\\/]|\\\\`)

// ContainerMCP picks the user's MCP servers (user scope, and those for the
// project in host) that can run in a Linux container: URL-based ones, and
// commands such as npx or uvx whose arguments name no Windows path. A
// Windows "cmd /c npx …" wrapper is unwrapped. skipped names the rest.
func ContainerMCP(claudeJSON []byte, host string) (servers map[string]json.RawMessage, skipped []string) {
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Projects   map[string]struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	servers = map[string]json.RawMessage{}
	if json.Unmarshal(claudeJSON, &cfg) != nil {
		return servers, nil
	}
	all := map[string]json.RawMessage{}
	for k, v := range cfg.MCPServers {
		all[k] = v
	}
	want := strings.ToLower(filepath.ToSlash(filepath.Clean(host)))
	for p, proj := range cfg.Projects {
		if strings.ToLower(filepath.ToSlash(filepath.Clean(p))) == want {
			for k, v := range proj.MCPServers {
				all[k] = v // the project's own win
			}
		}
	}
	for name, raw := range all {
		if s, ok := forLinux(raw); ok {
			servers[name] = s
		} else {
			skipped = append(skipped, name)
		}
	}
	sort.Strings(skipped)
	return servers, skipped
}

func forLinux(raw json.RawMessage) (json.RawMessage, bool) {
	var s map[string]any
	if json.Unmarshal(raw, &s) != nil {
		return nil, false
	}
	if u, _ := s["url"].(string); u != "" {
		return raw, true // http or sse: reachable from anywhere the network is
	}
	cmd, _ := s["command"].(string)
	args, _ := s["args"].([]any)
	if strings.EqualFold(strings.TrimSuffix(strings.ToLower(filepath.Base(cmd)), ".exe"), "cmd") && len(args) >= 2 {
		if a, _ := args[0].(string); strings.EqualFold(a, "/c") {
			cmd, _ = args[1].(string)
			args = args[2:]
		}
	}
	name := strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(filepath.Base(filepath.ToSlash(cmd))), ".cmd"), ".exe")
	if !linuxMCPCommands[name] {
		return nil, false
	}
	for _, a := range args {
		if str, _ := a.(string); windowsPathRE.MatchString(str) {
			return nil, false
		}
	}
	if env, _ := s["env"].(map[string]any); env != nil {
		for _, v := range env {
			if str, _ := v.(string); windowsPathRE.MatchString(str) {
				return nil, false
			}
		}
	}
	s["command"], s["args"] = name, args
	if s["args"] == nil {
		s["args"] = []any{}
	}
	b, err := json.Marshal(s)
	return b, err == nil
}

// WithMCP sets the MCP servers in a container's ~/.claude.json, keeping
// everything else the CLI keeps there.
func WithMCP(existing []byte, servers map[string]json.RawMessage) ([]byte, error) {
	doc := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(existing))) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			doc = map[string]json.RawMessage{} // unreadable: start again rather than fail the session
		}
	}
	b, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	doc["mcpServers"] = b
	return json.MarshalIndent(doc, "", "  ")
}

// ContainerMCPEnv makes the CLI wait for MCP servers before its first turn.
// By default it connects them in the background, so a server still
// connecting (a few seconds from a container, over WSL's network) has no
// tools in the session's first message. It waits at most 15 s, so a server
// that never answers only delays the start.
func ContainerMCPEnv() map[string]string {
	return map[string]string{"MCP_CONNECTION_NONBLOCKING": "0", "MCP_CONNECT_TIMEOUT_MS": "15000"}
}
