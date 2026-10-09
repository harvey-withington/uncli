package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/internal/adapter/manifest"
	"uncli/internal/session"
)

const agyPlugin = "../../testdata/providers/antigravity/" + manifest.File

// pluginService starts UNCLI with plugin folders: name → provider.yaml text.
func pluginService(t *testing.T, config string, plugins map[string]string) *Service {
	t.Helper()
	for name, text := range plugins {
		dir := filepath.Join(config, "providers", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, manifest.File), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := New(Paths{Config: config, Cache: filepath.Join(config, "cache"), Scratch: filepath.Join(config, "scratch")}, &recEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	return svc
}

func providerNamed(svc *Service, id string) (Provider, bool) {
	for _, p := range svc.Providers() {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

func agyText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(agyPlugin)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPluginStartsDisabled(t *testing.T) {
	config := t.TempDir()
	svc := pluginService(t, config, map[string]string{"antigravity-manifest": agyText(t)})
	ctx := context.Background()

	p, ok := providerNamed(svc, "antigravity-manifest")
	if !ok || p.Plugin == nil || p.Name != "Antigravity CLI" || p.Plugin.Error != "" {
		t.Fatalf("provider = %+v", p)
	}
	info := p.Plugin
	if info.Enabled || !info.Bypasses || !info.Hooked || len(info.Sources) == 0 || info.Command[0] != "agy" {
		t.Errorf("plugin = %+v", info)
	}
	if !p.Capabilities.HookApprovals {
		t.Errorf("capabilities = %+v", p.Capabilities)
	}
	// The built-ins are still there, first.
	if ps := svc.Providers(); ps[0].ID != "claude" || ps[1].ID != "antigravity" {
		t.Errorf("providers = %v, %v", ps[0].ID, ps[1].ID)
	}

	// Disabled: nothing is run or downloaded, and no session starts on it.
	if st := svc.ProviderStatus(ctx, "antigravity-manifest", true); st.LoggedIn || !strings.Contains(st.Error, "isn't enabled") {
		t.Errorf("status = %+v", st)
	}
	if _, err := svc.InstallProvider(ctx, "antigravity-manifest"); err == nil {
		t.Error("a disabled plugin's CLI was downloaded")
	}
	if _, err := svc.ProviderChannels(ctx, "antigravity-manifest"); err == nil {
		t.Error("a disabled plugin's release manifest was fetched")
	}
	if _, _, err := svc.CreateSessionWith(session.NewSession{Profile: "chat", Provider: "antigravity-manifest"}); err == nil {
		t.Error("a session was created on a disabled plugin")
	}

	// Enabled as shown, it can be used.
	if err := svc.EnablePlugin("antigravity-manifest", "not-the-hash"); err == nil {
		t.Error("enabled with the wrong hash")
	}
	if err := svc.EnablePlugin("antigravity-manifest", info.Hash); err != nil {
		t.Fatal(err)
	}
	v, _, err := svc.CreateSessionWith(session.NewSession{Profile: "chat", Provider: "antigravity-manifest"})
	if err != nil || v.Adapter != "antigravity-manifest" {
		t.Fatalf("session = %+v, %v", v, err)
	}

	// Changed on disk: not enabled until looked at again.
	dir := filepath.Join(config, "providers", "antigravity-manifest", manifest.File)
	if err := os.WriteFile(dir, []byte(agyText(t)+"\n# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnablePlugin("antigravity-manifest", info.Hash); err == nil {
		t.Error("enabled a manifest that changed after UNCLI started")
	}
	svc.Close()
	svc = pluginService(t, config, nil)
	p, _ = providerNamed(svc, "antigravity-manifest")
	if p.Plugin.Enabled || !p.Plugin.Changed {
		t.Errorf("after a change: %+v", p.Plugin)
	}
	if err := svc.pluginAllowed("antigravity-manifest"); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Errorf("allowed after a change: %v", err)
	}
	// Its session stays, and can't start until it's enabled again.
	if _, err := svc.Sessions.View(v.ID); err != nil {
		t.Error(err)
	}
	if err := svc.Sessions.Send(ctx, v.ID, "hi"); err == nil {
		t.Error("a changed plugin's session started")
	}

	// Disabled again.
	if err := svc.EnablePlugin("antigravity-manifest", p.Plugin.Hash); err != nil {
		t.Fatal(err)
	}
	if err := svc.DisablePlugin("antigravity-manifest"); err != nil {
		t.Fatal(err)
	}
	if err := svc.pluginAllowed("antigravity-manifest"); err == nil {
		t.Error("still allowed after disabling")
	}
}

// A plugin that can't be used is listed with why, and harms nothing else.
func TestBrokenPlugins(t *testing.T) {
	agy := agyText(t)
	svc := pluginService(t, t.TempDir(), map[string]string{
		"broken":     "id: broken\nname: [",
		"claude":     strings.Replace(agy, "id: antigravity-manifest", "id: claude", 1),
		"other-name": agy, // its id isn't its folder's
	})
	_ = os.MkdirAll(filepath.Join(svc.pluginsDir(), "no-manifest"), 0o755)
	svc.plugins = svc.loadPlugins([]string{"claude", "antigravity"})
	want := map[string]string{"broken": "provider.yaml", "other-name": "folder"}
	for id, msg := range want {
		p, ok := providerNamed(svc, id)
		if !ok || p.Plugin == nil || !strings.Contains(p.Plugin.Error, msg) {
			t.Errorf("%s: %+v", id, p.Plugin)
		}
	}
	// The built-in keeps its id; the plugin claiming it is listed as broken.
	n := 0
	for _, p := range svc.Providers() {
		if p.ID == "claude" {
			n++
			if p.Plugin != nil && !strings.Contains(p.Plugin.Error, "built-in") {
				t.Errorf("claude plugin: %+v", p.Plugin)
			}
		}
	}
	if n != 2 || svc.Providers()[0].Plugin != nil {
		t.Errorf("providers = %+v", svc.Providers())
	}
	if _, ok := providerNamed(svc, "no-manifest"); ok {
		t.Error("a folder without a manifest was listed")
	}
	// The built-ins work as before: the plugin claiming Claude Code's id
	// gates nothing.
	if _, _, err := svc.CreateSessionWith(session.NewSession{Profile: "chat"}); err != nil {
		t.Error(err)
	}
	if err := svc.pluginAllowed("claude"); err != nil {
		t.Errorf("the built-in was gated: %v", err)
	}
	if _, _, err := svc.CreateSessionWith(session.NewSession{Profile: "chat", Provider: "claude"}); err != nil {
		t.Error(err)
	}
}
