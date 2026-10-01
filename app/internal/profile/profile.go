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
	Folder          string   `yaml:"folder" json:"folder"` // scratch | pick | repo
	Model           string   `yaml:"model" json:"model"`
	SystemPrompt    string   `yaml:"system_prompt" json:"-"`
	AppendPrompt    string   `yaml:"append_prompt" json:"-"`
	Tools           []string `yaml:"tools" json:"tools"`
	AllowedTools    []string `yaml:"allowed_tools" json:"-"`
	DisallowedTools []string `yaml:"disallowed_tools" json:"-"`
	Isolated        bool     `yaml:"isolated" json:"-"`
	PermissionMode  string   `yaml:"permission_mode" json:"-"`
	ModifiersOn     []string `yaml:"modifiers_on" json:"modifiersOn"`
	IDELinks        bool     `yaml:"ide_links" json:"ideLinks"`
}

type Modifier struct {
	ID            string            `yaml:"id" json:"id"`
	Label         string            `yaml:"label" json:"label"`
	Icon          string            `yaml:"icon" json:"icon"`
	Scope         string            `yaml:"scope" json:"scope"` // turn | system
	Group         string            `yaml:"group" json:"group,omitempty"`
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
	}
	for _, m := range s.Modifiers {
		if m.ID == "" {
			return errors.New("a modifier has no id")
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

// Resolve turns a profile, model and active modifiers into a LaunchSpec.
// Session ids are filled in by the session.
func (s *Set) Resolve(p Profile, model string, active []string, workdir string) core.LaunchSpec {
	spec := core.LaunchSpec{
		Model: model, Workdir: workdir,
		SystemPrompt: strings.TrimSpace(p.SystemPrompt), AppendPrompt: strings.TrimSpace(p.AppendPrompt),
		Tools: p.Tools, AllowedTools: p.AllowedTools, DisallowedTools: p.DisallowedTools,
		PermissionMode: p.PermissionMode, Isolated: p.Isolated,
	}
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
// since the previous turn no longer applies. Empty when there is nothing to say.
func (s *Set) RenderDirectives(model string, active, previous []string) string {
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
		if m, ok := s.Modifier(id); ok && m.Scope != "system" {
			lines = append(lines, fmt.Sprintf("- The earlier %q instruction no longer applies; answer as you normally would.", m.Label))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "<session_directives>\n" + strings.Join(lines, "\n") + "\n</session_directives>"
}
