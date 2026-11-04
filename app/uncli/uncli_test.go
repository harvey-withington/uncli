package uncli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	if got, want := os.Getenv("CLAUDE_CONFIG_DIR"), filepath.Join(dir, "cli-config", "claude"); got != want {
		t.Errorf("the CLI's config is at %q, want %q", got, want)
	}
}

func TestTheUsersCLIConfigOnlyWhenAsked(t *testing.T) {
	start(t, Options{UserCLIConfig: true})
	if got := os.Getenv("CLAUDE_CONFIG_DIR"); got != "" {
		t.Errorf("the CLI's config moved to %q", got)
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
