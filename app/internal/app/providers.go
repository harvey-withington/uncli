package app

import (
	"fmt"
	"io/fs"

	"uncli/internal/adapter/manifest"
	"uncli/internal/core"

	"gopkg.in/yaml.v3"
)

// Provider is an AI provider: a CLI sessions run on (one per adapter), with
// the names the interface uses for it, since its own text names no CLI.
type Provider struct {
	ID      string `yaml:"id" json:"id"`
	Label   string `yaml:"-" json:"label"`         // the CLI's name (same as Name), for pickers
	Name    string `yaml:"name" json:"name"`       // the CLI: Claude Code
	Agent   string `yaml:"agent" json:"agent"`     // who acts in a session: Claude
	Account string `yaml:"account" json:"account"` // the account it signs in with: Claude
	// SignIn is how the CLI signs in: "link" (UNCLI opens its link and takes
	// the code back) or "elsewhere" (in the provider's own app or terminal,
	// whose sign-in the CLI shares).
	SignIn       string            `yaml:"signin" json:"signIn"`
	Capabilities core.Capabilities `yaml:"-" json:"capabilities"`
	Plugin       *PluginInfo       `yaml:"-" json:"plugin,omitempty"` // a provider plugin (decision 0013)
}

type providersFile struct {
	Providers []Provider `yaml:"providers"`
}

// loadProviders reads the built-in providers.yaml.
func loadProviders(defaults fs.FS) ([]Provider, error) {
	b, err := fs.ReadFile(defaults, "providers.yaml")
	if err != nil {
		return nil, err
	}
	var f providersFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("built-in providers.yaml: %w", err)
	}
	for i, p := range f.Providers {
		if p.ID == "" || p.Name == "" || p.Agent == "" {
			return nil, fmt.Errorf("providers.yaml: provider %d needs an id, name and agent", i+1)
		}
		f.Providers[i].Label = p.Name
	}
	return f.Providers, nil
}

// Providers lists the AI providers UNCLI has an adapter for, the first
// first, with the names providers.yaml gives them.
func (s *Service) Providers() []Provider {
	out := []Provider{}
	for _, c := range s.clis {
		id := c.adapter.ID()
		if _, ok := c.adapter.(*manifest.Adapter); ok {
			continue // a plugin: listed below, with what Settings shows about it
		}
		p := Provider{ID: id, Label: id, Name: id, Agent: id, Account: id}
		for _, q := range s.providers {
			if q.ID == id {
				p = q
			}
		}
		p.Capabilities = c.adapter.Capabilities()
		out = append(out, p)
	}
	for _, p := range s.plugins {
		out = append(out, s.pluginProvider(p))
	}
	return out
}
