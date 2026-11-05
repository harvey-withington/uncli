package app

import (
	"context"
	"fmt"
	"time"

	"github.com/harvey-withington/uncli/app/internal/core"
)

// What a host learns about its UNCLI's AI providers (decision 0027): which
// a session can run on now and why not, and the models of each, for the
// host's own settings ("Claude Code · Sonnet"). It needs no page open.

// HostProvider is a provider as a host sees it.
type HostProvider struct {
	ID     string
	Name   string // the CLI's name: Claude Code
	Ready  bool   // a session can start on it now
	Reason string // why not, when it can't: in a sentence the host can show
	Models []core.ModelInfo
	// DefaultModel is what a new session on it starts on when it's given
	// none: for the default profile (chat) on the first provider, that
	// profile's model; "" where the CLI picks.
	DefaultModel string
}

// firstModels are the first provider's stable aliases, offered until its
// CLI has reported its models (as the app's pickers do).
var firstModels = []core.ModelInfo{
	{Value: "opus", DisplayName: "Opus"},
	{Value: "sonnet", DisplayName: "Sonnet"},
	{Value: "haiku", DisplayName: "Haiku"},
}

// HostProviders lists the providers with whether each is ready, and its
// models. Checking a CLI's sign-in can take a few seconds the first time.
func (s *Service) HostProviders(ctx context.Context) []HostProvider {
	out := []HostProvider{}
	for _, p := range s.Providers() {
		hp := HostProvider{ID: p.ID, Name: p.Name}
		st := CLIStatus{}
		hp.Reason, st = s.notReady(ctx, p)
		hp.Ready = hp.Reason == ""
		if p.ID == s.Adapter.ID() {
			hp.Models = s.Sessions.Models()
			if len(hp.Models) == 0 {
				hp.Models = firstModels
			}
			if chat, ok := s.Profiles.Profile("chat"); ok {
				hp.DefaultModel = chat.Model
			}
		} else {
			hp.Models = st.Models
		}
		var listed []core.ModelInfo
		for _, m := range hp.Models {
			if m.Value != "default" { // the CLI's own pick, which DefaultModel "" stands for
				listed = append(listed, m)
			}
		}
		hp.Models = listed
		out = append(out, hp)
	}
	return out
}

// notReady says why a session can't start on a provider now ("" when it
// can), with the CLI's status as checked.
func (s *Service) notReady(ctx context.Context, p Provider) (string, CLIStatus) {
	if err := s.pluginAllowed(p.ID); err != nil {
		return err.Error(), CLIStatus{}
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	st := s.ProviderStatus(ctx, p.ID, false)
	switch {
	case !st.Installed:
		return fmt.Sprintf("%s isn't installed in this UNCLI yet", p.Name), st
	case st.Error != "":
		return fmt.Sprintf("%s can't be used now: %s", p.Name, st.Error), st
	case !st.LoggedIn:
		return fmt.Sprintf("%s isn't signed in", p.Name), st
	}
	return "", st
}

// providerReady is notReady as an error, for a host choosing a provider
// for a new session; an unknown one says so.
func (s *Service) providerReady(ctx context.Context, id string) error {
	for _, p := range s.Providers() {
		if p.ID == id {
			if why, _ := s.notReady(ctx, p); why != "" {
				return fmt.Errorf("can't start a session on %s: %s", p.Name, why)
			}
			return nil
		}
	}
	return fmt.Errorf("no AI provider %q in this UNCLI", id)
}
