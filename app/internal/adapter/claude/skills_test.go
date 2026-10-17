package claude

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
)

func skill(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The plugin's skills are links that follow the user's: added, pointed
// elsewhere and removed, without ever touching what they point to or a
// folder that isn't a link.
func TestSyncSkills(t *testing.T) {
	src, root := t.TempDir(), t.TempDir()
	a, b, a2 := skill(t, src, "a"), skill(t, src, "b"), skill(t, src, "a2")
	if err := syncSkills(root, map[string]string{"a": a, "b": b}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "skills", "a", "SKILL.md")); err != nil || !strings.Contains(string(got), "name: a") {
		t.Fatalf("a through its link: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(root, "skills", "not-a-link")
	os.MkdirAll(mine, 0o755)

	if err := syncSkills(root, map[string]string{"a": a2}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "skills", "a", "SKILL.md")); !strings.Contains(string(got), "name: a2") {
		t.Errorf("a should now be a2: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "skills", "b")); !os.IsNotExist(err) {
		t.Errorf("b's link should be gone: %v", err)
	}
	for _, p := range []string{filepath.Join(b, "SKILL.md"), filepath.Join(a, "SKILL.md"), mine} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should be untouched: %v", p, err)
		}
	}
}

func TestBuildCommandSharesSkills(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	skill(t, filepath.Join(cfg, "skills"), "notes")
	skill(t, filepath.Join(cfg, "skills", "synced", "org_account"), "docx")
	a := New(nil).WithSkills(filepath.Join(t.TempDir(), "plugin"))
	a.userConfig = func() []byte { return nil }
	spec := core.LaunchSpec{Isolated: true, Tools: []string{"WebSearch", "Write"}, ShareSkills: true}

	cmd, err := a.BuildCommand("x", spec)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(cmd.Args, "|")
	if !strings.Contains(args, "--tools|WebSearch,Write,Skill,Read|") || !strings.Contains(args, "--plugin-dir|"+a.skillsDir) ||
		strings.Contains(args, "--disable-slash-commands") || cmd.Env["CLAUDE_CODE_DISABLE_BUNDLED_SKILLS"] != "1" {
		t.Errorf("args = %s env = %v", args, cmd.Env)
	}
	for _, name := range []string{"notes", "docx"} {
		if _, err := os.Stat(filepath.Join(a.skillsDir, "skills", name, "SKILL.md")); err != nil {
			t.Errorf("%s not in the plugin: %v", name, err)
		}
	}

	spec.ShareSkills = false
	cmd, _ = a.BuildCommand("x", spec)
	if !slices.Contains(cmd.Args, "--disable-slash-commands") || slices.Contains(cmd.Args, "--plugin-dir") || strings.Contains(strings.Join(cmd.Args, "|"), "Skill") {
		t.Errorf("without skills: %q", cmd.Args)
	}
	// A session sharing tools gets ToolSearch, so their descriptions load
	// only when needed.
	spec.ShareMCP = true
	cmd, _ = a.BuildCommand("x", spec)
	if !strings.Contains(strings.Join(cmd.Args, "|"), "--tools|WebSearch,Write,ToolSearch|") {
		t.Errorf("with tools: %q", cmd.Args)
	}
	// No folder for the plugin: no skills, rather than a failed start.
	cmd, err = New(nil).BuildCommand("x", core.LaunchSpec{Isolated: true, ShareSkills: true})
	if err != nil || !slices.Contains(cmd.Args, "--disable-slash-commands") {
		t.Errorf("no plugin folder: %q %v", cmd.Args, err)
	}
}
