// Package bridge is the only package that imports Wails. It exposes the
// app service as bound methods and forwards its events to the frontend.
// Replacing it (with an HTTP server, say) is how UNCLI would run elsewhere.
package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"uncli/internal/app"
	"uncli/internal/notify"
	"uncli/internal/push"
	"uncli/internal/remote"
)

// App is bound to the frontend as window.go.bridge.App: the app's API
// (internal/app), plus the calls that need the window.
type App struct {
	*app.API
	ctx    context.Context
	svc    *app.Service
	remote *remote.Server

	mu       sync.Mutex
	pending  []func(context.Context) // events emitted before the window exists
	quitting atomic.Bool             // Quit was chosen: closing the window ends the app
}

func (a *App) Emit(name string, data any) {
	a.mu.Lock()
	ctx := a.ctx
	if ctx == nil {
		a.pending = append(a.pending, func(c context.Context) { wruntime.EventsEmit(c, name, data) })
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	wruntime.EventsEmit(ctx, name, data)
}

// context is the Wails context, nil until the window exists.
func (a *App) context() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx
}

// raise brings the window to the front as it was: maximized stays
// maximized. Unminimise restores a window that isn't minimized to its
// normal size, so it only runs on a minimized one, which comes back
// maximized if it was.
func raise(ctx context.Context) {
	if wruntime.WindowIsMinimised(ctx) {
		wruntime.WindowUnminimise(ctx)
	}
	wruntime.Show(ctx)
}

// Options are how the app was started.
type Options struct {
	Minimised bool // straight to the tray, no window (started with Windows)
}

// Run starts the desktop app with the embedded frontend. Closing the window
// hides it: UNCLI keeps running in the tray (decision 0021) until the tray
// menu's Quit.
func Run(assets fs.FS, opts Options) error {
	paths, err := app.DefaultPaths()
	if err != nil {
		return err
	}
	a := &App{}
	hub := remote.NewHub()
	svc, err := app.New(paths, both{a, hub})
	if err != nil {
		return err
	}
	a.svc = svc
	a.API = app.NewAPI(svc, func() context.Context {
		if ctx := a.context(); ctx != nil {
			return ctx
		}
		return context.Background()
	})
	svc.ShowWindow = func() {
		if ctx := a.context(); ctx != nil {
			raise(ctx)
		}
	}
	svc.Quit = func() {
		if ctx := a.context(); ctx != nil {
			a.quitting.Store(true)
			wruntime.Quit(ctx)
		}
	}
	// The phone app is the frontend's dist/m, served at /m/.
	pwa, err := fs.Sub(assets, "m")
	if err != nil {
		return err
	}
	// Push notifications for paired devices; without keys, devices are told
	// notifications aren't available.
	pusher, err := push.New(svc.Store)
	if err != nil {
		pusher = nil
	}
	a.remote = remote.New(remote.Config{API: app.NewAPI(svc, nil), Service: svc, Hub: hub, Store: svc.Store, PWA: pwa, Push: pusher,
		Certs: &remote.Tailscale{}, OnStatus: func(st remote.Status) { a.Emit(EvtRemoteStatus, st) },
		OnPairRequest: func(p remote.PairRequest) {
			// Someone is at the server to approve: bring the window up.
			if svc.ShowWindow != nil {
				svc.ShowWindow()
			}
			a.Emit(EvtPairRequest, p)
		},
		OnDevices: func(d []remote.PairedDevice) { a.Emit(EvtDevices, d) }})
	// Every change worth a note goes to the paired devices too, off the
	// session's own goroutine.
	svc.SetRemoteNotes(func(n notify.Note, approval bool) { go a.remote.Notify(n, approval) })
	defer svc.Close()
	defer a.remote.Close()
	go a.remote.Start()

	return wails.Run(&options.App{
		Title:            "UNCLI",
		Width:            1280,
		Height:           820,
		MinWidth:         880,
		MinHeight:        560,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 249, A: 255},
		StartHidden:      opts.Minimised,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID(os.Getenv("UNCLI_DATA_DIR")),
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
				if ctx := a.context(); ctx != nil && !slices.Contains(d.Args, app.MinimisedArg) {
					raise(ctx)
				}
			},
		},
		Windows: &windows.Options{Theme: windows.SystemDefault},
		// OS file drops give the page real paths (window.runtime.OnFileDrop).
		// On Windows that works through the WebView's own drop events, so
		// they stay on; the page cancels file drops itself (lib/drops.ts), so
		// a dropped file can't navigate the window.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		OnStartup: func(ctx context.Context) {
			a.mu.Lock()
			a.ctx = ctx
			pending := a.pending
			a.pending = nil
			a.mu.Unlock()
			for _, f := range pending {
				f(ctx)
			}
			if err := svc.KeepInTray(); err != nil {
				// Without a tray icon there'd be no way back to a hidden
				// window, so closing it quits instead.
				a.quitting.Store(true)
			}
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if a.quitting.Load() {
				return false
			}
			wruntime.WindowHide(ctx)
			return true
		},
		OnShutdown: func(context.Context) { svc.Sessions.Close() },
		Bind:       []any{a},
	})
}

// SignIn starts the CLI's sign-in and opens its link in the user's default
// browser (the CLI can't reliably do that without a terminal). The link is
// returned too, so the UI can offer it again.
func (a *App) SignIn() (app.SignInStart, error) {
	start, err := a.svc.SignIn(a.ctx)
	if err == nil && start.URL != "" {
		wruntime.BrowserOpenURL(a.ctx, start.URL)
	}
	return start, err
}

// StartDeviceSignIn starts a provider's device sign-in and opens its link
// in the browser; the code is returned for the UI to show (it can be
// approved on any device).
func (a *App) StartDeviceSignIn(id string) (app.DeviceSignIn, error) {
	d, err := a.svc.StartDeviceSignIn(a.ctx, id)
	if err == nil && d.URL != "" {
		wruntime.BrowserOpenURL(a.ctx, d.URL)
	}
	return d, err
}

func (a *App) PickFolder(title string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
}

func (a *App) CopyText(text string) error { return wruntime.ClipboardSetText(a.ctx, text) }

// OpenURL opens a link from an answer in the user's browser. Only web
// links are allowed; anything else is ignored.
func (a *App) OpenURL(url string) {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}

// both sends the app's events to the window and to remote clients.
type both struct{ window, hub app.Emitter }

func (b both) Emit(name string, data any) {
	b.window.Emit(name, data)
	b.hub.Emit(name, data)
}

// Remote access's events, for the window only.
const (
	EvtRemoteStatus = "remote:status"  // remote.Status
	EvtPairRequest  = "remote:pair"    // remote.PairRequest: a device waits for Allow
	EvtDevices      = "remote:devices" // []remote.PairedDevice
)

// RemoteStatus is Settings → Remote access's state: off, on (with the
// address), or why not (decision 0021). Only the window can see or change
// it; remote clients can't.
func (a *App) RemoteStatus() remote.Status { return a.remote.Status() }

// SetRemoteAccess turns Remote access on or off, on a port.
func (a *App) SetRemoteAccess(on bool, port int) (remote.Status, error) {
	return a.remote.SetSettings(remote.Settings{Enabled: on, Port: port})
}

// StartPairing shows a pairing code (decision 0022): a QR code and a link
// for the device to open.
func (a *App) StartPairing() (remote.Pairing, error) { return a.remote.StartPairing() }

func (a *App) CancelPairing() { a.remote.CancelPairing() }

// AnswerPairing is "Allow <name>?" answered.
func (a *App) AnswerPairing(id string, allow bool) error { return a.remote.AnswerPairing(id, allow) }

// PairedDevices lists the devices paired with this UNCLI.
func (a *App) PairedDevices() []remote.PairedDevice { return a.remote.Devices() }

// RevokeDevice unpairs a device and closes its connections.
func (a *App) RevokeDevice(id string) error { return a.remote.RevokeDevice(id) }

// instanceID is the single-instance lock's id: one UNCLI per data folder.
// A copy run on a data folder of its own (UNCLI_DATA_DIR: tests, wails
// dev) gets its own lock, so it starts alongside the user's app instead of
// handing over to it and quitting.
func instanceID(dataDir string) string {
	if dataDir == "" {
		return "app.uncli.desktop"
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dataDir))))
	return "app.uncli.desktop." + hex.EncodeToString(sum[:6])
}
