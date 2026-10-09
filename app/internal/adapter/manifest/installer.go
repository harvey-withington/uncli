package manifest

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

// Installer downloads a plugin's CLI into UNCLI's cache: the builds the
// manifest pins, or the latest from its release manifest, always checked
// against a checksum. It never touches the user's own install.
type Installer struct {
	M      *Manifest
	Dir    string // <user cache>/uncli/cli/<id>
	GOOS   string // detected when empty
	GOARCH string
	Client *http.Client

	mu sync.Mutex
}

func NewInstaller(m *Manifest, dir string) *Installer {
	return &Installer{M: m, Dir: dir, Client: &http.Client{Timeout: 30 * time.Minute}}
}

func (i *Installer) Pinned() string { return i.M.Install.Pinned }

func (i *Installer) platform() string {
	goos, goarch := i.GOOS, i.GOARCH
	if goos == "" {
		goos, goarch = runtime.GOOS, runtime.GOARCH
	}
	return goos + "/" + goarch
}

func (i *Installer) binPath(version string) string {
	name := i.M.Install.Binary
	if strings.HasPrefix(i.platform(), "windows/") && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	return filepath.Join(i.Dir, version, name)
}

func (i *Installer) Path(version string) (string, bool) {
	if !versionRE.MatchString(version) {
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

// latest reads the release manifest for this platform's newest build.
func (i *Installer) latest(ctx context.Context) (string, Build, error) {
	l := i.M.Install.Latest
	if l == nil {
		return "", Build{}, errors.New("no release manifest")
	}
	plat := l.Platforms[i.platform()]
	if plat == "" {
		return "", Build{}, fmt.Errorf("no %s build for %s", i.M.Name, i.platform())
	}
	resp, err := i.get(ctx, strings.ReplaceAll(l.URL, "{platform}", plat))
	if err != nil {
		return "", Build{}, err
	}
	defer resp.Body.Close()
	var doc any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&doc); err != nil {
		return "", Build{}, fmt.Errorf("release manifest: %w", err)
	}
	v, b := str(doc, l.Version), Build{URL: str(doc, l.Binary), SHA512: str(doc, l.SHA512), SHA256: str(doc, l.SHA256)}
	if !versionRE.MatchString(v) || !strings.HasPrefix(b.URL, "https://") || (len(b.SHA512) != 128 && len(b.SHA256) != 64) {
		return "", Build{}, errors.New("release manifest: unexpected content")
	}
	return v, b, nil
}

// Channels reports the latest build; with no release manifest, the pinned.
func (i *Installer) Channels(ctx context.Context) (map[string]string, error) {
	if i.M.Install.Latest == nil {
		return map[string]string{"stable": i.Pinned(), "latest": i.Pinned()}, nil
	}
	v, _, err := i.latest(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{"stable": v, "latest": v}, nil
}

func (i *Installer) build(ctx context.Context, version string) (Build, error) {
	if b, ok := i.M.Install.Builds[version][i.platform()]; ok {
		return b, nil
	}
	if i.M.Install.Latest == nil {
		return Build{}, fmt.Errorf("%s %s has no build for %s", i.M.Name, version, i.platform())
	}
	v, b, err := i.latest(ctx)
	if err != nil {
		return Build{}, err
	}
	if v != version {
		return Build{}, fmt.Errorf("%s %s can't be downloaded: only the pinned %s and the latest %s can", i.M.Name, version, i.Pinned(), v)
	}
	return b, nil
}

// Ensure returns the binary for version, downloading and checking it first
// if it isn't in the cache.
func (i *Installer) Ensure(ctx context.Context, version string, progress func(done, total int64)) (string, error) {
	if !versionRE.MatchString(version) {
		return "", fmt.Errorf("invalid version %q", version)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if p, ok := i.Path(version); ok {
		return p, nil
	}
	b, err := i.build(ctx, version)
	if err != nil {
		return "", err
	}
	var h hash.Hash
	want := b.SHA512
	if want != "" {
		h = sha512.New()
	} else {
		h, want = sha256.New(), b.SHA256
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
	pw := &progressWriter{total: resp.ContentLength, fn: progress}
	_, err = io.Copy(io.MultiWriter(tmp, h, pw), resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return "", errors.New("download: checksum does not match")
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
