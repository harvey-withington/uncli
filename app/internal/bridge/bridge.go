// Package bridge is the only package that imports Wails. It exposes the
// app service as bound methods and forwards its events to the frontend.
// Replacing it (with an HTTP server, say) is how UNCLI would run elsewhere.
package bridge

import (
	"context"
	"io/fs"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"uncli/internal/app"
	"uncli/internal/core"
	"uncli/internal/session"
	"uncli/internal/store"
)

// App is bound to the frontend as window.go.bridge.App.
type App struct {
	ctx context.Context
	svc *app.Service

	mu      sync.Mutex
	pending []func(context.Context) // events emitted before the window exists
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

// Run starts the desktop app with the embedded frontend.
func Run(assets fs.FS) error {
	paths, err := app.DefaultPaths()
	if err != nil {
		return err
	}
	a := &App{}
	svc, err := app.New(paths, a)
	if err != nil {
		return err
	}
	a.svc = svc
	defer svc.Close()

	return wails.Run(&options.App{
		Title:            "UNCLI",
		Width:            1280,
		Height:           820,
		MinWidth:         880,
		MinHeight:        560,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 249, A: 255},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "app.uncli.desktop",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if a.ctx != nil {
					wruntime.WindowUnminimise(a.ctx)
					wruntime.Show(a.ctx)
				}
			},
		},
		Windows: &windows.Options{Theme: windows.SystemDefault},
		// OS file drops give the page real paths (window.runtime.OnFileDrop);
		// the WebView's own drop handling is off so a dropped file can't
		// navigate the window.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		OnStartup: func(ctx context.Context) {
			a.mu.Lock()
			a.ctx = ctx
			pending := a.pending
			a.pending = nil
			a.mu.Unlock()
			for _, f := range pending {
				f(ctx)
			}
		},
		OnShutdown: func(context.Context) { svc.Sessions.Close() },
		Bind:       []any{a},
	})
}

func (a *App) Bootstrap() app.Bootstrap { return a.svc.Bootstrap() }

func (a *App) CLIStatus(fresh bool) app.CLIStatus { return a.svc.CLIStatus(a.ctx, fresh) }

func (a *App) InstallCLI() (app.CLIStatus, error) { return a.svc.InstallCLI(a.ctx) }

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

func (a *App) SubmitLoginCode(code string) (app.CLIStatus, error) {
	return a.svc.SubmitLoginCode(a.ctx, code)
}

func (a *App) CancelSignIn() { a.svc.CancelSignIn() }

func (a *App) SetCLIVersion(version string) (app.CLIStatus, error) {
	return a.svc.SetCLIVersion(a.ctx, version)
}

func (a *App) CLIChannels() (map[string]string, error) { return a.svc.CLIChannels(a.ctx) }

func (a *App) PickFolder(title string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
}

// CreateSession creates a session; the choices made are remembered for the
// next new-session dialog and returned with it.
func (a *App) CreateSession(profileID, workdir, model string) (app.CreatedSession, error) {
	v, last, err := a.svc.CreateSession(profileID, workdir, model)
	return app.CreatedSession{Session: v, LastNew: last}, err
}

func (a *App) Pages(sessionID string) ([]store.Page, error) { return a.svc.Sessions.Pages(sessionID) }

func (a *App) Send(sessionID, text string) error {
	return a.svc.Sessions.Send(context.Background(), sessionID, text)
}

func (a *App) Interrupt(sessionID string) error { return a.svc.Sessions.Interrupt(sessionID) }

func (a *App) SetModel(sessionID, model string) error {
	return a.svc.Sessions.SetModel(sessionID, model)
}

func (a *App) ToggleModifier(sessionID, modifierID string, on bool) (session.View, error) {
	return a.svc.Sessions.ToggleModifier(sessionID, modifierID, on)
}

func (a *App) SetSortOrder(sessionID string, order float64) (session.View, error) {
	return a.svc.Sessions.SetSortOrder(sessionID, order)
}

// DescribePaths says which paths dropped onto the window are folders.
func (a *App) DescribePaths(paths []string) []app.DroppedPath { return app.DescribePaths(paths) }

func (a *App) Rename(sessionID, title string) (session.View, error) {
	return a.svc.Sessions.Rename(sessionID, title)
}

func (a *App) Delete(sessionID string) error { return a.svc.Sessions.Delete(sessionID) }

func (a *App) SetBookmark(sessionID, pageID string, on bool) (store.Page, error) {
	return a.svc.Sessions.SetBookmark(sessionID, pageID, on)
}

func (a *App) SetPreferences(p app.Preferences) (app.Preferences, error) {
	return a.svc.SetPreferences(p)
}

// SummarisePage asks the quick-task model for a table of contents of a
// page; blocks are the answer's blocks as the UI numbers them.
func (a *App) SummarisePage(sessionID, pageID string, blocks []string) (store.Page, error) {
	return a.svc.SummarisePage(a.ctx, sessionID, pageID, blocks)
}

func (a *App) Focus(sessionID string) { a.svc.Sessions.Focus(sessionID) }

func (a *App) Usage() *core.UsageLimit { return a.svc.Sessions.Usage() }

func (a *App) OpenFolder(path string) error { return app.OpenFolder(path) }

func (a *App) CopyText(text string) error { return wruntime.ClipboardSetText(a.ctx, text) }

// OpenURL opens a link from an answer in the user's browser. Only web
// links are allowed; anything else is ignored.
func (a *App) OpenURL(url string) {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}
