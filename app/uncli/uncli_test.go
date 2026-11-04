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
	"github.com/harvey-withington/uncli/app/internal/session"
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
	_, err := Start(Options{DataDir: t.TempDir()})
	if !errors.Is(err, ErrServiceAccount) {
		t.Fatalf("started as LocalSystem: %v", err)
	}
	// The host can name the account in its own message.
	var sa *ServiceAccountError
	if !errors.As(err, &sa) || sa.Account != "LocalSystem" {
		t.Errorf("the error doesn't name the account: %v", err)
	}
	start(t, Options{AllowServiceAccount: true})
}

// The host's policy is asked only about the servers it gave the session,
// and only its three decisions count.
func TestHostToolPolicyOnlyForTheHostsServers(t *testing.T) {
	var seen HostToolCall
	o := Options{
		MCPServers: func(string) map[string]MCPServer {
			return map[string]MCPServer{"board": {URL: "http://127.0.0.1:1/mcp"}}
		},
		HostToolPolicy: func(c HostToolCall) HostDecision {
			seen = c
			switch c.Tool {
			case "odd":
				return HostDecision{Decision: "maybe"}
			case "update_checklist":
				return HostDecision{Decision: PolicyAsk, Reason: "Suggest mode", Prompt: &HostPrompt{Title: "Update checklist 'Preparation to Leave'", Detail: "Adds: Passport"}}
			}
			return HostDecision{Decision: PolicyDeny, Reason: "no", Prompt: &HostPrompt{Title: "ignored: only asks show a prompt"}}
		},
	}
	h := &hostTools{o: o, servers: map[string]map[string]bool{}}
	h.mcp("s1")
	call := func(sess, server, tool, input string) session.HostDecision {
		return h.decide(session.HostToolCall{SessionID: sess, Server: server, Tool: tool, Input: json.RawMessage(input)})
	}
	if d := call("s1", "board", "update_card", `{"id":"42","title":"Melbourne"}`); d.Decision != PolicyDeny || d.Reason != "no" || d.Prompt != nil {
		t.Errorf("the host's server: %+v", d)
	}
	// The host sees the call's arguments as a map.
	if seen.Session != "s1" || seen.Server != "board" || seen.Args["id"] != "42" || seen.Args["title"] != "Melbourne" {
		t.Errorf("the host saw %+v", seen)
	}
	d := call("s1", "board", "update_checklist", `{}`)
	if d.Decision != PolicyAsk || d.Prompt == nil || d.Prompt.Title != "Update checklist 'Preparation to Leave'" || d.Prompt.Detail != "Adds: Passport" {
		t.Errorf("an ask with a prompt: %+v", d)
	}
	if d := call("s1", "github", "create_issue", `{}`); d.Decision != "" {
		t.Errorf("the user's own server went to the host: %+v", d)
	}
	if d := call("s2", "board", "update_card", `{}`); d.Decision != "" {
		t.Errorf("a session the host gave no servers: %+v", d)
	}
	if d := call("s1", "board", "odd", `{}`); d.Decision != "" {
		t.Errorf("an unknown decision counted: %+v", d)
	}
}

// A session made with the host's meta carries it from the start, so the
// host's MCPServers knows what the session is for on its very first launch.
func TestSessionMetaFromTheStart(t *testing.T) {
	c := start(t, Options{})
	srv := httptest.NewServer(c.Handler())
	defer srv.Close()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "CreateSessionWith",
		"params": []any{"chat", "", "", "", "", nil, map[string]any{"meta": "card:42"}}})
	resp, err := http.Post(srv.URL+"/rpc", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r struct {
		Result struct {
			Session struct{ ID string } `json:"session"`
		}
		Error *struct{ Message string }
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.Error != nil {
		t.Fatalf("CreateSessionWith: %v %+v", err, r.Error)
	}
	if got := c.SessionMeta(r.Result.Session.ID); got != "card:42" {
		t.Errorf("meta = %q", got)
	}
	if got := c.SessionMeta("no-such-session"); got != "" {
		t.Errorf("an unknown session's meta = %q", got)
	}
}

// The host's MCP servers become the CLI's --mcp-config: HTTP servers, with
// the host's headers; none when the host gives none.
func TestHostMCPConfig(t *testing.T) {
	o := Options{MCPServers: func(id string) map[string]MCPServer {
		if id != "s1" {
			return nil
		}
		return map[string]MCPServer{"board": {URL: "http://127.0.0.1:9870/repos/r1/mcp", Headers: map[string]string{"Authorization": "Bearer t"}}}
	}}
	got := o.hostMCP("s1")
	if len(got) != 1 {
		t.Fatalf("config = %q", got)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(got[0]), &cfg); err != nil {
		t.Fatal(err)
	}
	b := cfg.MCPServers["board"]
	if b.Type != "http" || b.URL != "http://127.0.0.1:9870/repos/r1/mcp" || b.Headers["Authorization"] != "Bearer t" {
		t.Errorf("board = %+v", b)
	}
	if o.hostMCP("s2") != nil {
		t.Error("a session the host gives no servers got some")
	}
	// A server without headers has no headers key: the CLI drops a server
	// whose headers are null.
	bare := mcpConfig(map[string]MCPServer{"board": {URL: "http://127.0.0.1:1/mcp"}})
	if len(bare) != 1 || strings.Contains(bare[0], "headers") {
		t.Errorf("config = %q", bare)
	}
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
