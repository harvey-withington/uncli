package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"uncli/internal/core"
	"uncli/internal/runtime/local"
	"uncli/internal/session"
)

// Every AI provider's CLI: its adapter and installer, the version the user
// runs, and whether it is installed and signed in. The first provider is
// also s.Adapter and s.Installer, which containers and quick tasks use.

type cliProvider struct {
	adapter   core.Adapter
	installer core.Installer
	binEnv    string // a binary to use instead of a managed one (development)

	mu       sync.Mutex
	auth     *core.AuthInfo
	models   []core.ModelInfo // listed by its sign-in check, for CLIs that do
	authTime time.Time
}

// modelLister is an adapter whose sign-in check lists the models the
// account can use (the Antigravity CLI's "models").
type modelLister interface {
	ParseModels(out []byte) []core.ModelInfo
}

func cliVersionKey(id string) string { return "cli." + id + ".version" } // empty = the pinned version
func cliPathKey(id string) string    { return "cli." + id + ".path" }    // a binary instead of a managed one

func (s *Service) cli(id string) (*cliProvider, error) {
	if id == "" {
		return s.clis[0], nil
	}
	for _, c := range s.clis {
		if c.adapter.ID() == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no AI provider %q", id)
}

// versionOf is the version the user runs: the pinned one unless they chose
// another at their own risk.
func (s *Service) versionOf(c *cliProvider) string {
	if v, _ := s.Store.Setting(cliVersionKey(c.adapter.ID())); v != "" {
		return v
	}
	return c.installer.Pinned()
}

func (s *Service) overrideOf(c *cliProvider) string {
	if c.binEnv != "" {
		if p := os.Getenv(c.binEnv); p != "" {
			return p
		}
	}
	p, _ := s.Store.Setting(cliPathKey(c.adapter.ID()))
	return p
}

// binaryOf is a provider's CLI binary and its version (session.Deps.Binary).
func (s *Service) binaryOf(ctx context.Context, id string) (string, string, error) {
	c, err := s.cli(id)
	if err != nil {
		return "", "", err
	}
	if p := s.overrideOf(c); p != "" {
		return p, "custom", nil
	}
	v := s.versionOf(c)
	if p, ok := c.installer.Path(v); ok {
		return p, v, nil
	}
	return "", "", session.ErrNoCLI
}

// ProviderStatus reports whether a provider's CLI is installed and signed
// in. The sign-in check runs the CLI, so its result is cached briefly.
func (s *Service) ProviderStatus(ctx context.Context, id string, fresh bool) CLIStatus {
	c, err := s.cli(id)
	if err != nil {
		return CLIStatus{Error: err.Error()}
	}
	st := CLIStatus{Provider: c.adapter.ID(), Version: s.versionOf(c), Pinned: c.installer.Pinned(), Custom: s.overrideOf(c) != ""}
	bin, _, err := s.binaryOf(ctx, c.adapter.ID())
	if err != nil {
		return st
	}
	st.Installed = true
	if st.Custom {
		st.Version = "custom"
	}
	c.mu.Lock()
	cached, models := c.auth, c.models
	if fresh || cached == nil || time.Since(c.authTime) > 30*time.Second {
		cached = nil
	}
	c.mu.Unlock()
	if cached == nil {
		info, ms, err := s.authStatusOf(ctx, c, bin)
		if err != nil {
			st.Error = err.Error()
			return st
		}
		c.mu.Lock()
		c.auth, c.models, c.authTime = &info, ms, time.Now()
		c.mu.Unlock()
		cached, models = &info, ms
	}
	st.LoggedIn, st.Email, st.Subscription = cached.LoggedIn, cached.Email, cached.Subscription
	st.Models = models
	return st
}

// authStatusOf asks the CLI whether it is signed in. Some exit 1 when
// signed out but still say so, so the output decides. The first run of a
// freshly downloaded binary can be slow (antivirus scans), so it gets a
// generous timeout and one retry.
func (s *Service) authStatusOf(ctx context.Context, c *cliProvider, bin string) (core.AuthInfo, []core.ModelInfo, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		actx, cancel := context.WithTimeout(ctx, 60*time.Second)
		out, err := local.Run(actx, c.adapter.AuthStatusCommand(bin))
		cancel()
		if info, perr := c.adapter.ParseAuthStatus(out); perr == nil {
			var ms []core.ModelInfo
			if ml, ok := c.adapter.(modelLister); ok {
				ms = ml.ParseModels(out)
			}
			return info, ms, nil
		}
		switch {
		case err != nil:
			lastErr = err
		case len(strings.TrimSpace(string(out))) == 0:
			lastErr = errors.New("it printed nothing")
		default:
			lastErr = fmt.Errorf("unexpected output %q", truncate(string(out), 120))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return core.AuthInfo{}, nil, fmt.Errorf("couldn't check whether the CLI is signed in (%v)", lastErr)
}

// InstallProvider downloads the selected version of a provider's CLI,
// reporting progress.
func (s *Service) InstallProvider(ctx context.Context, id string) (CLIStatus, error) {
	c, err := s.cli(id)
	if err != nil {
		return CLIStatus{}, err
	}
	pid := c.adapter.ID()
	_, err = c.installer.Ensure(ctx, s.versionOf(c), func(done, total int64) {
		s.emit.Emit(EvtCLIProgress, Progress{Provider: pid, Done: done, Total: total})
	})
	st := s.ProviderStatus(ctx, pid, true)
	if err != nil {
		return st, err
	}
	s.emitStatus(st)
	return st, nil
}

// emitStatus tells the UI a provider's status changed.
func (s *Service) emitStatus(st CLIStatus) {
	if st.Provider == "" || st.Provider == s.clis[0].adapter.ID() {
		s.emit.Emit(EvtCLIStatus, st)
	}
	s.emit.Emit(EvtProviderStatus, st)
}

// SetProviderVersion selects a version of a provider's CLI; empty returns
// to the pinned one.
func (s *Service) SetProviderVersion(ctx context.Context, id, version string) (CLIStatus, error) {
	c, err := s.cli(id)
	if err != nil {
		return CLIStatus{}, err
	}
	version = strings.TrimSpace(version)
	if version == c.installer.Pinned() {
		version = ""
	}
	if err := s.Store.SetSetting(cliVersionKey(c.adapter.ID()), version); err != nil {
		return CLIStatus{}, err
	}
	return s.InstallProvider(ctx, c.adapter.ID())
}

// ProviderChannels reports what a provider's release channels point at.
func (s *Service) ProviderChannels(ctx context.Context, id string) (map[string]string, error) {
	c, err := s.cli(id)
	if err != nil {
		return nil, err
	}
	return c.installer.Channels(ctx)
}
