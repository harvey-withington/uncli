// Package uncli runs a UNCLI of a host's own, inside the host (decision
// 0027). The host mounts its Handler on its own server, under a path such
// as /uncli/, and its pages show <uncli-chat src="/uncli">. Nothing is
// shared with other hosts or an installed UNCLI: everything this UNCLI
// keeps, the CLIs' own sign-in and settings included, lives in the host's
// data folder.
//
//	core, err := uncli.Start(uncli.Options{DataDir: dir, Origins: []string{"https://bruv.example"}, Name: "BRUV"})
//	if err != nil { ... }
//	defer core.Close()
//	mux.Handle("/uncli/", http.StripPrefix("/uncli", requireSignIn(core.Handler())))
//
// Who reaches the mount is the host's business: anyone it lets in can make
// the CLIs run tools as the host's user, on its machine. Approvals, prompt
// levels and the safe list apply as they do in UNCLI.
package uncli

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/datadir"
	"github.com/harvey-withington/uncli/app/internal/hook"
	"github.com/harvey-withington/uncli/app/internal/remote"
)

// Options are how a host runs its UNCLI.
type Options struct {
	// DataDir holds everything this UNCLI keeps: its database, settings,
	// downloaded CLIs, session folders and the CLIs' own config. Required.
	DataDir string
	// Origins are the host's page origins (https://bruv.example). A request
	// a page sends from any other origin is refused.
	Origins []string
	// AllowOrigin admits origins the list can't name, for a server reached
	// by several names: true lets the page at origin, sending r, in, as if
	// it were listed. Say no to anything you don't recognise.
	AllowOrigin func(origin string, r *http.Request) bool
	// Name is the host's name, recorded on the approvals its pages answer.
	Name string
	// Key, when set, is what KeyedHandler requires (Authorization: Bearer
	// <key>); Handler adds it itself. Empty: a random one.
	Key string
	// Hooks says the host's main calls RunHook first thing, so its program
	// can answer the hooks of CLIs that ask UNCLI about tool calls that way
	// (Antigravity, decision 0012). Without it, those CLIs' sessions don't
	// start.
	Hooks bool
	// UserCLIConfig lets the CLIs use the user's own config (sign-in,
	// skills, MCP servers, memory) instead of one in DataDir. It's shared
	// with everything else on the machine that uses those CLIs.
	UserCLIConfig bool
	// AllowServiceAccount lets UNCLI run as one of Windows' service accounts
	// (LocalSystem and the like), which it refuses otherwise: the CLIs would
	// run tools as that account, and LocalSystem can do anything on the
	// machine. Run the host as a person's account instead (a service's
	// "Log on as") unless you really mean it.
	AllowServiceAccount bool
	// MCPServers gives a session the host's own MCP servers, by name: the
	// tools that work on the host's things (BRUV's board tools, say). It's
	// asked each time a session's CLI starts, with the session's id (as
	// newSession returned it), so a host can give each session the servers
	// for what it's about, with a current token. Their tool calls are
	// approved as any MCP server's are: by their annotations, the prompt
	// level and the safe list. nil: none.
	MCPServers func(session string) map[string]MCPServer
	// HostToolPolicy, with MCPServers: the host decides approvals for its
	// own servers' tools (see HostToolPolicy). nil: UNCLI decides, as for
	// any MCP server.
	HostToolPolicy HostToolPolicy
}

// HostToolPolicy decides approvals for the tools of the host's own MCP
// servers (those MCPServers gave the session), asked at each call, so a
// mode switch on the host's side (BRUV's Chat / Suggest / Edit) takes
// effect at once. Its decision: PolicyAllow runs the tool without asking;
// PolicyDeny refuses it, and reason is told to the model; PolicyAsk asks
// the user in the chat even where UNCLI wouldn't; "" leaves it to UNCLI's
// rules (annotations, the prompt level, the safe list). Tools of any other
// server, and the CLI's own tools, are never asked about: a host can't
// loosen those. The trace says the host decided, with its reason.
type HostToolPolicy func(session, server, tool string) (decision, reason string)

// HostToolPolicy's decisions.
const (
	PolicyAllow = "allow"
	PolicyDeny  = "deny"
	PolicyAsk   = "ask"
)

// hostTools keeps which of the host's servers each session was started
// with, so only those servers' tools go to the host's policy.
type hostTools struct {
	o       Options
	mu      sync.Mutex
	servers map[string]map[string]bool
}

func (h *hostTools) mcp(session string) []string {
	servers := h.o.MCPServers(session)
	names := map[string]bool{}
	for name := range servers {
		names[name] = true
	}
	h.mu.Lock()
	h.servers[session] = names
	h.mu.Unlock()
	return mcpConfig(servers)
}

func (h *hostTools) decide(session, server, tool string) (string, string) {
	h.mu.Lock()
	own := h.servers[session][server]
	h.mu.Unlock()
	if !own {
		return "", ""
	}
	d, reason := h.o.HostToolPolicy(session, server, tool)
	switch d {
	case PolicyAllow, PolicyDeny, PolicyAsk:
		return d, reason
	}
	return "", ""
}

// MCPServer is an MCP server of the host's, reached over HTTP.
type MCPServer struct {
	URL     string            // its MCP endpoint, http(s)://…
	Headers map[string]string // sent with each request: the host's own token, say
}

// hostMCP turns the host's servers for a session into the CLI's
// --mcp-config.
func (o Options) hostMCP(session string) []string { return mcpConfig(o.MCPServers(session)) }

func mcpConfig(servers map[string]MCPServer) []string {
	if len(servers) == 0 {
		return nil
	}
	cfg := map[string]any{}
	for name, s := range servers {
		server := map[string]any{"type": "http", "url": s.URL}
		// No headers, no key: the CLI drops a server whose headers are null.
		if len(s.Headers) > 0 {
			server["headers"] = s.Headers
		}
		cfg[name] = server
	}
	b, err := json.Marshal(map[string]any{"mcpServers": cfg})
	if err != nil {
		return nil
	}
	return []string{string(b)}
}

// ErrServiceAccount: Start refused because the host runs as a Windows
// service account (AllowServiceAccount). errors.Is finds it; errors.As
// gives a *ServiceAccountError, with the account's name, for the host's
// own message.
var ErrServiceAccount = errors.New("uncli: this program runs as a Windows service account")

// ServiceAccountError is ErrServiceAccount with the account named.
type ServiceAccountError struct {
	Account string // LocalSystem, LocalService or NetworkService
}

func (e *ServiceAccountError) Error() string {
	return fmt.Sprintf("%s (%s), so the CLIs would run tools as %s. Run it as a person's account (the service's Log on as), or set AllowServiceAccount", ErrServiceAccount, e.Account, e.Account)
}

func (e *ServiceAccountError) Is(target error) bool { return target == ErrServiceAccount }

// runningAs is serviceAccount, replaced in tests.
var runningAs = serviceAccount

// asAPerson refuses a service account unless the host allows it.
func asAPerson(o Options) error {
	if a := runningAs(); a != "" && !o.AllowServiceAccount {
		return &ServiceAccountError{Account: a}
	}
	return nil
}

// Core is a running UNCLI of the host's own.
type Core struct {
	svc     *app.Service
	server  *remote.Server
	key     string
	routes  http.Handler
	release func() // frees the data folder
	closed  sync.Once
}

// HookArg is the argument the host's program is started with when a CLI
// asks UNCLI about a tool call (Options.Hooks).
const HookArg = "uncli-hook"

// RunHook answers a CLI's hook and exits, when the program was started
// for one; otherwise it returns at once. With Options.Hooks, call it first
// thing in main.
func RunHook() {
	if len(os.Args) == 3 && os.Args[1] == HookArg {
		os.Exit(hook.Run(os.Args[2], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
	}
}

// Start runs the host's UNCLI. Close it when the host stops.
func Start(o Options) (*Core, error) {
	if o.DataDir == "" {
		return nil, errors.New("uncli: Options.DataDir is required")
	}
	dir, err := filepath.Abs(o.DataDir)
	if err != nil {
		return nil, err
	}
	if err := asAPerson(o); err != nil {
		return nil, err
	}
	if err := notTheDesktopApps(dir); err != nil {
		return nil, err
	}
	name := o.Name
	if name == "" {
		name = "Host"
	}
	// One UNCLI at a time on the folder, and only the host it was made for.
	release, err := datadir.Claim(dir, datadir.Owner{Kind: datadir.KindHost, Name: name})
	if err != nil {
		return nil, err
	}
	started := false
	defer func() {
		if !started {
			release()
		}
	}()
	// The CLIs keep their sign-in and settings in the host's data, not the
	// user's home: told to the CLIs UNCLI starts, not set in the host's
	// environment, so programs the host starts itself keep the user's.
	cliConfig := ""
	if !o.UserCLIConfig {
		cliConfig = filepath.Join(dir, "cli-config", "claude")
	}
	key := o.Key
	if key == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		key = hex.EncodeToString(b)
	}
	hookCmd := ""
	if o.Hooks {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		hookCmd = `"` + exe + `" ` + HookArg
	}
	hub := remote.NewHub()
	paths := app.Paths{Config: dir, Cache: filepath.Join(dir, "cache"), Scratch: filepath.Join(dir, "scratch")}
	host := &hostTools{o: o, servers: map[string]map[string]bool{}}
	var hostMCP func(string) []string
	if o.MCPServers != nil {
		hostMCP = host.mcp
	}
	var hostTool func(string, string, string) (string, string)
	if o.MCPServers != nil && o.HostToolPolicy != nil {
		hostTool = host.decide
	}
	svc, err := app.NewWith(paths, hub, app.Options{Embedded: true, HookCommand: hookCmd, CLIConfigDir: cliConfig, HostMCP: hostMCP, HostTool: hostTool})
	if err != nil {
		return nil, err
	}
	server := remote.New(remote.Config{API: app.NewAPI(svc, nil), Service: svc, Hub: hub,
		Auth: keyAuth{key: key, device: remote.Device{ID: "host", Name: name}}, Methods: remote.EmbeddedMethods()})
	started = true
	return &Core{svc: svc, server: server, key: key, routes: server.EmbeddedHandler(o.allowed), release: release}, nil
}

// allowed: a listed origin, or one AllowOrigin admits.
func (o Options) allowed(origin string, r *http.Request) bool {
	for _, a := range o.Origins {
		if strings.EqualFold(strings.TrimSuffix(a, "/"), origin) {
			return true
		}
	}
	return o.AllowOrigin != nil && o.AllowOrigin(origin, r)
}

// notTheDesktopApps refuses UNCLI's own folders: a host's UNCLI shares
// nothing with the user's (decision 0027).
func notTheDesktopApps(dir string) error {
	var mine []string
	if cfg, err := os.UserConfigDir(); err == nil {
		mine = append(mine, filepath.Join(cfg, "uncli"))
	}
	if cache, err := os.UserCacheDir(); err == nil {
		mine = append(mine, filepath.Join(cache, "uncli"))
	}
	if p, err := app.DefaultPaths(); err == nil { // UNCLI_DATA_DIR, when set
		mine = append(mine, p.Config)
	}
	for _, m := range mine {
		if datadir.Inside(dir, m) {
			return fmt.Errorf("%w: %s is UNCLI's own (%s). Give this app a folder of its own", datadir.ErrNotYours, dir, m)
		}
	}
	return nil
}

// Handler is UNCLI's routes, for mounting on the host's own server behind
// its sign-in. It adds the key itself, so the host's pages send none.
func (c *Core) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+c.key)
		c.routes.ServeHTTP(w, r)
	})
}

// KeyedHandler is UNCLI's routes requiring the key: for serving where
// others could reach them (a port on loopback, uncli-core), with the host's
// proxy adding the key.
func (c *Core) KeyedHandler() http.Handler { return c.routes }

// SessionMeta is what the host's page said a session is for when it made
// it (<uncli-chat>'s newSession(…, { meta })), so MCPServers can pick a
// session's servers from its very first start, before the host has
// recorded anything of its own. Kept while this UNCLI runs; empty for a
// session made without it, or after a restart (by then the host has its
// own record).
func (c *Core) SessionMeta(session string) string { return c.svc.Sessions.Meta(session) }

// Close stops the sessions and the CLIs they run, and frees the folder.
// Closing again does nothing.
func (c *Core) Close() {
	c.closed.Do(func() {
		c.server.Close()
		c.svc.Close()
		c.release()
	})
}

// keyAuth accepts only the host's key, as the host.
type keyAuth struct {
	key    string
	device remote.Device
}

func (k keyAuth) Authenticate(token string) (remote.Device, bool) {
	return k.device, subtle.ConstantTimeCompare([]byte(token), []byte(k.key)) == 1
}
