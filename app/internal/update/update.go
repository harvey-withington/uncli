// Package update finds newer UNCLI releases on GitHub and downloads their
// installer, checked against the release's SHA256SUMS.txt (decision 0025).
// Installing is the install script's job (install-build.ps1, embedded).
package update

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
	"strconv"
	"strings"
	"time"
)

// ReleasesURL lists UNCLI's releases, newest first.
const ReleasesURL = "https://api.github.com/repos/harvey-withington/uncli/releases?per_page=20"

// Release is a release that can be installed: its version, its page, and
// its installer with the checksum file beside it.
type Release struct {
	Version   string    `json:"version"` // without the v: 0.4.0-alpha
	URL       string    `json:"url"`     // its page on GitHub
	Published time.Time `json:"published"`
	Setup     string    `json:"setup"` // the installer's file name
	setupURL  string
	setupSize int64
	sumsURL   string
}

// SetupName is a release's installer, as release.mjs names it.
func SetupName(version string) string { return "uncli-" + version + "-windows-x64-setup.exe" }

type ghRelease struct {
	Tag        string    `json:"tag_name"`
	Draft      bool      `json:"draft"`
	URL        string    `json:"html_url"`
	Published  time.Time `json:"published_at"`
	Assets     []ghAsset `json:"assets"`
	Prerelease bool      `json:"prerelease"`
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Latest is the newest release that has an installer and its checksums.
// Pre-releases count: every UNCLI release so far is one.
func Latest(ctx context.Context, c *http.Client, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "UNCLI")
	res, err := c.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub answered %s", res.Status)
	}
	var list []ghRelease
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&list); err != nil {
		return Release{}, fmt.Errorf("GitHub sent something unexpected: %w", err)
	}
	var best Release
	for _, r := range list {
		v := strings.TrimPrefix(r.Tag, "v")
		if r.Draft || !Valid(v) || (best.Version != "" && Compare(v, best.Version) <= 0) {
			continue
		}
		rel := Release{Version: v, URL: r.URL, Published: r.Published, Setup: SetupName(v)}
		for _, a := range r.Assets {
			switch a.Name {
			case rel.Setup:
				rel.setupURL, rel.setupSize = a.URL, a.Size
			case "SHA256SUMS.txt":
				rel.sumsURL = a.URL
			}
		}
		if rel.setupURL != "" && rel.sumsURL != "" {
			best = rel
		}
	}
	if best.Version == "" {
		return Release{}, errors.New("no release with an installer was found")
	}
	return best, nil
}

// Download fetches a release's installer into dir and checks it against the
// release's SHA256SUMS.txt; a file that doesn't match is deleted. progress,
// if set, is told how far it's got (0 to 100).
func Download(ctx context.Context, c *http.Client, r Release, dir string, progress func(int)) (string, error) {
	sums, err := fetch(ctx, c, r.sumsURL, 1<<20)
	if err != nil {
		return "", fmt.Errorf("couldn't get the checksums: %w", err)
	}
	want, ok := sumFor(string(sums), r.Setup)
	if !ok {
		return "", fmt.Errorf("SHA256SUMS.txt doesn't list %s", r.Setup)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, r.Setup)
	tmp := path + ".part"
	_ = os.Remove(tmp)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.setupURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "UNCLI")
	res, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't download %s: %w", r.Setup, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("couldn't download %s: %s", r.Setup, res.Status)
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	total := res.ContentLength
	if total <= 0 {
		total = r.setupSize
	}
	w := &counter{w: io.MultiWriter(f, h), total: total, progress: progress}
	_, err = io.Copy(w, io.LimitReader(res.Body, maxSetup+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && w.n > maxSetup {
		err = errors.New("the installer is far larger than expected")
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = fmt.Errorf("%s doesn't match its SHA-256 in SHA256SUMS.txt, so it wasn't kept", r.Setup)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

// maxSetup is far above an installer's size (about 14 MB): a bound on what
// a download may write.
const maxSetup = 256 << 20

type counter struct {
	w        io.Writer
	n, total int64
	last     int
	progress func(int)
}

func (c *counter) Write(p []byte) (int, error) {
	k, err := c.w.Write(p)
	c.n += int64(k)
	if c.progress != nil && c.total > 0 {
		if pc := int(c.n * 100 / c.total); pc != c.last && pc <= 100 {
			c.last = pc
			c.progress(pc)
		}
	}
	return k, err
}

func fetch(ctx context.Context, c *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "UNCLI")
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, limit))
}

// sumFor finds a file's SHA-256 in a SHA256SUMS.txt ("<hex>  <name>" lines,
// a "*" before the name for binary mode).
func sumFor(sums, name string) (string, bool) {
	for _, line := range strings.Split(strings.ReplaceAll(sums, "\r\n", "\n"), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0]), true
		}
	}
	return "", false
}

// Valid says v is a version UNCLI releases use: 1.2.3, or 1.2.3 with a
// pre-release part (1.2.3-alpha, 1.2.3-rc.1).
func Valid(v string) bool {
	core, _, _ := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil || p == "" {
			return false
		}
	}
	return true
}

// Compare orders versions as semantic versioning does: by the numbers, then
// a release before any of its pre-releases (1.0.0 > 1.0.0-rc.1), whose
// parts compare as numbers when both are, else as text. -1, 0 or 1.
func Compare(a, b string) int {
	ac, ap, _ := strings.Cut(a, "-")
	bc, bp, _ := strings.Cut(b, "-")
	an, bn := strings.Split(ac, "."), strings.Split(bc, ".")
	for i := 0; i < 3; i++ {
		if c := cmpInt(at(an, i), at(bn, i)); c != 0 {
			return c
		}
	}
	switch {
	case ap == bp:
		return 0
	case ap == "":
		return 1
	case bp == "":
		return -1
	}
	ai, bi := strings.Split(ap, "."), strings.Split(bp, ".")
	for i := 0; i < len(ai) && i < len(bi); i++ {
		x, xerr := strconv.Atoi(ai[i])
		y, yerr := strconv.Atoi(bi[i])
		var c int
		switch {
		case xerr == nil && yerr == nil:
			c = cmp(x, y)
		case xerr == nil:
			c = -1 // numbers come before words
		case yerr == nil:
			c = 1
		default:
			c = strings.Compare(ai[i], bi[i])
		}
		if c != 0 {
			return c
		}
	}
	return cmp(len(ai), len(bi))
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "0"
}

func cmpInt(a, b string) int {
	x, _ := strconv.Atoi(a)
	y, _ := strconv.Atoi(b)
	return cmp(x, y)
}

func cmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
