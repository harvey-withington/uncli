package profile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
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
	if !spec.Isolated || spec.SystemPrompt == "" || !slices.Contains(spec.Tools, "Write") || spec.Workdir != "/w" || !spec.ShareMCP || !spec.ShareConnectors {
		t.Errorf("chat spec = %+v", spec)
	}
	code, _ := s.Profile("code")
	if spec := s.Resolve(code, "opus", nil, "/r"); spec.PermissionMode != "acceptEdits" || len(spec.Tools) != 0 || spec.ShareMCP {
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

func TestSharingValues(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "profiles.yaml"), []byte("profiles:\n  - id: chat\n    isolated: true\n    mcp: everything\n"), 0o644)
	defaults, _ := fsSub()
	if _, err := Load(defaults, dir); err == nil || !strings.Contains(err.Error(), "mcp is shared or none") {
		t.Errorf("err = %v", err)
	}
}

// A modifier's tone is one the toolbar knows; the built-ins colour the
// verbosity group by cost.
func TestModifierTones(t *testing.T) {
	defaults, _ := fsSub()
	s, err := Load(defaults, "")
	if err != nil {
		t.Fatal(err)
	}
	if e, _ := s.Modifier("efficiency"); e.Tone != "success" {
		t.Errorf("efficiency tone = %q", e.Tone)
	}
	if th, _ := s.Modifier("thorough"); th.Tone != "warning" {
		t.Errorf("thorough tone = %q", th.Tone)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "modifiers.yaml"), []byte("modifiers:\n  - id: loud\n    label: Loud\n    tone: red\n"), 0o644)
	if _, err := Load(defaults, dir); err == nil || !strings.Contains(err.Error(), "tone is success, warning or accent") {
		t.Errorf("err = %v", err)
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
	if !strings.Contains(got, `The earlier instruction "Use the minimum tokens and turns needed. No preamble, no recap." no longer applies`) {
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

// A session's choice of tools gives what its type shares of the MCP
// servers and connectors, or both when the type shares neither.
func TestApplyShares(t *testing.T) {
	for name, tc := range map[string]struct {
		p                 Profile
		shares            []string
		mcp, conn, skills bool
	}{
		"chat, all":            {Profile{MCP: Shared, Connectors: Shared}, []string{ShareTools, ShareSkills}, true, true, true},
		"chat, none":           {Profile{MCP: Shared, Connectors: Shared}, []string{}, false, false, false},
		"servers only by type": {Profile{MCP: Shared}, []string{ShareTools}, true, false, false},
		"connectors only":      {Profile{MCP: None, Connectors: Shared}, []string{ShareTools}, false, true, false},
		"type shares neither":  {Profile{}, []string{ShareTools}, true, true, false},
		"skills without tools": {Profile{MCP: Shared}, []string{ShareSkills}, false, false, true},
	} {
		t.Run(name, func(t *testing.T) {
			var spec core.LaunchSpec
			ApplyShares(&spec, tc.p, tc.shares)
			if spec.ShareMCP != tc.mcp || spec.ShareConnectors != tc.conn || spec.ShareSkills != tc.skills {
				t.Errorf("got mcp %v connectors %v skills %v", spec.ShareMCP, spec.ShareConnectors, spec.ShareSkills)
			}
		})
	}
	s := load(t, nil)
	chat, _ := s.Profile("chat")
	code, _ := s.Profile("code")
	if !slices.Equal(chat.Shares, []string{ShareTools, ShareSkills}) || len(code.Shares) != 0 || code.Shares == nil {
		t.Errorf("defaults: chat %v code %v", chat.Shares, code.Shares)
	}
}
