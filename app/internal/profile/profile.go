// Package profile loads profiles, modifiers and the toolbar from YAML,
// resolves them into a LaunchSpec, and renders turn-scope directives.
package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"uncli/internal/core"
)

type Profile struct {
	ID              string   `yaml:"id" json:"id"`
	Label           string   `yaml:"label" json:"label"`
	Icon            string   `yaml:"icon" json:"icon"`
	Hue             int      `yaml:"hue" json:"hue"`       // icon and accent colour: HSL hue, drawn at 100% saturation; 0 = the app accent
	Folder          string   `yaml:"folder" json:"folder"` // scratch | pick | repo
	Model           string   `yaml:"model" json:"model"`
	SystemPrompt    string   `yaml:"system_prompt" json:"-"`
	AppendPrompt    string   `yaml:"append_prompt" json:"-"`
	Tools           []string `yaml:"tools" json:"tools"`
	AllowedTools    []string `yaml:"allowed_tools" json:"-"`
	DisallowedTools []string `yaml:"disallowed_tools" json:"-"`
	Isolated        bool     `yaml:"isolated" json:"isolated"`
	MCPConfig       []string `yaml:"mcp_config" json:"-"` // extra MCP server configs (--mcp-config); relative paths are from the session folder
	// What an isolated profile's sessions get of the user's own by default:
	// MCP is "shared" for the MCP servers they set up for the CLI,
	// Connectors for their account's connectors, Skills for their skills;
	// "none" (the default) for none. Each session can change it (Shares).
	MCP            string   `yaml:"mcp" json:"-"`
	Connectors     string   `yaml:"connectors" json:"-"`
	Skills         string   `yaml:"skills" json:"-"`
	Shares         []string `yaml:"-" json:"shares"` // DefaultShares, for the UI
	PermissionMode string   `yaml:"permission_mode" json:"-"`
	ModifiersOn    []string `yaml:"modifiers_on" json:"modifiersOn"`
	IDELinks       bool     `yaml:"ide_links" json:"ideLinks"`
	Artifacts      bool     `yaml:"artifacts" json:"artifacts"` // versions ./artifacts/ per page and shows it in the artifact pane
}

type Modifier struct {
	ID    string `yaml:"id" json:"id"`
	Label string `yaml:"label" json:"label"`
	Icon  string `yaml:"icon" json:"icon"`
	Scope string `yaml:"scope" json:"scope"` // turn | system
	Group string `yaml:"group" json:"group,omitempty"`
	// Tone colours it in its group's toggle by what it costs: success
	// (cheaper), warning (dearer), accent; empty is neutral.
	Tone          string            `yaml:"tone" json:"tone,omitempty"`
	RequiresTools []string          `yaml:"requires_tools" json:"requiresTools,omitempty"`
	Settings      map[string]string `yaml:"settings" json:"settings,omitempty"`
	Text          map[string]string `yaml:"text" json:"-"`
}

type ToolbarItem struct {
	Kind    string `yaml:"kind" json:"kind"` // model_picker | modifiers | separator | slash | native
	Label   string `yaml:"label" json:"label,omitempty"`
	Icon    string `yaml:"icon" json:"icon,omitempty"`
	Command string `yaml:"command" json:"command,omitempty"`
	Action  string `yaml:"action" json:"action,omitempty"`
}

type Set struct {
	Profiles  []Profile     `json:"profiles"`
	Modifiers []Modifier    `json:"modifiers"`
	Toolbar   []ToolbarItem `json:"toolbar"`
}

type profilesFile struct {
	Profiles []Profile `yaml:"profiles"`
}
type modifiersFile struct {
	Modifiers []Modifier `yaml:"modifiers"`
}
type toolbarFile struct {
	Toolbar []ToolbarItem `yaml:"toolbar"`
}

// Load reads the built-in files from defaults (profiles.yaml, modifiers.yaml,
// toolbar.yaml at its root) and merges the user's files from userDir over
// them: profiles and modifiers by id, the toolbar as a whole.
func Load(defaults fs.FS, userDir string) (*Set, error) {
	var s Set
	var pf profilesFile
	var mf modifiersFile
	var tf toolbarFile
	for name, dst := range map[string]any{"profiles.yaml": &pf, "modifiers.yaml": &mf, "toolbar.yaml": &tf} {
		b, err := fs.ReadFile(defaults, name)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(b, dst); err != nil {
			return nil, fmt.Errorf("built-in %s: %w", name, err)
		}
	}
	s.Profiles, s.Modifiers, s.Toolbar = pf.Profiles, mf.Modifiers, tf.Toolbar

	if userDir != "" {
		var upf profilesFile
		var umf modifiersFile
		var utf toolbarFile
		for name, dst := range map[string]any{"profiles.yaml": &upf, "modifiers.yaml": &umf, "toolbar.yaml": &utf} {
			b, err := os.ReadFile(filepath.Join(userDir, name))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(b, dst); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Join(userDir, name), err)
			}
		}
		s.Profiles = mergeByID(s.Profiles, upf.Profiles, func(p Profile) string { return p.ID })
		s.Modifiers = mergeByID(s.Modifiers, umf.Modifiers, func(m Modifier) string { return m.ID })
		if len(utf.Toolbar) > 0 {
			s.Toolbar = utf.Toolbar
		}
	}
	for i := range s.Profiles {
		s.Profiles[i].Shares = s.Profiles[i].DefaultShares()
	}
	return &s, s.validate()
}

func mergeByID[T any](base, over []T, id func(T) string) []T {
	out := slices.Clone(base)
	for _, o := range over {
		if i := slices.IndexFunc(out, func(b T) bool { return id(b) == id(o) }); i >= 0 {
			out[i] = o
		} else {
			out = append(out, o)
		}
	}
	return out
}

func (s *Set) validate() error {
	for _, p := range s.Profiles {
		if p.ID == "" {
			return errors.New("a profile has no id")
		}
		if p.PermissionMode == "bypassPermissions" {
			return fmt.Errorf("profile %s: bypassPermissions is never allowed as a default", p.ID)
		}
		for name, v := range map[string]string{"mcp": p.MCP, "connectors": p.Connectors, "skills": p.Skills} {
			if v != "" && v != Shared && v != None {
				return fmt.Errorf("profile %s: %s is shared or none, not %q", p.ID, name, v)
			}
		}
	}
	for _, m := range s.Modifiers {
		if m.ID == "" {
			return errors.New("a modifier has no id")
		}
		switch m.Tone {
		case "", "success", "warning", "accent":
		default:
			return fmt.Errorf("modifier %s: tone is success, warning or accent, not %q", m.ID, m.Tone)
		}
	}
	return nil
}

func (s *Set) Profile(id string) (Profile, bool) {
	i := slices.IndexFunc(s.Profiles, func(p Profile) bool { return p.ID == id })
	if i < 0 {
		return Profile{}, false
	}
	return s.Profiles[i], true
}

func (s *Set) Modifier(id string) (Modifier, bool) {
	i := slices.IndexFunc(s.Modifiers, func(m Modifier) bool { return m.ID == id })
	if i < 0 {
		return Modifier{}, false
	}
	return s.Modifiers[i], true
}

// Available reports whether a modifier can be used with a profile: its
// required tools must exist in the profile's tool set (empty = CLI defaults).
func (s *Set) Available(p Profile, m Modifier) bool {
	if len(p.Tools) == 0 {
		return true
	}
	for _, t := range m.RequiresTools {
		if !slices.Contains(p.Tools, t) {
			return false
		}
	}
	return true
}

// Toggle switches a modifier on or off, keeping groups exclusive, and
// returns the new active list in modifier order.
func (s *Set) Toggle(active []string, id string, on bool) []string {
	m, ok := s.Modifier(id)
	if !ok {
		return active
	}
	next := map[string]bool{}
	for _, a := range active {
		next[a] = true
	}
	if on {
		if m.Group != "" {
			for _, other := range s.Modifiers {
				if other.Group == m.Group {
					delete(next, other.ID)
				}
			}
		}
		next[id] = true
	} else {
		delete(next, id)
	}
	return s.ordered(next)
}

func (s *Set) ordered(set map[string]bool) []string {
	out := []string{}
	for _, m := range s.Modifiers {
		if set[m.ID] {
			out = append(out, m.ID)
		}
	}
	return out
}

// Values of a profile's mcp, connectors and skills.
const (
	Shared = "shared"
	None   = "none"
)

// What a session can share of the user's own (store.Session.Shares): their
// MCP servers and connectors together, and their skills.
const (
	ShareTools  = "tools"
	ShareSkills = "skills"
)

// DefaultShares is what a new session of an isolated profile shares.
func (p Profile) DefaultShares() []string {
	out := []string{}
	if !p.Isolated {
		return out
	}
	if p.MCP == Shared || p.Connectors == Shared {
		out = append(out, ShareTools)
	}
	if p.Skills == Shared {
		out = append(out, ShareSkills)
	}
	return out
}

// ApplyShares sets what an isolated spec shares from a session's choice.
// Tools on gives what the profile shares of the MCP servers and the
// connectors, or both when it shares neither by default.
func ApplyShares(spec *core.LaunchSpec, p Profile, shares []string) {
	tools := slices.Contains(shares, ShareTools)
	either := p.MCP == Shared || p.Connectors == Shared
	spec.ShareMCP = tools && (p.MCP == Shared || !either)
	spec.ShareConnectors = tools && (p.Connectors == Shared || !either)
	spec.ShareSkills = slices.Contains(shares, ShareSkills)
}

// Resolve turns a profile, model and active modifiers into a LaunchSpec.
// Session ids are filled in by the session.
func (s *Set) Resolve(p Profile, model string, active []string, workdir string) core.LaunchSpec {
	spec := core.LaunchSpec{
		Model: model, Workdir: workdir,
		SystemPrompt: strings.TrimSpace(p.SystemPrompt), AppendPrompt: strings.TrimSpace(p.AppendPrompt),
		Tools: p.Tools, AllowedTools: p.AllowedTools, DisallowedTools: p.DisallowedTools,
		PermissionMode: p.PermissionMode, Isolated: p.Isolated, MCPConfig: p.MCPConfig,
	}
	ApplyShares(&spec, p, p.DefaultShares())
	for _, id := range active {
		if m, ok := s.Modifier(id); ok {
			if e := m.Settings["effort"]; e != "" {
				spec.Effort = e
			}
		}
	}
	return spec
}

// textFor picks a modifier's text for a model: the first glob that matches
// the model alias or id (with or without the "claude-" prefix), else default.
func textFor(m Modifier, model string) string {
	candidates := []string{model, strings.TrimPrefix(model, "claude-")}
	keys := make([]string, 0, len(m.Text))
	for k := range m.Text {
		if k != "default" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		for _, c := range candidates {
			if ok, _ := path.Match(k, c); ok {
				return m.Text[k]
			}
		}
	}
	return m.Text["default"]
}

// RenderDirectives builds the hidden block sent before the user's text. It
// restates every active turn-scope modifier, and says when one switched off
// since the previous turn no longer applies, then any extra lines the
// session adds. Empty when there is nothing to say.
func (s *Set) RenderDirectives(model string, active, previous []string, extra ...string) string {
	var lines []string
	for _, id := range active {
		m, ok := s.Modifier(id)
		if !ok || m.Scope == "system" {
			continue
		}
		if t := strings.TrimSpace(textFor(m, model)); t != "" {
			lines = append(lines, "- "+t)
		}
	}
	for _, id := range previous {
		if slices.Contains(active, id) {
			continue
		}
		// Quote what the model was told (its label it never saw).
		if m, ok := s.Modifier(id); ok && m.Scope != "system" {
			said := strings.TrimSpace(textFor(m, model))
			if said == "" {
				said = m.Label
			}
			lines = append(lines, fmt.Sprintf("- The earlier instruction %q no longer applies; answer as you normally would.", said))
		}
	}
	for _, t := range extra {
		lines = append(lines, "- "+t)
	}
	if len(lines) == 0 {
		return ""
	}
	return "<session_directives>\n" + strings.Join(lines, "\n") + "\n</session_directives>"
}
