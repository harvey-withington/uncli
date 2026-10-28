package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"uncli/internal/buildinfo"
)

// releaseServer is a fake GitHub with one release, 0.4.0-alpha.
func releaseServer(t *testing.T, setup []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(setup)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			fmt.Fprintf(w, `[{"tag_name":"v0.4.0-alpha","prerelease":true,"html_url":"%[1]s/page","assets":[
				{"name":"uncli-0.4.0-alpha-windows-x64-setup.exe","browser_download_url":"%[1]s/setup.exe","size":%[2]d},
				{"name":"SHA256SUMS.txt","browser_download_url":"%[1]s/sums"}]}]`, srv.URL, len(setup))
		case "/sums":
			fmt.Fprintf(w, "%s  uncli-0.4.0-alpha-windows-x64-setup.exe\n", hex.EncodeToString(sum[:]))
		case "/setup.exe":
			_, _ = w.Write(setup)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func asRelease(t *testing.T, v string) {
	t.Helper()
	prev := buildinfo.Version
	buildinfo.Version = v
	t.Cleanup(func() { buildinfo.Version = prev })
}

func TestUpdatesFindAndInstallANewerRelease(t *testing.T) {
	asRelease(t, "0.3.0-alpha")
	t.Setenv("LOCALAPPDATA", t.TempDir()) // where the download goes
	s := newTestService(t)
	s.initUpdates()
	srv := releaseServer(t, []byte("installer bytes"))
	s.updates.url, s.updates.client = srv.URL+"/releases", srv.Client()
	var handed string
	s.updates.install = func(p string) error { handed = p; return nil }

	st, err := s.CheckForUpdates(context.Background())
	if err != nil || !st.Available || st.Latest == nil || st.Latest.Version != "0.4.0-alpha" || st.Current != "0.3.0-alpha" || st.CheckedAt == 0 {
		t.Fatalf("check = %+v, %v", st, err)
	}
	st, err = s.InstallUpdate(context.Background())
	if err != nil || st.State != UpdateInstalling {
		t.Fatalf("install = %+v, %v", st, err)
	}
	if got, _ := os.ReadFile(handed); string(got) != "installer bytes" || !strings.HasSuffix(handed, "uncli-0.4.0-alpha-windows-x64-setup.exe") {
		t.Errorf("handed the installer %s: %q", handed, got)
	}
}

func TestUpdatesSayWhenUpToDate(t *testing.T) {
	asRelease(t, "0.4.0-alpha")
	s := newTestService(t)
	s.initUpdates()
	srv := releaseServer(t, []byte("x"))
	s.updates.url, s.updates.client = srv.URL+"/releases", srv.Client()
	st, err := s.CheckForUpdates(context.Background())
	if err != nil || st.Available || st.State != UpdateIdle {
		t.Fatalf("check = %+v, %v", st, err)
	}
	if _, err := s.InstallUpdate(context.Background()); err == nil {
		t.Error("installed with nothing newer")
	}
}

func TestUpdatesNeverCheckInADevelopmentBuild(t *testing.T) {
	asRelease(t, "dev")
	s := newTestService(t)
	s.initUpdates()
	s.updates.url = "http://127.0.0.1:1/never"
	if st, err := s.CheckForUpdates(context.Background()); err == nil || !st.Dev || !strings.Contains(err.Error(), "npm run install-build") {
		t.Errorf("check = %+v, %v", st, err)
	}
}

func TestUpdateCheckPreference(t *testing.T) {
	s := newTestService(t)
	if s.Preferences().UpdateCheck != UpdateAuto {
		t.Fatalf("default = %q", s.Preferences().UpdateCheck)
	}
	p := s.Preferences()
	p.UpdateCheck = UpdateOff
	if saved, err := s.SetPreferences(p); err != nil || saved.UpdateCheck != UpdateOff {
		t.Fatalf("saved %+v, %v", saved, err)
	}
	p.UpdateCheck = "sometimes"
	if _, err := s.SetPreferences(p); err == nil {
		t.Error("accepted an unknown setting")
	}
}
