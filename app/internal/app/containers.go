package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"uncli/config"
	"uncli/internal/adapter/claude"
	"uncli/internal/core"
	"uncli/internal/runtime/wsl"
	"uncli/internal/session"
)

// Containers are WSL distros UNCLI builds from container profiles
// (decision 0011). Profiles are config (containers.yaml); a built profile
// is a distro named uncli-<id>.

// EvtContainers tells the UI the containers changed (ContainersInfo).
const EvtContainers = "containers:changed"

// secretContainerToken names the CLI's OAuth token for containers in the
// credential store (from claude setup-token; container-signin.go).
const secretContainerToken = "claude-oauth-token"

// Base is a pinned root filesystem.
type Base struct {
	ID     string `yaml:"id" json:"id"`
	Label  string `yaml:"label" json:"label"`
	URL    string `yaml:"url" json:"-"`
	SHA256 string `yaml:"sha256" json:"-"`
	CLI    string `yaml:"cli" json:"-"` // the CLI build's platform, e.g. linux-x64-musl
}

// ContainerProfile is one container a session can run in.
type ContainerProfile struct {
	ID          string   `yaml:"id" json:"id"`
	Label       string   `yaml:"label" json:"label"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Base        string   `yaml:"base" json:"base"`
	Packages    []string `yaml:"packages" json:"packages"`
	// Brain is what the container knows of the user: "shared" mounts their
	// memories, skills, agents and commands and copies in their CLAUDE.md
	// and git name; "sandboxed" (the default) keeps its own.
	Brain string `yaml:"brain" json:"brain"`
	// MCP is "shared" to give the container the user's MCP servers that can
	// run on Linux; "none" (the default) gives it none.
	MCP string `yaml:"mcp" json:"mcp"`
	// Connectors is "shared" to give the container the user's claude.ai
	// connectors (Jira, Office…), which needs the provider's full account
	// sign-in; "none" (the default) keeps it to the models-only token.
	Connectors string `yaml:"connectors" json:"connectors"`
}

const (
	BrainShared      = "shared"
	BrainSandboxed   = "sandboxed"
	MCPShared        = "shared"
	MCPNone          = "none"
	ConnectorsShared = "shared"
	ConnectorsNone   = "none"
)

type containersFile struct {
	Bases      []Base             `yaml:"bases"`
	Containers []ContainerProfile `yaml:"containers"`
}

// loadContainers reads the built-in containers.yaml and merges the user's
// over it by id.
func loadContainers(defaults fs.FS, userDir string) (containersFile, error) {
	var c containersFile
	b, err := fs.ReadFile(defaults, "containers.yaml")
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("built-in containers.yaml: %w", err)
	}
	if userDir != "" {
		b, err := os.ReadFile(filepath.Join(userDir, "containers.yaml"))
		if err == nil {
			var u containersFile
			if err := yaml.Unmarshal(b, &u); err != nil {
				return c, fmt.Errorf("%s: %w", filepath.Join(userDir, "containers.yaml"), err)
			}
			c.Bases = mergeID(c.Bases, u.Bases, func(b Base) string { return b.ID })
			c.Containers = mergeID(c.Containers, u.Containers, func(p ContainerProfile) string { return p.ID })
		} else if !errors.Is(err, os.ErrNotExist) {
			return c, err
		}
	}
	for _, p := range c.Containers {
		if !wsl.ValidProfile(p.ID) {
			return c, fmt.Errorf("container id %q: use lower-case letters, digits and dashes", p.ID)
		}
		if !slices.ContainsFunc(c.Bases, func(b Base) bool { return b.ID == p.Base }) {
			return c, fmt.Errorf("container %q: no base %q", p.ID, p.Base)
		}
		if p.Brain != "" && p.Brain != BrainShared && p.Brain != BrainSandboxed {
			return c, fmt.Errorf("container %q: brain is shared or sandboxed, not %q", p.ID, p.Brain)
		}
		if p.MCP != "" && p.MCP != MCPShared && p.MCP != MCPNone {
			return c, fmt.Errorf("container %q: mcp is shared or none, not %q", p.ID, p.MCP)
		}
		if p.Connectors != "" && p.Connectors != ConnectorsShared && p.Connectors != ConnectorsNone {
			return c, fmt.Errorf("container %q: connectors is shared or none, not %q", p.ID, p.Connectors)
		}
	}
	return c, nil
}

func mergeID[T any](base, over []T, id func(T) string) []T {
	out := append([]T(nil), base...)
	for _, o := range over {
		i := slices.IndexFunc(out, func(x T) bool { return id(x) == id(o) })
		if i >= 0 {
			out[i] = o
		} else {
			out = append(out, o)
		}
	}
	return out
}

// containers is the service's container state.
type containers struct {
	mu       sync.Mutex
	cfg      containersFile
	err      error             // a broken containers.yaml
	building map[string]string // profile id -> step
	percent  map[string]int    // profile id -> how much of a download is done (0-100)
	failed   map[string]string // profile id -> why its last build failed
	runtimes map[string]*wsl.Runtime
	status   wsl.Status // WSL as last asked; progress is sent with this
	emitMu   sync.Mutex // updates leave in the order they were made
}

// checkWSL asks wsl.exe (tests replace it).
var checkWSL = wsl.CheckStatus

// ContainerInfo is a profile as the UI shows it.
type ContainerInfo struct {
	ContainerProfile
	BaseLabel string `json:"baseLabel"`
	Built     bool   `json:"built"`
	Step      string `json:"step,omitempty"`    // building: download, remove, import, packages, cli, lockdown, check
	Percent   int    `json:"percent,omitempty"` // download: how much is done, 0 when not known
	Error     string `json:"error,omitempty"`   // the last build failed
	// Changes: what differs from how a built container was built (packages,
	// base, cli, setup, or unknown when it predates the record); a rebuild
	// brings it up to date.
	Changes []string `json:"changes,omitempty"`
}

// ContainersInfo is everything the containers settings show.
type ContainersInfo struct {
	WSL      wsl.Status `json:"wsl"`
	SignedIn bool       `json:"signedIn"`
	// AccountSignedIn: the provider's full account sign-in is kept, for
	// containers that share connectors.
	AccountSignedIn bool            `json:"accountSignedIn"`
	Containers      []ContainerInfo `json:"containers"`
	Error           string          `json:"error,omitempty"` // containers.yaml couldn't be read
}

// Containers asks WSL afresh and reports it, the sign-in and each profile.
func (s *Service) Containers(ctx context.Context) ContainersInfo {
	s.reloadContainers()
	st, _ := checkWSL(ctx)
	s.containers.mu.Lock()
	s.containers.status = st
	s.containers.mu.Unlock()
	return s.containersInfo()
}

// containersInfo reports with WSL as last asked.
func (s *Service) containersInfo() ContainersInfo {
	c := &s.containers
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.status
	if st.Distros == nil {
		st.Distros = []string{}
	}
	info := ContainersInfo{WSL: st, SignedIn: s.containerToken() != "", AccountSignedIn: s.containerAccount() != "", Containers: []ContainerInfo{}}
	if c.err != nil {
		info.Error = c.err.Error()
	}
	for _, p := range c.cfg.Containers {
		ci := ContainerInfo{ContainerProfile: p, Built: slices.Contains(st.Distros, wsl.DistroName(p.ID)),
			Step: c.building[p.ID], Percent: c.percent[p.ID], Error: c.failed[p.ID]}
		if ci.Packages == nil {
			ci.Packages = []string{}
		}
		for _, b := range c.cfg.Bases {
			if b.ID == p.Base {
				ci.BaseLabel = b.Label
				if ci.Built && ci.Step == "" {
					ci.Changes = s.changesSince(p.ID, recordFor(p, b, s.cliVersion()))
				}
			}
		}
		info.Containers = append(info.Containers, ci)
	}
	return info
}

// emitContainers sends the containers straight away, with WSL as last
// asked: asking again means waiting on wsl.exe, which queues behind a
// build's own WSL work, so progress would arrive late or out of order.
func (s *Service) emitContainers() {
	s.containers.emitMu.Lock()
	defer s.containers.emitMu.Unlock()
	s.emit.Emit(EvtContainers, s.containersInfo())
}

// refreshContainers asks WSL afresh (after a build or a removal), then sends.
func (s *Service) refreshContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.Containers(ctx)
	s.emitContainers()
}

func (s *Service) containerToken() string { return s.secret(secretContainerToken, "") }

// InstallWSL turns WSL on (one administrator prompt; a restart follows).
func (s *Service) InstallWSL(ctx context.Context) error { return wsl.InstallWSL(ctx) }

func (s *Service) profileAndBase(id string) (ContainerProfile, Base, error) {
	c := &s.containers
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.cfg.Containers {
		if p.ID != id {
			continue
		}
		for _, b := range c.cfg.Bases {
			if b.ID == p.Base {
				return p, b, nil
			}
		}
	}
	return ContainerProfile{}, Base{}, fmt.Errorf("no container %q", id)
}

// BuildContainer builds (or rebuilds) a profile's distro in the background;
// progress and the outcome arrive as EvtContainers.
func (s *Service) BuildContainer(id string) error {
	p, b, err := s.profileAndBase(id)
	if err != nil {
		return err
	}
	c := &s.containers
	c.mu.Lock()
	if c.building[id] != "" {
		c.mu.Unlock()
		return errors.New("that container is already being built")
	}
	c.building[id] = "download"
	delete(c.failed, id)
	c.mu.Unlock()
	s.emitContainers()
	go func() {
		rec := recordFor(p, b, s.cliVersion())
		err := s.buildContainer(p, b)
		if err == nil {
			err = s.saveBuildRecord(id, rec)
		}
		c.mu.Lock()
		delete(c.building, id)
		if err != nil {
			c.failed[id] = err.Error()
		}
		c.mu.Unlock()
		s.refreshContainers()
	}()
	return nil
}

func (s *Service) buildContainer(p ContainerProfile, b Base) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	step := func(name string) {
		s.containers.mu.Lock()
		s.containers.building[p.ID] = name
		delete(s.containers.percent, p.ID)
		s.containers.mu.Unlock()
		s.emitContainers()
	}
	// Downloads say how far they've got, a whole percent at a time.
	progress := func(done, total int64) {
		if total <= 0 {
			return
		}
		pct := int(done * 100 / total)
		s.containers.mu.Lock()
		changed := s.containers.percent[p.ID] != pct
		s.containers.percent[p.ID] = pct
		s.containers.mu.Unlock()
		if changed {
			s.emitContainers()
		}
	}
	rootfs, err := fetchVerified(ctx, b.URL, b.SHA256, filepath.Join(s.Paths.Cache, "wsl", b.ID+".tar.gz"), progress)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", b.Label, err)
	}
	step("download") // the CLI, from 0 again
	inst := claude.NewInstaller(filepath.Join(s.Paths.Cache, "cli", "claude-"+b.CLI))
	inst.Platform = b.CLI
	cli, err := inst.Ensure(ctx, s.cliVersion(), progress)
	if err != nil {
		return fmt.Errorf("downloading the CLI for %s: %w", b.Label, err)
	}
	name := wsl.DistroName(p.ID)
	step("remove") // a rebuild starts from scratch: its sessions stop first
	if s.Sessions != nil {
		s.Sessions.StopIn(session.RuntimeWSL, p.ID)
	}
	s.closeRuntime(p.ID)
	if err := wsl.Remove(ctx, name); errors.Is(err, wsl.ErrUnresponsive) {
		return err
	}
	dir := filepath.Join(s.Paths.Config, "wsl", p.ID)
	_ = os.RemoveAll(dir)
	return wsl.Build(ctx, wsl.Spec{Name: name, Dir: dir, RootFS: rootfs, CLI: cli, Packages: p.Packages}, step)
}

// stopContainer stops a container gently (WSL is shared with other tools:
// Docker, Podman, dev containers): its sessions' CLIs are asked to end,
// then the process holding it up, then WSL's own terminate of that distro
// alone. Nothing is killed by name and no other distro is touched.
func (s *Service) stopContainer(ctx context.Context, id string) error {
	if s.Sessions != nil {
		s.Sessions.StopIn(session.RuntimeWSL, id)
	}
	s.closeRuntime(id)
	return wsl.Terminate(ctx, wsl.DistroName(id))
}

// StopContainers stops UNCLI's own containers, for when one is stuck or
// before Windows restarts WSL. It never runs wsl --shutdown, which would
// stop every other tool's distros and containers too.
func (s *Service) StopContainers(ctx context.Context) error {
	st, _ := checkWSL(ctx)
	var errs []error
	for _, d := range st.Distros {
		id := strings.TrimPrefix(d, "uncli-")
		if _, ok := s.profile(id); ok {
			errs = append(errs, s.stopContainer(ctx, id))
		}
	}
	s.refreshContainers()
	return errors.Join(errs...)
}

// RemoveContainer deletes a profile's distro and everything in it.
func (s *Service) RemoveContainer(ctx context.Context, id string) error {
	if _, _, err := s.profileAndBase(id); err != nil {
		return err
	}
	if s.Sessions != nil {
		s.Sessions.StopIn(session.RuntimeWSL, id)
	}
	s.closeRuntime(id)
	if err := wsl.Remove(ctx, wsl.DistroName(id)); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(s.Paths.Config, "wsl", id))
	s.refreshContainers()
	return nil
}

// reloadContainers reads containers.yaml again, so an edit shows without
// restarting UNCLI (a broken file keeps the last good one, and says why).
// What a profile shares applies from its sessions' next start.
func (s *Service) reloadContainers() {
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	cfg, err := loadContainers(defaults, s.Paths.Config)
	c := &s.containers
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
	if err != nil {
		return
	}
	c.cfg = cfg
	for id, r := range c.runtimes {
		for _, p := range cfg.Containers {
			if p.ID == id {
				r.Env = nil
				if p.MCP == MCPShared || p.Connectors == ConnectorsShared {
					r.Env = claude.ContainerMCPEnv()
				}
			}
		}
	}
}

// profile is a container profile as configured now.
func (s *Service) profile(id string) (ContainerProfile, bool) {
	c := &s.containers
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.cfg.Containers {
		if p.ID == id {
			return p, true
		}
	}
	return ContainerProfile{}, false
}

// runtimeFor is session.Deps.Runtimes: the runtime of a container profile,
// one per profile, kept while UNCLI runs.
func (s *Service) runtimeFor(id, ref string) (core.Runtime, error) {
	if id != session.RuntimeWSL {
		return nil, fmt.Errorf("this session runs in %s, which this UNCLI can't start", id)
	}
	if _, _, err := s.profileAndBase(ref); err != nil {
		return nil, err
	}
	c := &s.containers
	c.mu.Lock()
	defer c.mu.Unlock()
	if r := c.runtimes[ref]; r != nil {
		return r, nil
	}
	r := wsl.New(ref, func() (string, error) {
		if t := s.containerToken(); t != "" {
			return t, nil
		}
		return "", wsl.ErrNoToken
	})
	for _, p := range c.cfg.Containers {
		if p.ID == ref {
			r.Prepare = prepareContainer(func() ContainerProfile { q, _ := s.profile(ref); return q }, filepath.Join(s.Paths.Config, "wsl-data", p.ID), s.containerAccount)
			r.Account = func() bool {
				q, _ := s.profile(ref)
				return q.Connectors == ConnectorsShared && s.containerAccount() != ""
			}
		}
		if p.ID == ref && (p.MCP == MCPShared || p.Connectors == ConnectorsShared) {
			r.Env = claude.ContainerMCPEnv() // their tools in the first message too
		}
	}
	c.runtimes[ref] = r
	return r, nil
}

func (s *Service) closeRuntime(id string) {
	c := &s.containers
	c.mu.Lock()
	r := c.runtimes[id]
	delete(c.runtimes, id)
	c.mu.Unlock()
	if r != nil {
		r.Close()
	}
}

func (s *Service) closeContainers() {
	c := &s.containers
	c.mu.Lock()
	rs := c.runtimes
	c.runtimes = map[string]*wsl.Runtime{}
	c.mu.Unlock()
	var wg sync.WaitGroup
	for _, r := range rs {
		wg.Add(1)
		go func() { defer wg.Done(); r.Close() }()
	}
	wg.Wait()
}

// fetchVerified downloads url to dest unless dest already holds a file with
// the expected SHA-256, and checks what it downloaded.
func fetchVerified(ctx context.Context, url, sum, dest string, progress func(done, total int64)) (string, error) {
	if ok, _ := hasSum(dest, sum); ok {
		return dest, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := fetchClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h, &counter{total: resp.ContentLength, report: progress}), io.LimitReader(resp.Body, 2<<30))
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, sum) {
		os.Remove(tmp)
		return "", errors.New("the download doesn't match its pinned checksum")
	}
	return dest, os.Rename(tmp, dest)
}

func hasSum(path, sum string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), sum), nil
}

// fetchClient gives up on a download that stalls instead of waiting forever.
var fetchClient = &http.Client{Timeout: 15 * time.Minute, Transport: &http.Transport{
	Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 30 * time.Second, TLSHandshakeTimeout: 15 * time.Second,
}}

// counter reports how much of a download has been written.
type counter struct {
	done, total int64
	report      func(done, total int64)
}

func (c *counter) Write(p []byte) (int, error) {
	c.done += int64(len(p))
	if c.report != nil {
		c.report(c.done, c.total)
	}
	return len(p), nil
}
