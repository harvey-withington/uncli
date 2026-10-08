package app

import (
	"fmt"
	"io/fs"

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

// Providers lists the AI providers UNCLI has an adapter for.
func (s *Service) Providers() []Provider {
	for _, p := range s.providers {
		if p.ID == s.Adapter.ID() {
			return []Provider{p}
		}
	}
	id := s.Adapter.ID()
	return []Provider{{ID: id, Label: id, Name: id, Agent: id}}
}
