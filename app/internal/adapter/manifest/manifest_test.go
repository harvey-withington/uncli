package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
)

func agyText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(agyManifest, File))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// What a manifest may not do, each refused when it's read.
func TestManifestChecks(t *testing.T) {
	src := agyText(t)
	if _, err := Parse([]byte(src)); err != nil {
		t.Fatalf("the Antigravity manifest: %v", err)
	}
	// The hook section, cut out whole.
	i := strings.Index(src, "\napprovals:")
	noHook := src[:i]
	cases := map[string]struct{ src, want string }{
		"checks off without a hook":   {noHook, "launch.approvals switches the CLI's own checks off"},
		"an http download":            {strings.Replace(src, "url: https://storage", "url: http://storage", 1), "must be https"},
		"no checksum":                 {strings.Replace(src, "sha512: f346", "sha513: f346", 1), "field sha513 not found"},
		"a file outside its folder":   {strings.Replace(src, `"{state}/config/hooks.json"`, `"{state}/../../evil.json"`, 1), "only files in {state}"},
		"a tool hook that never runs": {strings.Replace(src, `"{hook:pre-tool}"`, `"echo allow"`, 1), "never run the pre-tool hook"},
		"a kind UNCLI doesn't have":   {strings.Replace(src, "run_command: shell", "run_command: anything", 1), `"anything" isn't a kind`},
		"an id that isn't one":        {strings.Replace(src, "id: antigravity-manifest", "id: Anti Gravity", 1), "lower-case letters"},
		"an unknown field":            {strings.Replace(src, "signin: elsewhere", "signin: elsewhere\nrunAsAdmin: true", 1), "field runAsAdmin not found"},
		"no way to tell sign-in":      {strings.Replace(src, "models:\n  line:", "notmodels:\n  line:", 1), "field notmodels not found"},
	}
	for name, c := range cases {
		_, err := Parse([]byte(c.src))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Without the hook and without switching the checks off, it's fine: the
	// CLI keeps its own checks.
	plain := strings.Replace(noHook, "  approvals: [\"--dangerously-skip-permissions\"]\n", "", 1)
	m, err := Parse([]byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	a := New(m, nil, t.TempDir(), "x")
	if c := a.Capabilities(); c.Approvals || c.HookApprovals {
		t.Errorf("capabilities = %+v", c)
	}
	cmd, _ := a.BuildCommand("cli", core.LaunchSpec{Approvals: true})
	if slices.Contains(cmd.Args, "--dangerously-skip-permissions") {
		t.Errorf("args = %q", cmd.Args)
	}
}

// A plugin the user hasn't enabled (as it is now) never starts its CLI.
func TestNotAllowedNeverStarts(t *testing.T) {
	_, man := pair(t)
	man.Allowed = func() error { return os.ErrPermission }
	if _, err := man.BuildCommand("agy.exe", core.LaunchSpec{Approvals: true}); err == nil {
		t.Fatal("started while not allowed")
	}
	if _, err := os.Stat(filepath.Join(man.StateDir, "config", "hooks.json")); err == nil {
		t.Error("wrote its hooks while not allowed")
	}
}

func TestLoadHashes(t *testing.T) {
	dir := t.TempDir()
	src := agyText(t)
	_ = os.WriteFile(filepath.Join(dir, File), []byte(src), 0o644)
	_, h1, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, File), []byte(src+"\n# changed\n"), 0o644)
	_, h2, _ := Load(dir)
	if h1 == "" || h1 == h2 {
		t.Errorf("hashes %s / %s", h1, h2)
	}
}

func TestInstaller(t *testing.T) {
	bin := []byte("a cli")
	sum := sha256.Sum256(bin)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/linux_x.json":
			_ = json.NewEncoder(w).Encode(map[string]string{"v": "2.0.0", "u": "https://" + r.Host + "/bin", "s": hex.EncodeToString(sum[:])})
		case "/bin":
			_, _ = w.Write(bin)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := &Manifest{Name: "Test CLI", Install: Install{Pinned: "1.0.0", Binary: "test",
		Builds: map[string]map[string]Build{
			"1.0.0": {"linux/amd64": {URL: srv.URL + "/bin", SHA256: hex.EncodeToString(sum[:])}},
			"1.0.1": {"linux/amd64": {URL: srv.URL + "/bin", SHA256: strings.Repeat("0", 64)}},
		},
		Latest: &Latest{URL: srv.URL + "/latest/{platform}.json", Platforms: map[string]string{"linux/amd64": "linux_x"}, Version: "v", Binary: "u", SHA256: "s"},
	}}
	i := NewInstaller(m, t.TempDir())
	i.GOOS, i.GOARCH, i.Client = "linux", "amd64", srv.Client()

	p, err := i.Ensure(context.Background(), "1.0.0", nil)
	if err != nil || filepath.Base(p) != "test" {
		t.Fatalf("pinned: %s, %v", p, err)
	}
	if _, err := i.Ensure(context.Background(), "1.0.1", nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("a bad checksum: %v", err)
	}
	if ch, err := i.Channels(context.Background()); err != nil || ch["latest"] != "2.0.0" {
		t.Errorf("channels = %v, %v", ch, err)
	}
	if _, err := i.Ensure(context.Background(), "2.0.0", nil); err != nil {
		t.Errorf("latest: %v", err)
	}
	if _, err := i.Ensure(context.Background(), "3.0.0", nil); err == nil {
		t.Error("a version from nowhere was downloaded")
	}
	if v, _ := i.Installed(); !slices.Equal(v, []string{"1.0.0", "2.0.0"}) {
		t.Errorf("installed = %v", v)
	}
	i.GOOS = "windows"
	if p, _ := i.Path("1.0.0"); filepath.Base(p) != "test.exe" {
		t.Errorf("windows binary = %s", p)
	}
}
