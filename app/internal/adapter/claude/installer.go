package claude

import (
	"context"
	"crypto/sha256"
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

// DefaultBaseURL is the official Claude Code distribution.
const DefaultBaseURL = "https://downloads.claude.ai/claude-code-releases"

var versionRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// Installer downloads pinned Claude Code binaries into UNCLI's own cache,
// verified against the release manifest's SHA-256. It never touches the
// user's own install.
type Installer struct {
	Dir      string // <user cache>/uncli/cli/claude
	BaseURL  string
	Platform string // e.g. win32-x64; detected when empty
	Client   *http.Client

	mu sync.Mutex
}

func NewInstaller(dir string) *Installer {
	return &Installer{Dir: dir, BaseURL: DefaultBaseURL, Client: &http.Client{Timeout: 30 * time.Minute}}
}

func (i *Installer) Pinned() string { return PinnedVersion }

func (i *Installer) platform() string {
	if i.Platform != "" {
		return i.Platform
	}
	return DetectPlatform(runtime.GOOS, runtime.GOARCH)
}

// DetectPlatform maps Go's names to the distribution's platform keys.
func DetectPlatform(goos, goarch string) string {
	os := map[string]string{"windows": "win32", "darwin": "darwin", "linux": "linux"}[goos]
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[goarch]
	if os == "" || arch == "" {
		return ""
	}
	return os + "-" + arch
}

func binaryName(platform string) string {
	if strings.HasPrefix(platform, "win32") {
		return "claude.exe"
	}
	return "claude"
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
	Version   string `json:"version"`
	Platforms map[string]struct {
		Binary   string `json:"binary"`
		Checksum string `json:"checksum"`
		Size     int64  `json:"size"`
	} `json:"platforms"`
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

// Channels reports what the stable and latest channels point at.
func (i *Installer) Channels(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for _, ch := range []string{"stable", "latest"} {
		resp, err := i.get(ctx, i.BaseURL+"/"+ch)
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		v := strings.TrimSpace(string(b))
		if !versionRe.MatchString(v) {
			return nil, fmt.Errorf("channel %s: unexpected content", ch)
		}
		out[ch] = v
	}
	return out, nil
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
	plat := i.platform()
	if plat == "" {
		return "", fmt.Errorf("no Claude Code build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	resp, err := i.get(ctx, i.BaseURL+"/"+version+"/manifest.json")
	if err != nil {
		return "", err
	}
	var m manifest
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("manifest: %w", err)
	}
	entry, ok := m.Platforms[plat]
	if !ok || entry.Checksum == "" {
		return "", fmt.Errorf("manifest has no %s build", plat)
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

	resp, err = i.get(ctx, i.BaseURL+"/"+version+"/"+plat+"/"+binaryName(plat))
	if err != nil {
		tmp.Close()
		return "", err
	}
	defer resp.Body.Close()
	total := entry.Size
	if total == 0 {
		total = resp.ContentLength
	}
	h := sha256.New()
	pw := &progressWriter{total: total, fn: progress}
	n, err := io.Copy(io.MultiWriter(tmp, h, pw), resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	if entry.Size > 0 && n != entry.Size {
		return "", fmt.Errorf("download: got %d bytes, manifest says %d", n, entry.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, entry.Checksum) {
		return "", errors.New("download: checksum does not match the release manifest")
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
