package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"uncli/internal/adapter/manifest"
	"uncli/internal/core"
)

// Provider plugins (decision 0013): a CLI described by a provider.yaml in
// <config>/providers/<id>/, run by the generic manifest adapter beside the
// built-in ones. A plugin starts disabled: until the user enables it (this
// exact manifest, by its hash), UNCLI neither downloads nor runs its CLI,
// nor offers it for a session. One that can't be read is listed with its
// error and does nothing else.

const settingPlugins = "plugins.enabled" // provider id → the manifest hash the user enabled

// PluginInfo is what Settings shows about a plugin.
type PluginInfo struct {
	Folder  string `json:"folder"`
	Hash    string `json:"hash,omitempty"`
	Enabled bool   `json:"enabled"`
	// Changed: enabled before, but its manifest has changed since.
	Changed bool `json:"changed,omitempty"`
	// Bypasses: it runs its CLI with the CLI's own permission checks off,
	// UNCLI's hook deciding every tool call instead (decision 0012).
	Bypasses bool     `json:"bypasses"`
	Hooked   bool     `json:"hooked"`            // UNCLI's hook decides its tool calls
	Command  []string `json:"command,omitempty"` // what it runs for a session, before per-session flags
	Sources  []string `json:"sources,omitempty"` // where its CLI is downloaded from (hosts)
	Error    string   `json:"error,omitempty"`   // why it can't be used
}

// plugin is a loaded plugin, or one that failed to load.
type plugin struct {
	id     string
	folder string
	m      *manifest.Manifest
	hash   string
	err    error
}

func (s *Service) pluginsDir() string { return filepath.Join(s.Paths.Config, "providers") }

// loadPlugins reads every plugin folder. builtin are the ids plugins can't use.
func (s *Service) loadPlugins(builtin []string) []*plugin {
	entries, err := os.ReadDir(s.pluginsDir())
	if err != nil {
		return nil
	}
	var out []*plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.pluginsDir(), e.Name())
		if _, err := os.Stat(filepath.Join(dir, manifest.File)); err != nil {
			continue
		}
		p := &plugin{id: e.Name(), folder: dir}
		p.m, p.hash, p.err = manifest.Load(dir)
		switch {
		case p.err != nil:
		case p.m.ID != e.Name():
			p.err = fmt.Errorf("its id is %q but its folder is %q: they must be the same", p.m.ID, e.Name())
		case contains(builtin, p.m.ID):
			p.err = fmt.Errorf("%q is a built-in provider's id", p.m.ID)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (s *Service) enabledPlugins() map[string]string {
	m := map[string]string{}
	if raw, _ := s.Store.Setting(settingPlugins); raw != "" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	return m
}

func (s *Service) plugin(id string) (*plugin, bool) {
	for _, p := range s.plugins {
		if p.id == id {
			return p, true
		}
	}
	return nil, false
}

// pluginAllowed: nil when the plugin is enabled as its manifest is now.
// Only a plugin that loaded runs as a provider; one that didn't (whatever
// id it claims, a built-in's included) gates nothing.
func (s *Service) pluginAllowed(id string) error {
	p, ok := s.plugin(id)
	if !ok || p.err != nil {
		return nil
	}
	switch h, ok := s.enabledPlugins()[id]; {
	case !ok:
		return fmt.Errorf("the %s plugin isn't enabled: enable it in Settings → AI Providers", p.m.Name)
	case h != p.hash:
		return fmt.Errorf("the %s plugin has changed since it was enabled: look at it again in Settings → AI Providers", p.m.Name)
	}
	return nil
}

// pluginInfo is what Settings shows about a plugin.
func (s *Service) pluginInfo(p *plugin) *PluginInfo {
	info := &PluginInfo{Folder: p.folder}
	if p.err != nil {
		info.Error = p.err.Error()
		return info
	}
	info.Hash = p.hash
	h, was := s.enabledPlugins()[p.id]
	info.Enabled = was && h == p.hash
	info.Changed = was && h != p.hash
	info.Bypasses, info.Hooked = p.m.Bypasses(), p.m.HookGated()
	info.Command = append([]string{p.m.Install.Binary}, p.m.Launch.Args...)
	if p.m.Bypasses() {
		info.Command = append(info.Command, p.m.Launch.Approvals...)
	}
	hosts := map[string]bool{}
	for _, plats := range p.m.Install.Builds {
		for _, b := range plats {
			if u, err := url.Parse(b.URL); err == nil {
				hosts[u.Host] = true
			}
		}
	}
	if l := p.m.Install.Latest; l != nil {
		if u, err := url.Parse(l.URL); err == nil {
			hosts[u.Host+" (latest)"] = true
		}
	}
	for h := range hosts {
		info.Sources = append(info.Sources, h)
	}
	sort.Strings(info.Sources)
	return info
}

// EnablePlugin lets a plugin run, as its manifest is now: hash is the one
// Settings showed, so a manifest that changed in the meantime isn't enabled
// unseen.
func (s *Service) EnablePlugin(id, hash string) error {
	p, ok := s.plugin(id)
	if !ok {
		return fmt.Errorf("no plugin %q", id)
	}
	if p.err != nil {
		return p.err
	}
	if _, cur, err := manifest.Load(p.folder); err != nil || cur != p.hash {
		return errors.New("the plugin's manifest changed after UNCLI started: restart UNCLI to look at it again")
	}
	if hash != p.hash {
		return errors.New("the plugin's manifest has changed: look at it again")
	}
	return s.setPluginEnabled(id, hash)
}

// DisablePlugin stops a plugin from being used; its sessions stay, and
// can't continue until it's enabled again.
func (s *Service) DisablePlugin(id string) error {
	if _, ok := s.plugin(id); !ok {
		return fmt.Errorf("no plugin %q", id)
	}
	return s.setPluginEnabled(id, "")
}

func (s *Service) setPluginEnabled(id, hash string) error {
	m := s.enabledPlugins()
	if hash == "" {
		delete(m, id)
	} else {
		m[id] = hash
	}
	b, _ := json.Marshal(m)
	if err := s.Store.SetSetting(settingPlugins, string(b)); err != nil {
		return err
	}
	if c, err := s.cli(id); err == nil {
		c.mu.Lock()
		c.auth, c.models = nil, nil // checked afresh
		c.mu.Unlock()
	}
	return nil
}

// pluginAdapters builds the adapters for the plugins that loaded; their
// CLIs only run while enabled.
func (s *Service) pluginAdapters() []*cliProvider {
	var out []*cliProvider
	for _, p := range s.plugins {
		if p.err != nil {
			continue
		}
		inst := manifest.NewInstaller(p.m, filepath.Join(s.Paths.Cache, "cli", p.id))
		a := manifest.New(p.m, inst, filepath.Join(s.Paths.Config, "provider-state", p.id), hookCommand())
		id := p.id
		a.Allowed = func() error { return s.pluginAllowed(id) }
		out = append(out, &cliProvider{adapter: a, installer: inst, allowed: a.Allowed})
	}
	return out
}

// pluginProvider is a plugin as AI Providers lists it.
func (s *Service) pluginProvider(p *plugin) Provider {
	pr := Provider{ID: p.id, Label: p.id, Name: p.id, Agent: p.id, Account: p.id, SignIn: "elsewhere", Plugin: s.pluginInfo(p)}
	if p.m != nil {
		pr.Label, pr.Name, pr.Agent, pr.Account = p.m.Name, p.m.Name, p.m.Agent, p.m.Account
		if pr.Account == "" {
			pr.Account = p.m.Name
		}
	}
	if c, err := s.cli(p.id); err == nil {
		pr.Capabilities = c.adapter.Capabilities()
	}
	return pr
}

var _ core.Adapter = (*manifest.Adapter)(nil)
