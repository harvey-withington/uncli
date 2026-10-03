package profile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func load(t *testing.T, userFiles map[string]string) *Set {
	t.Helper()
	dir := t.TempDir()
	for name, body := range userFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	defaults, err := fsSub()
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(defaults, dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBuiltIns(t *testing.T) {
	s := load(t, nil)
	var ids []string
	for _, p := range s.Profiles {
		ids = append(ids, p.ID)
	}
	if !slices.Equal(ids, []string{"chat", "cowork", "code"}) {
		t.Errorf("profiles = %v", ids)
	}
	chat, _ := s.Profile("chat")
	spec := s.Resolve(chat, "sonnet", nil, "/w")
	if !spec.Isolated || spec.SystemPrompt == "" || !slices.Contains(spec.Tools, "Write") || spec.Workdir != "/w" {
		t.Errorf("chat spec = %+v", spec)
	}
	code, _ := s.Profile("code")
	if spec := s.Resolve(code, "opus", nil, "/r"); spec.PermissionMode != "acceptEdits" || len(spec.Tools) != 0 {
		t.Errorf("code spec = %+v", spec)
	}
	if len(s.Toolbar) == 0 || s.Toolbar[0].Kind != "model_picker" {
		t.Errorf("toolbar = %+v", s.Toolbar)
	}
	slash := 0
	for _, it := range s.Toolbar {
		if it.Kind == "slash" {
			slash++
		}
	}
	if slash != 2 {
		t.Errorf("phase 1 toolbar should have two slash commands, has %d", slash)
	}
}

func TestUserOverridesByID(t *testing.T) {
	s := load(t, map[string]string{
		"profiles.yaml":  "profiles:\n  - id: chat\n    label: My chat\n    model: haiku\n  - id: writing\n    label: Writing\n",
		"modifiers.yaml": "modifiers:\n  - id: pirate\n    label: Pirate\n    scope: turn\n    text: { default: Talk like a pirate. }\n",
	})
	chat, _ := s.Profile("chat")
	if chat.Label != "My chat" || chat.Model != "haiku" {
		t.Errorf("override = %+v", chat)
	}
	if _, ok := s.Profile("writing"); !ok || len(s.Profiles) != 4 {
		t.Error("new user profile should be appended")
	}
	if _, ok := s.Modifier("pirate"); !ok || len(s.Modifiers) != 4 {
		t.Error("new user modifier should be appended")
	}
}

func TestBypassRefused(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "profiles.yaml"), []byte("profiles:\n  - id: yolo\n    permission_mode: bypassPermissions\n"), 0o644)
	defaults, _ := fsSub()
	if _, err := Load(defaults, dir); err == nil {
		t.Error("bypassPermissions must be refused")
	}
}

func TestToggleGroups(t *testing.T) {
	s := load(t, nil)
	a := s.Toggle(nil, "efficiency", true)
	a = s.Toggle(a, "use-agents", true)
	if !slices.Equal(a, []string{"use-agents", "efficiency"}) {
		t.Errorf("active = %v", a)
	}
	a = s.Toggle(a, "thorough", true)
	if !slices.Equal(a, []string{"use-agents", "thorough"}) {
		t.Errorf("thorough should replace efficiency in its group: %v", a)
	}
	a = s.Toggle(a, "use-agents", false)
	if !slices.Equal(a, []string{"thorough"}) {
		t.Errorf("active = %v", a)
	}
}

func TestRenderDirectives(t *testing.T) {
	s := load(t, nil)
	if got := s.RenderDirectives("sonnet", nil, nil); got != "" {
		t.Errorf("nothing active should render nothing, got %q", got)
	}
	got := s.RenderDirectives("sonnet", []string{"efficiency"}, nil)
	if !strings.HasPrefix(got, "<session_directives>\n") || !strings.Contains(got, "minimum tokens") {
		t.Errorf("efficiency = %q", got)
	}
	got = s.RenderDirectives("sonnet", nil, []string{"efficiency"})
	if !strings.Contains(got, `"Efficiency Mode" instruction no longer applies`) {
		t.Errorf("switched off = %q", got)
	}
	if got := s.RenderDirectives("sonnet", nil, nil, "The user is away."); got != "<session_directives>\n- The user is away.\n</session_directives>" {
		t.Errorf("extra lines = %q", got)
	}
	for _, model := range []string{"haiku", "claude-haiku-4-5-20251001"} {
		if got := s.RenderDirectives(model, []string{"use-agents"}, nil); !strings.Contains(got, "Keep each delegation brief") {
			t.Errorf("%s should get the haiku text: %q", model, got)
		}
	}
	if got := s.RenderDirectives("opus", []string{"use-agents"}, nil); !strings.Contains(got, "wherever work can be parallelised") {
		t.Errorf("opus should get default text: %q", got)
	}
}

func TestEffortFromModifierSettings(t *testing.T) {
	s := load(t, nil)
	code, _ := s.Profile("code")
	if e := s.Resolve(code, "opus", []string{"thorough"}, "").Effort; e != "high" {
		t.Errorf("effort = %q", e)
	}
	if e := s.Resolve(code, "opus", []string{"efficiency"}, "").Effort; e != "" {
		t.Errorf("effort = %q", e)
	}
}

func TestAvailable(t *testing.T) {
	s := load(t, nil)
	chat, _ := s.Profile("chat")
	code, _ := s.Profile("code")
	agents, _ := s.Modifier("use-agents")
	if s.Available(chat, agents) {
		t.Error("chat has no Task tool, so Use Agents is unavailable")
	}
	if !s.Available(code, agents) {
		t.Error("code uses CLI default tools, which include Task")
	}
}
