package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/notify"
	"github.com/harvey-withington/uncli/app/internal/push"
)

// The app installed from each PC is named for it, from the name the phone
// reached it by; an address or localhost keeps the plain name.
func TestPWANamedForMachine(t *testing.T) {
	for host, want := range map[string]string{
		"deviant.tail1234.ts.net:9871": "deviant",
		"deviant.tail1234.ts.net":      "deviant",
		"100.101.102.103:9871":         "",
		"[fd7a:115c::1]:9871":          "",
		"127.0.0.1:9871":               "",
		"localhost:9871":               "",
	} {
		if got := machineName(host); got != want {
			t.Errorf("machineName(%q) = %q, want %q", host, got, want)
		}
	}
	named, err := namedManifest([]byte(`{"name":"UNCLI","short_name":"UNCLI","scope":"/m/"}`), "deviant")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(named, &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "UNCLI · deviant" || m["short_name"] != "deviant" || m["scope"] != "/m/" {
		t.Errorf("named manifest = %v", m)
	}
	plain := []byte(`{"name":"UNCLI"}`)
	if same, _ := namedManifest(plain, ""); string(same) != string(plain) {
		t.Errorf("unnamed manifest changed: %s", same)
	}
}

// The phone app's files are served with the right types, caching and
// content security policy.
func TestPWAServing(t *testing.T) {
	files := fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>UNCLI</title>")},
		"sw.js":                {Data: []byte("self.addEventListener('push', () => {})")},
		"manifest.webmanifest": {Data: []byte(`{"name":"UNCLI"}`)},
		"assets/app-1a2b.js":   {Data: []byte("console.log(1)")},
	}
	r := newRigWith(t, Config{PWA: files})
	get := func(p string) *http.Response {
		resp, err := r.https.Get(r.base() + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	page := get("/m/")
	if page.StatusCode != 200 || !strings.Contains(page.Header.Get("Content-Security-Policy"), "connect-src 'self'") {
		t.Errorf("page = %d, CSP %q", page.StatusCode, page.Header.Get("Content-Security-Policy"))
	}
	if ct := get("/m/manifest.webmanifest").Header.Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("manifest type = %q", ct)
	}
	resp, err := r.https.Get(r.base() + "/m/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	resp.Body.Close()
	if err != nil || manifest["name"] != "UNCLI · test" || manifest["short_name"] != "test" {
		t.Errorf("served manifest = %v (%v), want it named for the machine", manifest, err)
	}
	if cc := get("/m/sw.js").Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("service worker caching = %q", cc)
	}
	if cc := get("/m/assets/app-1a2b.js").Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("asset caching = %q", cc)
	}
	if code := get("/m/assets/").StatusCode; code != http.StatusNotFound {
		t.Errorf("folder listing = %d", code)
	}
	resp, err = (&http.Client{Transport: r.https.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(r.base() + "/")
	if err != nil || resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/m/" {
		t.Errorf("root = %v, %v", resp, err)
	}
}

// TestPWAPreview serves the built phone app (frontend/dist/m) on loopback
// with a real app service and the fake CLI, allows every pairing request,
// and keeps running for UNCLI_PWA_PREVIEW minutes, so headless Edge can
// drive it at a phone's size. It writes the loopback address and a
// pairing code to UNCLI_PWA_PREVIEW_OUT. Never part of a normal run.
func TestPWAPreview(t *testing.T) {
	mins, _ := strconv.Atoi(os.Getenv("UNCLI_PWA_PREVIEW"))
	if mins <= 0 {
		t.Skip("set UNCLI_PWA_PREVIEW=<minutes> to serve the phone app for a preview")
	}
	exe, _ := os.Executable()
	t.Setenv("UNCLI_FAKE_CLI", "1")
	t.Setenv("UNCLI_CLAUDE_BIN", exe)
	dist, err := filepath.Abs(filepath.Join("..", "..", "frontend", "dist", "m"))
	if err != nil {
		t.Fatal(err)
	}
	var r *rig
	r = newRigWith(t, Config{PWA: os.DirFS(dist), OnPairRequest: func(p PairRequest) {
		time.Sleep(1500 * time.Millisecond) // long enough to see "Waiting for Allow"
		_ = r.srv.AnswerPairing(p.ID, true)
	}})
	// Push as the app has it: real push services only.
	if p, err := push.New(r.svc.Store); err == nil {
		r.srv.push = p
		r.svc.SetRemoteNotes(func(n notify.Note, approval bool) {
			go func() {
				r.srv.Notify(n, approval)
				t.Logf("pushed %q", n.Body)
			}()
		})
	}
	loop := fmt.Sprintf("http://localhost:%d", r.port)
	p, _ := r.srv.pair.start(loop)
	// A session to look at, waiting on an approval, made through the API.
	api := app.NewAPI(r.svc, nil)
	created, err := api.CreateSession("code", t.TempDir(), "haiku", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = api.Rename(created.Session.ID, "Push the release branch")
	if err := api.Send(created.Session.ID, "Push the release branch to origin", nil); err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("UNCLI_PWA_PREVIEW_OUT")
	if out != "" {
		_ = os.WriteFile(out, []byte(loop+"\n"+p.Code+"\n"), 0o644)
	}
	t.Logf("phone app at %s/m/#pair=%s", loop, p.Code)
	time.Sleep(time.Duration(mins) * time.Minute)
}
