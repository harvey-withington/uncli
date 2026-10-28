package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"0.4.0-alpha", "0.3.0-alpha", 1},
		{"0.3.0-alpha", "0.3.0-alpha", 0},
		{"0.3.0", "0.3.0-alpha", 1},        // a release beats its pre-releases
		{"0.3.0-beta", "0.3.0-alpha", 1},   // words compare as text
		{"0.3.0-rc.10", "0.3.0-rc.2", 1},   // numbers as numbers
		{"0.3.0-rc.1", "0.3.0-rc", 1},      // more parts win when the rest match
		{"0.10.0-alpha", "0.9.0-alpha", 1}, // not as text
		{"1.0.0-alpha", "0.99.0", 1},
		// A test build is the release it was built after: offered the next
		// one, never the same or an older one.
		{"0.3.0-alpha+dev.66", "0.3.0-alpha", 0},
		{"0.4.0-alpha", "0.3.0-alpha+dev.66", 1},
		{"0.3.0-alpha+dev.66", "0.2.0-alpha", 1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Compare(c.b, c.a); got != -c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.b, c.a, got, -c.want)
		}
	}
	for v, ok := range map[string]bool{"0.3.0-alpha": true, "1.2.3": true, "0.3.0-alpha+dev.66": true, "dev": false, "1.2": false, "1.x.3": false, "": false} {
		if Valid(v) != ok {
			t.Errorf("Valid(%q) = %v", v, !ok)
		}
	}
}

// A fake GitHub: releases, each with an installer and SHA256SUMS.txt
// served from the same test server.
func fakeGitHub(t *testing.T, setup []byte, releases func(base string) []ghRelease) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(setup)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases":
			_ = json.NewEncoder(w).Encode(releases(srv.URL))
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
			v := strings.Split(r.URL.Path, "/")[2]
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), SetupName(v))
		case strings.HasSuffix(r.URL.Path, "-setup.exe"):
			_, _ = w.Write(setup)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func rel(base, v string, pre, draft, assets bool) ghRelease {
	r := ghRelease{Tag: "v" + v, Prerelease: pre, Draft: draft, URL: base + "/page/" + v}
	if assets {
		r.Assets = []ghAsset{
			{Name: SetupName(v), URL: base + "/dl/" + v + "/" + SetupName(v), Size: 9},
			{Name: "SHA256SUMS.txt", URL: base + "/dl/" + v + "/SHA256SUMS.txt"},
		}
	}
	return r
}

func TestLatestPicksTheNewestInstallableRelease(t *testing.T) {
	srv := fakeGitHub(t, []byte("installer"), func(base string) []ghRelease {
		return []ghRelease{
			rel(base, "0.5.0-alpha", true, true, true),   // a draft
			rel(base, "0.4.1-alpha", true, false, false), // no installer yet
			rel(base, "0.3.0-alpha", true, false, true),
			rel(base, "0.4.0-alpha", true, false, true), // out of order
			{Tag: "nightly"},
		}
	})
	r, err := Latest(context.Background(), srv.Client(), srv.URL+"/releases")
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "0.4.0-alpha" || r.Setup != "uncli-0.4.0-alpha-windows-x64-setup.exe" || r.URL != srv.URL+"/page/0.4.0-alpha" {
		t.Errorf("latest = %+v", r)
	}
}

func TestDownloadChecksTheInstaller(t *testing.T) {
	setup := []byte("the real installer")
	srv := fakeGitHub(t, setup, func(base string) []ghRelease { return []ghRelease{rel(base, "0.4.0-alpha", true, false, true)} })
	r, err := Latest(context.Background(), srv.Client(), srv.URL+"/releases")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var seen []int
	path, err := Download(context.Background(), srv.Client(), r, dir, func(p int) { seen = append(seen, p) })
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(setup) || filepath.Base(path) != r.Setup {
		t.Errorf("downloaded %s: %q", path, got)
	}
	if len(seen) == 0 || seen[len(seen)-1] != 100 {
		t.Errorf("progress = %v", seen)
	}

	// A file that doesn't match its checksum isn't kept.
	bad := fakeGitHub(t, []byte("tampered"), func(base string) []ghRelease { return []ghRelease{rel(base, "0.4.0-alpha", true, false, true)} })
	r2, _ := Latest(context.Background(), bad.Client(), bad.URL+"/releases")
	r2.sumsURL = srv.URL + "/dl/0.4.0-alpha/SHA256SUMS.txt" // the real file's sum
	dir2 := t.TempDir()
	if _, err := Download(context.Background(), bad.Client(), r2, dir2, nil); err == nil || !strings.Contains(err.Error(), "doesn't match its SHA-256") {
		t.Fatalf("tampered: %v", err)
	}
	if left, _ := os.ReadDir(dir2); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
}

func TestSumFor(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	sums := sum + "  other.exe\r\n" + strings.ToUpper(sum) + " *uncli-1.0.0-windows-x64-setup.exe\n"
	if got, ok := sumFor(sums, "uncli-1.0.0-windows-x64-setup.exe"); !ok || got != sum {
		t.Errorf("sum = %q, %v", got, ok)
	}
	if _, ok := sumFor(sums, "missing.exe"); ok {
		t.Error("found a missing file")
	}
}
