// Package manifest is the generic adapter for provider plugins (decision
// 0013): a CLI described as data in a provider.yaml, run as a core.Adapter.
// The built-in adapters are Go code of their own and don't use it.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is a plugin's manifest, in its folder.
const File = "provider.yaml"

// Manifest describes a CLI to UNCLI.
type Manifest struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`    // the CLI, in setup and settings
	Agent   string `yaml:"agent"`   // who acts in a session
	Account string `yaml:"account"` // the account it signs in with
	// SignIn: elsewhere (its own interface, which UNCLI opens in a terminal
	// window, or the provider's app) or device (a link and a code, approved
	// on any device; status.login prints them).
	SignIn string `yaml:"signin"`
	// Protocol: lines (the default: a line a turn, events by the rules below)
	// or acp (the Agent Client Protocol; decision 0014).
	Protocol string `yaml:"protocol"`
	// Files are written into the state folder before the CLI starts: text
	// as it is, anything else as JSON.
	Files map[string]any `yaml:"files"`

	Install Install           `yaml:"install"`
	Env     map[string]string `yaml:"env"`
	// State gives the CLI a folder of UNCLI's ({state}), and StatePaths
	// are its own bookkeeping folders in it (core.StatePather).
	State      bool     `yaml:"state"`
	StatePaths []string `yaml:"statePaths"`

	Launch       Launch          `yaml:"launch"`
	Turn         Turn            `yaml:"turn"`
	Capabilities map[string]bool `yaml:"capabilities"`
	Status       Status          `yaml:"status"`
	Models       Models          `yaml:"models"`
	// LostConversation: words on stderr that mean the CLI no longer has the
	// conversation it was asked to resume (case doesn't matter).
	LostConversation []string `yaml:"lostConversation"`
	Events           []Rule   `yaml:"events"`
	Tools            Tools    `yaml:"tools"`
	// ACP: what ACP leaves to each agent (protocol: acp).
	ACP       ACPExtras `yaml:"acp"`
	Approvals Approvals `yaml:"approvals"`
}

type Install struct {
	Pinned string                      `yaml:"pinned"` // the version this plugin was written and tested for
	Binary string                      `yaml:"binary"` // its name in UNCLI's cache (.exe is added on Windows)
	Builds map[string]map[string]Build `yaml:"builds"` // version → goos/goarch → build
	Latest *Latest                     `yaml:"latest"`
}

// Build is one download, checked against its checksum.
type Build struct {
	URL    string `yaml:"url"`
	SHA512 string `yaml:"sha512"`
	SHA256 string `yaml:"sha256"`
}

// Latest is a release manifest (JSON) that names the newest build: where
// it is ({platform} in its URL) and which of its fields hold what.
type Latest struct {
	URL       string            `yaml:"url"`
	Platforms map[string]string `yaml:"platforms"` // goos/goarch → the manifest's name for it
	Version   string            `yaml:"version"`
	Binary    string            `yaml:"binary"`
	SHA512    string            `yaml:"sha512"`
	SHA256    string            `yaml:"sha256"`
}

// Launch is how the CLI starts: Args always, and each of the others when
// it applies ({state}, {resume}, {id}, {model}, {effort}).
type Launch struct {
	Args      []string `yaml:"args"`
	Approvals []string `yaml:"approvals"` // when UNCLI's hook decides every tool call: switches the CLI's own checks off
	Resume    []string `yaml:"resume"`
	NewID     []string `yaml:"newId"` // a new conversation with UNCLI's id for it
	Model     []string `yaml:"model"`
	Effort    []string `yaml:"effort"`
	Plan      []string `yaml:"plan"` // the plan permission mode
	Tail      []string `yaml:"tail"` // always last (a subcommand the options above belong before)
}

// Turn is one line on stdin: {text} is the message (directives first).
type Turn struct {
	Line any `yaml:"line"`
}

type Status struct {
	Args []string `yaml:"args"`
	// Login: how to start the CLI for the user to sign in, in a terminal
	// window UNCLI opens (its own interface); empty: the bare binary.
	Login    []string `yaml:"login"`
	Method   string   `yaml:"method"`   // how it's signed in, to show
	SignedIn string   `yaml:"signedIn"` // a pattern its output matches when signed in (else: models listed)
	// SignedOut: a pattern its output matches when it isn't signed in; any
	// other output means it is.
	SignedOut string `yaml:"signedOut"`
	// DeviceURL and DeviceCode find the link and code a device sign-in
	// prints (defaults: the first https link, and a code like ABCD-1234).
	DeviceURL  string `yaml:"deviceUrl"`
	DeviceCode string `yaml:"deviceCode"`
}

type Models struct {
	Line string `yaml:"line"` // a pattern for one model a line: its id, and optionally its name
}

// Rule maps output lines to UNCLI's events: when every field in When
// matches, each part present applies, in this order.
type Rule struct {
	When        map[string]any `yaml:"when"`
	Session     *SessionRule   `yaml:"session"`
	TurnStarted bool           `yaml:"turnStarted"`
	Text        *TextRule      `yaml:"text"`
	Usage       *UsageRule     `yaml:"usage"`
	Tool        *ToolRule      `yaml:"tool"`
	Result      *ResultRule    `yaml:"result"`
}

type SessionRule struct {
	ID             string `yaml:"id"`
	Model          string `yaml:"model"`
	Tools          string `yaml:"tools"`
	PermissionMode string `yaml:"permissionMode"`
}

// TextRule: Delta is streamed text, gathered per Block until Done holds;
// then the block's text is complete. Whole is a complete block at once.
type TextRule struct {
	Delta string         `yaml:"delta"`
	Block string         `yaml:"block"`
	Done  map[string]any `yaml:"done"`
	Whole string         `yaml:"whole"`
}

// UsageRule adds the line's tokens to the turn's (Paths are summed).
type UsageRule struct {
	Input      Paths `yaml:"input"`
	Output     Paths `yaml:"output"`
	CacheRead  Paths `yaml:"cacheRead"`
	CacheWrite Paths `yaml:"cacheWrite"`
}

// ToolRule: a tool call, started the first time its id is seen and
// finished when Done holds.
type ToolRule struct {
	ID     string         `yaml:"id"` // a template
	Name   Paths          `yaml:"name"`
	Input  string         `yaml:"input"`
	Done   map[string]any `yaml:"done"`
	OK     map[string]any `yaml:"ok"`
	Output string         `yaml:"output"`
	Error  string         `yaml:"error"`  // present: it failed, with this message
	Denied string         `yaml:"denied"` // the error says so: refused, not failed
}

// ResultRule ends the turn.
type ResultRule struct {
	OK          map[string]any `yaml:"ok"`
	Text        string         `yaml:"text"`
	Error       string         `yaml:"error"`
	ErrorCode   string         `yaml:"errorCode"`
	Interrupted map[string]any `yaml:"interrupted"`
	// Durations: seconds or milliseconds, for the turn or (Total) the
	// process so far.
	DurationSeconds string `yaml:"durationSeconds"`
	DurationMS      string `yaml:"durationMs"`
	DurationTotal   bool   `yaml:"durationTotal"`
	// Usage: "turn" for what the turn's lines added up to, else from here.
	Usage     any    `yaml:"usage"`
	Cost      string `yaml:"cost"`
	CostTotal bool   `yaml:"costTotal"`
	Denials   string `yaml:"denials"` // a count, or a list whose length is one
}

// ACPExtras are an ACP agent's own additions UNCLI reads.
type ACPExtras struct {
	// Usage: where a prompt's result has the turn's tokens and cost (paths
	// into the result); CostScale turns the cost into US dollars.
	Usage struct {
		Input      Paths   `yaml:"input"`
		Output     Paths   `yaml:"output"`
		CacheRead  Paths   `yaml:"cacheRead"`
		CacheWrite Paths   `yaml:"cacheWrite"`
		Cost       string  `yaml:"cost"`
		CostScale  float64 `yaml:"costScale"`
	} `yaml:"usage"`
}

// Tools says what the CLI's tools do, in UNCLI's action kinds.
type Tools struct {
	Shell struct {
		Dialect map[string]string `yaml:"dialect"` // goos (or "other") → bash | powershell
	} `yaml:"shell"`
	Kinds    map[string]string `yaml:"kinds"`    // tool → kind
	Prefixes map[string]string `yaml:"prefixes"` // a tool name's prefix → kind
	Command  Paths             `yaml:"command"`  // arguments holding a shell command
	Path     Paths             `yaml:"path"`     // arguments holding a file or folder
	MCP      struct {
		Server Paths `yaml:"server"`
		Tool   Paths `yaml:"tool"`
	} `yaml:"mcp"`
	Summary Paths             `yaml:"summary"` // arguments to say in one line what a call does
	Touched map[string]string `yaml:"touched"` // tool → write | edit, for the files a page lists
}

type Approvals struct {
	Hook *Hook `yaml:"hook"`
}

// Hook: UNCLI's hook (decision 0012). Files are written before the CLI
// starts, with {hook:<event>} as the command that runs it for an event.
type Hook struct {
	Files  map[string]any       `yaml:"files"`
	Events map[string]HookEvent `yaml:"events"`
}

// HookEvent reads one hook event's calls and writes UNCLI's answers.
type HookEvent struct {
	Kind   string `yaml:"kind"` // tool | instructions
	ID     string `yaml:"id"`   // tool: a template, the same as the stream's tool id
	Name   string `yaml:"name"`
	Input  string `yaml:"input"`
	Allow  any    `yaml:"allow"`  // tool: the answer that lets it run
	Deny   any    `yaml:"deny"`   // tool: the refusal; {reason}
	Answer any    `yaml:"answer"` // instructions: {instructions}
	None   any    `yaml:"none"`   // instructions: when there are none
}

// Paths is one dotted path or several.
type Paths []string

func (p *Paths) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*p = Paths{n.Value}
		return nil
	}
	var l []string
	if err := n.Decode(&l); err != nil {
		return err
	}
	*p = l
	return nil
}

var (
	idRE      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,40}$`)
	versionRE = regexp.MustCompile(`^\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$`)
	hexRE     = regexp.MustCompile(`^[0-9a-fA-F]+$`)
)

var kinds = map[string]bool{"shell": true, "read": true, "search": true, "write": true, "edit": true, "web": true,
	"mcp": true, "question": true, "plan": true, "agent": true, "internal": true, "other": true}

// Load reads and checks a plugin's manifest. Its hash ties the user's
// approval to exactly this manifest.
func Load(dir string) (*Manifest, string, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		return nil, "", err
	}
	m, err := Parse(b)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(b)
	return m, hex.EncodeToString(sum[:]), nil
}

// Parse reads and checks a manifest.
func Parse(b []byte) (*Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	if err := m.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	return &m, nil
}

func (m *Manifest) check() error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if !idRE.MatchString(m.ID) {
		bad("id %q: use lower-case letters, digits and dashes", m.ID)
	}
	if m.Name == "" || m.Agent == "" {
		bad("a name and an agent are needed")
	}
	if m.SignIn != "" && m.SignIn != "elsewhere" && m.SignIn != "device" {
		bad("signin is elsewhere or device, not %q", m.SignIn)
	}
	if m.SignIn == "device" && len(m.Status.Login) == 0 {
		bad("a device sign-in needs status.login, the command that prints its link and code")
	}
	acp := m.Protocol == "acp"
	if m.Protocol != "" && m.Protocol != "lines" && !acp {
		bad("protocol is lines or acp, not %q", m.Protocol)
	}
	for path := range m.Files {
		if !strings.HasPrefix(path, "{state}/") || strings.Contains(path, "..") {
			bad("files %q: only files in {state} can be written", path)
		}
	}
	if len(m.Files) > 0 && !m.State {
		bad("files are written into the state folder: state must be on")
	}
	if acp {
		// ACP has its own approvals: the agent asks, UNCLI answers. Switching
		// its checks off, or a hook besides, has no place.
		if len(m.Launch.Approvals) > 0 || m.Approvals.Hook != nil {
			bad("an ACP plugin's tool calls are approved through ACP: launch.approvals and approvals.hook aren't allowed")
		}
		if len(m.Events) > 0 || m.Turn.Line != nil {
			bad("an ACP plugin has no events or turn: the protocol defines them")
		}
	}
	// Install: pinned builds over https with a checksum.
	if !versionRE.MatchString(m.Install.Pinned) {
		bad("install.pinned %q isn't a version", m.Install.Pinned)
	}
	if m.Install.Binary == "" || strings.ContainsAny(m.Install.Binary, `/\:`) {
		bad("install.binary is a plain file name")
	}
	if len(m.Install.Builds[m.Install.Pinned]) == 0 {
		bad("install.builds has nothing for the pinned version %s", m.Install.Pinned)
	}
	for v, plats := range m.Install.Builds {
		if !versionRE.MatchString(v) {
			bad("install.builds: %q isn't a version", v)
		}
		for plat, b := range plats {
			if !strings.HasPrefix(b.URL, "https://") {
				bad("install.builds %s %s: the URL must be https", v, plat)
			}
			if !(len(b.SHA512) == 128 && hexRE.MatchString(b.SHA512)) && !(len(b.SHA256) == 64 && hexRE.MatchString(b.SHA256)) {
				bad("install.builds %s %s: a sha512 or sha256 is needed", v, plat)
			}
		}
	}
	if l := m.Install.Latest; l != nil {
		if !strings.HasPrefix(l.URL, "https://") || l.Version == "" || l.Binary == "" || (l.SHA512 == "" && l.SHA256 == "") {
			bad("install.latest needs an https url and its version, binary and checksum fields")
		}
	}
	if len(m.Launch.Args) == 0 {
		bad("launch.args is empty")
	}
	if m.Turn.Line == nil && !acp {
		bad("turn.line is missing")
	}
	if len(m.Status.Args) == 0 {
		bad("status.args is empty")
	}
	if m.Status.SignedIn == "" && m.Status.SignedOut == "" && m.Models.Line == "" {
		bad("status.signedIn, status.signedOut or models.line is needed to tell whether the CLI is signed in")
	}
	for _, p := range []string{m.Status.SignedIn, m.Status.SignedOut, m.Models.Line, m.Status.DeviceURL, m.Status.DeviceCode} {
		if p != "" {
			if _, err := regexp.Compile(p); err != nil {
				bad("pattern %q: %v", p, err)
			}
		}
	}
	if !m.State && strings.Contains(strings.Join(append(append([]string{}, m.Launch.Args...), m.StatePaths...), " "), "{state}") {
		bad("{state} is used but state isn't on")
	}
	if len(m.Events) == 0 && !acp {
		bad("events has no rules")
	}
	for i, r := range m.Events {
		if len(r.When) == 0 {
			bad("events rule %d has no when", i+1)
		}
	}
	for t, k := range m.Tools.Kinds {
		if !kinds[k] {
			bad("tools.kinds %s: %q isn't a kind", t, k)
		}
	}
	for p, k := range m.Tools.Prefixes {
		if !kinds[k] {
			bad("tools.prefixes %s: %q isn't a kind", p, k)
		}
	}
	for t, how := range m.Tools.Touched {
		if how != "write" && how != "edit" {
			bad("tools.touched %s: write or edit, not %q", t, how)
		}
	}
	errs = append(errs, m.checkApprovals()...)
	return errors.Join(errs...)
}

// checkApprovals: a CLI whose own checks are switched off must route every
// tool call through UNCLI's hook (decisions 0012 and 0013).
func (m *Manifest) checkApprovals() []error {
	var errs []error
	h := m.Approvals.Hook
	if h == nil {
		if len(m.Launch.Approvals) > 0 {
			errs = append(errs, errors.New("launch.approvals switches the CLI's own checks off, so approvals.hook must decide every tool call"))
		}
		return errs
	}
	tools := 0
	for name, e := range h.Events {
		switch e.Kind {
		case "tool":
			tools++
			if e.Name == "" || e.ID == "" || e.Allow == nil || e.Deny == nil {
				errs = append(errs, fmt.Errorf("approvals.hook.events %s: a tool event needs id, name, allow and deny", name))
			}
			if !strings.Contains(fmt.Sprint(h.Files), "{hook:"+name+"}") {
				errs = append(errs, fmt.Errorf("approvals.hook.files never run the %s hook ({hook:%s})", name, name))
			}
		case "instructions":
			if e.Answer == nil {
				errs = append(errs, fmt.Errorf("approvals.hook.events %s: an instructions event needs an answer", name))
			}
		default:
			errs = append(errs, fmt.Errorf("approvals.hook.events %s: kind is tool or instructions, not %q", name, e.Kind))
		}
	}
	if tools != 1 {
		errs = append(errs, fmt.Errorf("approvals.hook needs exactly one tool event, not %d", tools))
	}
	if !m.State {
		errs = append(errs, errors.New("approvals.hook writes its files into the CLI's state folder: state must be on"))
	}
	for path := range h.Files {
		if !strings.HasPrefix(path, "{state}/") || strings.Contains(path, "..") {
			errs = append(errs, fmt.Errorf("approvals.hook.files %q: only files in {state} can be written", path))
		}
	}
	return errs
}

// HookGated says whether UNCLI's hook decides the CLI's tool calls.
func (m *Manifest) HookGated() bool { return m.Approvals.Hook != nil }

// Bypasses says whether the CLI runs with its own permission checks off.
func (m *Manifest) Bypasses() bool { return len(m.Launch.Approvals) > 0 }
