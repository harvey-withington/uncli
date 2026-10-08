package antigravity

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ManifestURL is where the official installer learns the latest build: its
// version, a versioned download URL and the binary's SHA-512.
const ManifestURL = "https://antigravity-cli-auto-updater-974169037036.us-central1.run.app/manifests"

var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Build is one release of the CLI for one platform.
type Build struct {
	URL    string `json:"url"`
	SHA512 string `json:"sha512"`
}

// pinned are the builds this UNCLI is tested with. The manifest only ever
// describes the latest build, so a pinned one is fetched from its own
// versioned URL and checked against the checksum recorded here.
var pinned = map[string]map[string]Build{
	PinnedVersion: {
		"windows_amd64": {
			URL:    "https://storage.googleapis.com/antigravity-public/antigravity-cli/1.3.1-4582356770750464/windows-x64/cli_windows_x64.exe",
			SHA512: "f346d2cd9d68e7e5395cb94ff95e7e8ab2e0c65cf2ad835d0eb671fb41f12f5fc713d7a26ceb4510678eb9baa62c6cd9ab8205211e3661c6cd1e23ca7bc69781",
		},
	},
}

// Installer downloads the Antigravity CLI into UNCLI's own cache, checked
// against its SHA-512. It never touches the user's own install
// (%LOCALAPPDATA%\agy), and UNCLI keeps the CLI from updating itself.
type Installer struct {
	Dir         string // <user cache>/uncli/cli/antigravity
	ManifestURL string
	Platform    string // e.g. windows_amd64; detected when empty
	Client      *http.Client

	mu sync.Mutex
}

func NewInstaller(dir string) *Installer {
	return &Installer{Dir: dir, ManifestURL: ManifestURL, Client: &http.Client{Timeout: 30 * time.Minute}}
}

func (i *Installer) Pinned() string { return PinnedVersion }

func (i *Installer) platform() string {
	if i.Platform != "" {
		return i.Platform
	}
	return DetectPlatform(runtime.GOOS, runtime.GOARCH)
}

// DetectPlatform maps Go's names to the manifest's platform keys.
func DetectPlatform(goos, goarch string) string {
	os := map[string]string{"windows": "windows", "darwin": "darwin", "linux": "linux"}[goos]
	arch := map[string]string{"amd64": "amd64", "arm64": "arm64"}[goarch]
	if os == "" || arch == "" {
		return ""
	}
	return os + "_" + arch
}

func binaryName(platform string) string {
	if strings.HasPrefix(platform, "windows") {
		return "agy.exe"
	}
	return "agy"
}

func (i *Installer) binPath(version string) string {
	return filepath.Join(i.Dir, version, binaryName(i.platform()))
}

func (i *Installer) Path(version string) (string, bool) {
	if !versionRe.MatchString(version) {
		return "", false
	}
	p := i.binPath(version)
	st, err := os.Stat(p)
	return p, err == nil && st.Mode().IsRegular()
}

func (i *Installer) Installed() ([]string, error) {
	entries, err := os.ReadDir(i.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if _, ok := i.Path(e.Name()); e.IsDir() && ok {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

type manifest struct {
	Version string `json:"version"`
	Build
}

func (i *Installer) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := i.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

func (i *Installer) latest(ctx context.Context) (manifest, error) {
	var m manifest
	resp, err := i.get(ctx, i.ManifestURL+"/"+i.platform()+".json")
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&m); err != nil {
		return m, fmt.Errorf("manifest: %w", err)
	}
	if !versionRe.MatchString(m.Version) || !strings.HasPrefix(m.URL, "https://") || len(m.SHA512) != 128 {
		return m, errors.New("manifest: unexpected content")
	}
	return m, nil
}

// Channels reports the latest build. The CLI has no separate stable
// channel: what the manifest serves is what its own installer installs.
func (i *Installer) Channels(ctx context.Context) (map[string]string, error) {
	m, err := i.latest(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{"stable": m.Version, "latest": m.Version}, nil
}

// build finds where to fetch version from: UNCLI's own table, or the
// manifest when it is the latest.
func (i *Installer) build(ctx context.Context, version string) (Build, error) {
	if b, ok := pinned[version][i.platform()]; ok {
		return b, nil
	}
	m, err := i.latest(ctx)
	if err != nil {
		return Build{}, err
	}
	if m.Version != version {
		return Build{}, fmt.Errorf("Antigravity CLI %s can't be downloaded: only %s (pinned) and %s (the latest) can", version, PinnedVersion, m.Version)
	}
	return m.Build, nil
}

// Ensure returns the binary for version, downloading and verifying it first
// if it isn't in the cache.
func (i *Installer) Ensure(ctx context.Context, version string, progress func(done, total int64)) (string, error) {
	if !versionRe.MatchString(version) {
		return "", fmt.Errorf("invalid version %q", version)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if p, ok := i.Path(version); ok {
		return p, nil
	}
	if i.platform() == "" {
		return "", fmt.Errorf("no Antigravity CLI build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	b, err := i.build(ctx, version)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(i.Dir, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	resp, err := i.get(ctx, b.URL)
	if err != nil {
		tmp.Close()
		return "", err
	}
	defer resp.Body.Close()
	h := sha512.New()
	pw := &progressWriter{total: resp.ContentLength, fn: progress}
	_, err = io.Copy(io.MultiWriter(tmp, h, pw), resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, b.SHA512) {
		return "", errors.New("download: checksum does not match the release")
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	final := i.binPath(version)
	if err := os.Rename(tmp.Name(), final); err != nil {
		return "", err
	}
	return final, nil
}

type progressWriter struct {
	done, total int64
	last        time.Time
	fn          func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.fn != nil && (time.Since(p.last) > 100*time.Millisecond || p.done == p.total) {
		p.last = time.Now()
		p.fn(p.done, p.total)
	}
	return len(b), nil
}
