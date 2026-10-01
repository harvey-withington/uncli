package local

import (
	"slices"
	"testing"

	"uncli/internal/core"
)

func TestEnvDropsAndOverrides(t *testing.T) {
	base := []string{"PATH=/bin", "CLAUDE_CODE_ENTRYPOINT=claude-vscode", "ClaudeCode=1", "CLAUDECODE=1", "HOME=/h",
		"DISABLE_AUTOUPDATER=0", "CLAUDE_EFFORT=max", "CLAUDE_CONFIG_DIR=/cfg"}
	got := Env(base, core.Command{
		Env:     map[string]string{"DISABLE_AUTOUPDATER": "1"},
		EnvDrop: []string{"CLAUDE_CODE_", "CLAUDECODE", "CLAUDE_EFFORT"},
	})
	slices.Sort(got)
	want := []string{"CLAUDE_CONFIG_DIR=/cfg", "DISABLE_AUTOUPDATER=1", "HOME=/h", "PATH=/bin"}
	if !slices.Equal(got, want) {
		t.Errorf("env = %v, want %v", got, want)
	}
}
