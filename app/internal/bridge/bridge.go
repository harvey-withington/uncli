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
	"uncli/internal/attach"
	"uncli/internal/clipfiles"
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

// Send starts a turn with the text and any attached files.
func (a *App) Send(sessionID, text string, attachments []app.AttachmentRef) error {
	return a.svc.Send(context.Background(), sessionID, text, attachments)
}

func (a *App) Interrupt(sessionID string) error { return a.svc.Sessions.Interrupt(sessionID) }

// AnswerApproval gives the user's decision on a waiting tool use: allow
// (once), safe (and remember it as safe, in scope "project" or "all") or deny.
func (a *App) AnswerApproval(sessionID, requestID, decision, scope string) error {
	return a.svc.Sessions.Answer(sessionID, requestID, decision, scope)
}

// SetUnattended turns a session's unattended mode on or off: anything that
// would prompt is declined, with a note to the model.
func (a *App) SetUnattended(sessionID string, on bool) (session.View, error) {
	return a.svc.Sessions.SetUnattended(sessionID, on)
}

// SetMode sets when a session prompts: always, unsafe or never.
func (a *App) SetMode(sessionID, mode string) (session.View, error) {
	return a.svc.Sessions.SetMode(sessionID, mode)
}

// SafeList lists the safe-list entries for a session's project and for all projects.
func (a *App) SafeList(sessionID string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.SafeList(sessionID)
}

// SetSafeEntry adds or changes a safe-list entry, in scope "project" or "all".
func (a *App) SetSafeEntry(sessionID string, e store.SafeEntry, scope string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.SetSafeEntry(sessionID, e, scope)
}

// DeleteSafeEntry removes a safe-list entry.
func (a *App) DeleteSafeEntry(sessionID string, e store.SafeEntry) ([]store.SafeEntry, error) {
	return a.svc.Sessions.DeleteSafeEntry(sessionID, e)
}

// MoveSafeEntry gives a safe-list entry another scope: "project" or "all".
func (a *App) MoveSafeEntry(sessionID string, e store.SafeEntry, scope string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.MoveSafeEntry(sessionID, e, scope)
}

// Teach remembers command classes or tools with a verdict, in scope
// ("This should prompt" teaches unsafe).
func (a *App) Teach(sessionID string, classes []store.SafeClass, verdict, scope string) error {
	return a.svc.Sessions.Teach(sessionID, classes, verdict, scope)
}

// PreviewClasses says what each part of an example command would be remembered as.
func (a *App) PreviewClasses(sessionID, command string) ([]session.ClassPreview, error) {
	return a.svc.Sessions.PreviewClasses(sessionID, command)
}

// KnownTools lists a session's MCP tools, for adding to the safe list.
func (a *App) KnownTools(sessionID string) ([]string, error) {
	return a.svc.Sessions.KnownTools(sessionID)
}

// ExplainCommand says what a session would do with a command now (runs,
// prompts or blocked) and why for each part, without running it.
func (a *App) ExplainCommand(sessionID, command string) (session.Explanation, error) {
	return a.svc.Sessions.ExplainCommand(sessionID, command)
}

// SessionAllowlist is the list of what a session's type counts as safe, as written.
func (a *App) SessionAllowlist(sessionID string) ([]string, error) {
	return a.svc.Sessions.SessionAllowlist(sessionID)
}

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

// DescribeAttachments says which dropped or pasted files can be attached.
func (a *App) DescribeAttachments(paths []string) []attach.Info {
	return app.DescribeAttachments(paths)
}

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
// Search finds pages across all sessions (full-text, with filters).
func (a *App) Search(q store.SearchQuery) (store.SearchResult, error) { return a.svc.Store.Search(q) }

func (a *App) SummarisePage(sessionID, pageID string, blocks []string) (store.Page, error) {
	return a.svc.SummarisePage(a.ctx, sessionID, pageID, blocks)
}

func (a *App) Focus(sessionID string) { a.svc.Sessions.Focus(sessionID) }

func (a *App) Usage() *core.UsageLimit { return a.svc.Sessions.Usage() }

func (a *App) OpenFolder(path string) error { return app.OpenFolder(path) }

func (a *App) CopyText(text string) error { return wruntime.ClipboardSetText(a.ctx, text) }

// ClipboardFiles lists the files on the OS clipboard (copied in Explorer),
// so pasting them can insert their paths; empty when there are none.
func (a *App) ClipboardFiles() ([]string, error) {
	p, err := clipfiles.Paths()
	if p == nil {
		p = []string{}
	}
	return p, err
}

// OpenURL opens a link from an answer in the user's browser. Only web
// links are allowed; anything else is ignored.
func (a *App) OpenURL(url string) {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}
