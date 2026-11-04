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
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/harvey-withington/uncli/app/internal/app"
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
}

// Core is a running UNCLI of the host's own.
type Core struct {
	svc    *app.Service
	server *remote.Server
	key    string
	routes http.Handler
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
	if !o.UserCLIConfig {
		// The CLIs keep their sign-in and settings in the host's data, not
		// the user's home. Every part of UNCLI that looks for them, and the
		// CLIs it starts, read this variable.
		if err := os.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "cli-config", "claude")); err != nil {
			return nil, err
		}
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
	name := o.Name
	if name == "" {
		name = "Host"
	}
	hub := remote.NewHub()
	paths := app.Paths{Config: dir, Cache: filepath.Join(dir, "cache"), Scratch: filepath.Join(dir, "scratch")}
	svc, err := app.NewWith(paths, hub, app.Options{Embedded: true, HookCommand: hookCmd})
	if err != nil {
		return nil, err
	}
	server := remote.New(remote.Config{API: app.NewAPI(svc, nil), Service: svc, Hub: hub,
		Auth: keyAuth{key: key, device: remote.Device{ID: "host", Name: name}}, Methods: remote.EmbeddedMethods()})
	return &Core{svc: svc, server: server, key: key, routes: server.EmbeddedHandler(o.Origins)}, nil
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

// Close stops the sessions and the CLIs they run.
func (c *Core) Close() {
	c.server.Close()
	c.svc.Close()
}

// keyAuth accepts only the host's key, as the host.
type keyAuth struct {
	key    string
	device remote.Device
}

func (k keyAuth) Authenticate(token string) (remote.Device, bool) {
	return k.device, subtle.ConstantTimeCompare([]byte(token), []byte(k.key)) == 1
}
