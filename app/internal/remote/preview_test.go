package remote

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"uncli/internal/app"
	"uncli/internal/notify"
	"uncli/internal/push"
)

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
	if cc := get("/m/sw.js").Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("service worker caching = %q", cc)
	}
	if cc := get("/m/assets/app-1a2b.js").Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("asset caching = %q", cc)
	}
	resp, err := (&http.Client{Transport: r.https.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(r.base() + "/")
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
