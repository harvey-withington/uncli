package uncli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harvey-withington/uncli/app/internal/datadir"
)

const hostOrigin = "https://bruv.example"

// start runs a host's UNCLI on a temp folder. The CLIs' config variable is
// put back afterwards: Start sets it for the whole process.
func start(t *testing.T, o Options) *Core {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if o.DataDir == "" {
		o.DataDir = t.TempDir()
	}
	c, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

type reply struct {
	Status int
	Result json.RawMessage
	Error  *struct{ Message string }
}

// rpc calls a method as a page from origin would, with headers as given.
func rpc(t *testing.T, url, origin string, headers map[string]string, method string) reply {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": []any{}})
	req, _ := http.NewRequest("POST", url+"/rpc", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := reply{Status: resp.StatusCode}
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestStartNeedsAFolder(t *testing.T) {
	if _, err := Start(Options{}); err == nil {
		t.Fatal("started without a data folder")
	}
}

func TestOneUNCLIOnAFolderAtATime(t *testing.T) {
	dir := t.TempDir()
	c := start(t, Options{DataDir: dir, Name: "BRUV"})
	if _, err := Start(Options{DataDir: dir, Name: "BRUV"}); !errors.Is(err, datadir.ErrInUse) {
		t.Fatalf("a second UNCLI started on the folder: %v", err)
	}
	c.Close()
	again, err := Start(Options{DataDir: dir, Name: "BRUV"})
	if err != nil {
		t.Fatalf("the folder wasn't freed: %v", err)
	}
	again.Close()
	// Another app can't take it over.
	if _, err := Start(Options{DataDir: dir, Name: "Another app"}); !errors.Is(err, datadir.ErrNotYours) {
		t.Fatalf("another app started on BRUV's folder: %v", err)
	}
}

func TestNeverTheDesktopAppsFolders(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	t.Setenv("AppData", filepath.Join(home, "Roaming"))
	t.Setenv("LocalAppData", filepath.Join(home, "Local"))
	t.Setenv("UNCLI_DATA_DIR", "")
	for _, dir := range []string{
		filepath.Join(home, "Roaming", "uncli"),
		filepath.Join(home, "Roaming", "uncli", "bruv"),
		filepath.Join(home, "Local", "uncli"),
	} {
		if c, err := Start(Options{DataDir: dir, Name: "BRUV"}); !errors.Is(err, datadir.ErrNotYours) {
			if c != nil {
				c.Close()
			}
			t.Errorf("%s: %v", dir, err)
		}
	}
	// A folder of the host's own beside them is fine.
	c, err := Start(Options{DataDir: filepath.Join(home, "Roaming", "bruv", "uncli"), Name: "BRUV"})
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestEverythingStaysInTheHostsFolder(t *testing.T) {
	dir := t.TempDir()
	start(t, Options{DataDir: dir})
	if _, err := os.Stat(filepath.Join(dir, "uncli.db")); err != nil {
		t.Errorf("no database in the host's folder: %v", err)
	}
}

// The CLIs UNCLI starts keep their config in the host's folder; the host's
// own environment is left alone, so programs it starts keep the user's.
func TestTheCLIsConfigIsTheHostsWithoutTouchingItsEnvironment(t *testing.T) {
	dir := t.TempDir()
	c := start(t, Options{DataDir: dir})
	want := filepath.Join(dir, "cli-config", "claude")
	if got, _ := c.svc.Adapter.ConfigDir(); got != want {
		t.Errorf("UNCLI reads the CLI's config at %q, want %q", got, want)
	}
	if got := c.svc.Adapter.AuthStatusCommand("claude").Env["CLAUDE_CONFIG_DIR"]; got != want {
		t.Errorf("the CLI is started with its config at %q, want %q", got, want)
	}
	if got := os.Getenv("CLAUDE_CONFIG_DIR"); got != "" {
		t.Errorf("the host's environment changed: CLAUDE_CONFIG_DIR=%q", got)
	}
}

func TestTheUsersCLIConfigOnlyWhenAsked(t *testing.T) {
	c := start(t, Options{UserCLIConfig: true})
	if got := c.svc.Adapter.ConfigDirSet(); got != "" {
		t.Errorf("the CLI's config moved to %q", got)
	}
	if _, ok := c.svc.Adapter.AuthStatusCommand("claude").Env["CLAUDE_CONFIG_DIR"]; ok {
		t.Error("the CLI is told a config folder of UNCLI's")
	}
}

func TestTheMountServesTheHostsPagesWithoutAKey(t *testing.T) {
	c := start(t, Options{Origins: []string{hostOrigin}})
	mux := http.NewServeMux()
	mux.Handle("/uncli/", http.StripPrefix("/uncli", c.Handler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	r := rpc(t, srv.URL+"/uncli", hostOrigin, nil, "Containers")
	if r.Status != http.StatusOK || r.Error != nil {
		t.Fatalf("the host's page was refused: %+v", r)
	}
	// Containers belong to the whole Windows user: a host's UNCLI has none.
	var info struct{ Containers []any }
	_ = json.Unmarshal(r.Result, &info)
	if len(info.Containers) != 0 {
		t.Errorf("containers offered: %s", r.Result)
	}
	// A page on another site is refused before anything runs.
	if r := rpc(t, srv.URL+"/uncli", "https://evil.example", nil, "Containers"); r.Status != http.StatusForbidden {
		t.Errorf("another site's page got %d", r.Status)
	}
	// The host's server itself (no Origin) gets through.
	if r := rpc(t, srv.URL+"/uncli", "", nil, "Containers"); r.Status != http.StatusOK {
		t.Errorf("a request without an Origin got %d", r.Status)
	}
}

// A host's page may come from another origin than its server (a desktop
// app's webview): its origins get CORS, with its own sign-in in a header.
func TestAHostsPageOnAnotherOrigin(t *testing.T) {
	const webview = "http://wails.localhost:34115"
	c := start(t, Options{Origins: []string{webview}})
	srv := httptest.NewServer(c.Handler())
	defer srv.Close()
	preflight := func(origin string) *http.Response {
		req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/rpc", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	resp := preflight(webview)
	if resp.StatusCode != http.StatusNoContent || resp.Header.Get("Access-Control-Allow-Origin") != webview ||
		!strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("the host's webview was refused its preflight: %d %v", resp.StatusCode, resp.Header)
	}
	if resp := preflight("https://evil.example"); resp.StatusCode != http.StatusForbidden || resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("another site got a preflight: %d %v", resp.StatusCode, resp.Header)
	}
	// The call itself: readable by the webview, with the host's own sign-in
	// header passing through (Handler swaps in UNCLI's key after it).
	r := rpc(t, srv.URL, webview, map[string]string{"Authorization": "Bearer the-hosts-own-token"}, "Containers")
	if r.Status != http.StatusOK || r.Error != nil {
		t.Fatalf("the webview's call: %+v", r)
	}
}

// The CLIs would run tools as the host's account: never a service account
// (LocalSystem can do anything) unless the host says so.
func TestNotAsAServiceAccount(t *testing.T) {
	if a := serviceAccount(); a != "" {
		t.Fatalf("the tests run as %s", a)
	}
	runningAs = func() string { return "LocalSystem" }
	t.Cleanup(func() { runningAs = serviceAccount })
	if _, err := Start(Options{DataDir: t.TempDir()}); !errors.Is(err, errServiceAccount) {
		t.Fatalf("started as LocalSystem: %v", err)
	}
	start(t, Options{AllowServiceAccount: true})
}

// A server reached by several names checks origins itself.
func TestAllowOriginAdmitsWhatTheListCant(t *testing.T) {
	c := start(t, Options{AllowOrigin: func(origin string, r *http.Request) bool {
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host // the page's own server, by whatever name
	}})
	srv := httptest.NewServer(c.Handler())
	defer srv.Close()
	if r := rpc(t, srv.URL, srv.URL, nil, "Containers"); r.Status != http.StatusOK {
		t.Errorf("the same host was refused: %d", r.Status)
	}
	if r := rpc(t, srv.URL, "https://evil.example", nil, "Containers"); r.Status != http.StatusForbidden {
		t.Errorf("another host got %d", r.Status)
	}
}

func TestTheKeyedRoutesNeedTheKey(t *testing.T) {
	c := start(t, Options{Origins: []string{hostOrigin}, Key: "secret"})
	srv := httptest.NewServer(c.KeyedHandler())
	defer srv.Close()
	if r := rpc(t, srv.URL, "", nil, "Containers"); r.Status != http.StatusUnauthorized {
		t.Errorf("no key got %d", r.Status)
	}
	if r := rpc(t, srv.URL, "", map[string]string{"Authorization": "Bearer wrong"}, "Containers"); r.Status != http.StatusUnauthorized {
		t.Errorf("a wrong key got %d", r.Status)
	}
	if r := rpc(t, srv.URL, "", map[string]string{"Authorization": "Bearer secret"}, "Containers"); r.Status != http.StatusOK {
		t.Errorf("the key got %d", r.Status)
	}
}

func TestWhatTheHostDoesItselfIsntOffered(t *testing.T) {
	c := start(t, Options{})
	srv := httptest.NewServer(c.Handler())
	defer srv.Close()
	for _, m := range []string{"InstallUpdate", "CheckForUpdates", "PushSubscribe", "TestPush", "SetPreferences", "BuildContainer"} {
		r := rpc(t, srv.URL, "", nil, m)
		if r.Error == nil || !strings.Contains(r.Error.Message, "doesn't offer") {
			t.Errorf("%s: %+v", m, r)
		}
	}
}
