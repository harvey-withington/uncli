package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// An isolated session reads none of the user's settings, so it doesn't
// find their skills either. The CLI loads a plugin from any folder
// (--plugin-dir) with all setting sources off, but only from inside that
// folder: a plugin.json naming the user's skill folders loads nothing (CLI
// 2.1.285). So UNCLI keeps a plugin of its own whose skills are links to
// the user's: their own skills and the ones synced from claude.ai, which
// the CLI keeps per account (skillDirs). A link follows the folder, so an
// edited skill needs nothing done here; the links are checked each time a
// session starts, for skills added or removed since.

// SkillsPlugin is the plugin's name, which the CLI puts before each skill's
// (my:docx).
const SkillsPlugin = "my"

// skillsPlugin brings the plugin folder up to date and returns it; empty
// when there's no folder for it or the user has no skills.
func (a *Adapter) skillsPlugin() (string, error) {
	if a.skillsDir == "" {
		return "", nil
	}
	cfg, err := configDir()
	if err != nil {
		return "", nil
	}
	want := skillDirs(filepath.Join(cfg, "skills"), true)
	if len(want) == 0 {
		return "", nil
	}
	a.skillsMu.Lock()
	defer a.skillsMu.Unlock()
	if err := syncSkills(a.skillsDir, want); err != nil {
		return "", err
	}
	return a.skillsDir, nil
}

// syncSkills makes root a plugin whose skills are links to want (name to
// folder): links that point elsewhere or to a skill that's gone are
// removed (only ever the link, never what it points to), missing ones made.
func syncSkills(root string, want map[string]string) error {
	manifest, _ := json.Marshal(map[string]string{"name": SkillsPlugin, "version": "1.0.0",
		"description": "Your own skills, for sessions that read none of your settings."})
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), manifest, 0o644); err != nil {
		return err
	}
	dir := filepath.Join(root, "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	have := map[string]bool{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		target, err := os.Readlink(p)
		if err != nil {
			continue // not a link: not UNCLI's, left alone
		}
		if w, ok := want[e.Name()]; ok && samePath(target, w) {
			have[e.Name()] = true
			continue
		}
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	for name, target := range want {
		if have[name] {
			continue
		}
		if err := link(target, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(strings.TrimPrefix(a, `\??\`)), filepath.Clean(b)
	if filepath.Separator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}
