// Package bridge is the only package that imports Wails. It exposes the
// app service as bound methods and forwards its events to the frontend.
// Replacing it (with an HTTP server, say) is how UNCLI would run elsewhere.
package bridge

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"uncli/internal/app"
	"uncli/internal/artifacts"
	"uncli/internal/attach"
	"uncli/internal/clipfiles"
	"uncli/internal/core"
	"uncli/internal/ide"
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

// context is the Wails context, nil until the window exists.
func (a *App) context() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx
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
	svc.ShowWindow = func() {
		if ctx := a.context(); ctx != nil {
			wruntime.WindowUnminimise(ctx)
			wruntime.Show(ctx)
		}
	}
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

// Any provider's CLI: status (with the models its account can use, for
// CLIs that list them), install, version and channels.
func (a *App) ProviderStatus(id string, fresh bool) app.CLIStatus {
	return a.svc.ProviderStatus(a.ctx, id, fresh)
}

func (a *App) InstallProvider(id string) (app.CLIStatus, error) {
	return a.svc.InstallProvider(a.ctx, id)
}

func (a *App) SetProviderVersion(id, version string) (app.CLIStatus, error) {
	return a.svc.SetProviderVersion(a.ctx, id, version)
}

func (a *App) ProviderChannels(id string) (map[string]string, error) {
	return a.svc.ProviderChannels(a.ctx, id)
}

func (a *App) PickFolder(title string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
}

// CreateSession creates a session; the choices made are remembered for the
// next new-session dialog and returned with it.
func (a *App) CreateSession(profileID, workdir, model, container, provider string) (app.CreatedSession, error) {
	v, last, err := a.svc.CreateSessionWith(session.NewSession{Profile: profileID, Workdir: workdir, Model: model, Container: container, Provider: provider})
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

// SetSafeLabel gives a safe-list entry the user's own name for it.
func (a *App) SetSafeLabel(sessionID string, e store.SafeEntry, label string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.SetSafeLabel(sessionID, e, label)
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

// SetPinned pins or unpins a page.
func (a *App) SetPinned(sessionID, pageID string, on bool) (store.Page, error) {
	return a.svc.Sessions.SetPinned(sessionID, pageID, on)
}

// Pinned lists the pinned pages of every session.
func (a *App) Pinned() ([]store.PinnedPage, error) { return a.svc.Sessions.Pinned() }

// Archive archives a session (on) or restores it.
func (a *App) Archive(sessionID string, on bool) (session.View, error) {
	return a.svc.Sessions.Archive(sessionID, on)
}

func (a *App) SetPreferences(p app.Preferences) (app.Preferences, error) {
	return a.svc.SetPreferences(p)
}

// MarkSafe toggles "This is safe" on a waiting approval card.
func (a *App) MarkSafe(sessionID, requestID string, on bool, scope string) error {
	return a.svc.Sessions.MarkSafe(sessionID, requestID, on, scope)
}

// SetDecisionKey saves the decision model's API key; HasDecisionKey says
// whether one is saved (the key itself never comes back).
func (a *App) SetDecisionKey(key string) error { return a.svc.SetDecisionKey(key) }
func (a *App) HasDecisionKey() bool            { return a.svc.HasDecisionKey() }

// TestDecisionModel asks the decision model about a sample command.
func (a *App) TestDecisionModel() (app.DecisionTest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return a.svc.TestDecisionModel(ctx)
}

// SummarisePage asks the quick-task model for a table of contents of a
// page; blocks are the answer's blocks as the UI numbers them.
// Search finds pages across all sessions (full-text, with filters).
func (a *App) Search(q store.SearchQuery) (store.SearchResult, error) { return a.svc.Store.Search(q) }

func (a *App) SummarisePage(sessionID, pageID string, blocks []string) (store.Page, error) {
	return a.svc.SummarisePage(a.ctx, sessionID, pageID, blocks)
}

func (a *App) Focus(sessionID string) { a.svc.Sessions.Focus(sessionID) }

// TestNotification shows a sample desktop notification.
func (a *App) TestNotification() error { return a.svc.TestNotification() }

// Containers reports WSL, the containers' sign-in and each container
// profile (decision 0011); changes arrive as containers:changed.
func (a *App) Containers() app.ContainersInfo { return a.svc.Containers(a.ctx) }

// InstallWSL turns WSL on; Windows asks for administrator rights once and
// needs a restart afterwards.
func (a *App) InstallWSL() error { return a.svc.InstallWSL(a.ctx) }

// StopContainers stops UNCLI's own containers (never all of WSL).
func (a *App) StopContainers() error { return a.svc.StopContainers(a.ctx) }

// BuildContainer builds or rebuilds a container profile's distro.
func (a *App) BuildContainer(id string) error { return a.svc.BuildContainer(id) }

// SaveContainer adds or replaces a container in the user's containers.yaml;
// RemoveContainerConfig takes the user's version out (a built-in one goes
// back to how it ships; one of their own is deleted with its container).
func (a *App) SaveContainer(p app.ContainerProfile) error { return a.svc.SaveContainer(p) }
func (a *App) RemoveContainerConfig(id string) error {
	return a.svc.RemoveContainerConfig(a.ctx, id)
}

// RemoveContainer deletes a container profile's distro.
func (a *App) RemoveContainer(id string) error { return a.svc.RemoveContainer(a.ctx, id) }

// StartContainerSignIn returns the link to approve; FinishContainerSignIn
// takes the code the page shows and keeps the token in the credential store.
func (a *App) StartContainerSignIn() (string, error) { return a.svc.StartContainerSignIn(a.ctx) }
func (a *App) FinishContainerSignIn(code string) error {
	return a.svc.FinishContainerSignIn(code)
}
func (a *App) CancelContainerSignIn()   { a.svc.CancelContainerSignIn() }
func (a *App) SignOutContainers() error { return a.svc.SignOutContainers() }

// The full account sign-in for containers that share connectors.
func (a *App) StartContainerAccountSignIn() (string, error) {
	return a.svc.StartContainerAccountSignIn(a.ctx)
}
func (a *App) FinishContainerAccountSignIn(code string) error {
	return a.svc.FinishContainerAccountSignIn(code)
}
func (a *App) CancelContainerAccountSignIn()  { a.svc.CancelContainerAccountSignIn() }
func (a *App) SignOutContainerAccount() error { return a.svc.SignOutContainerAccount() }

func (a *App) Usage() *core.UsageLimit { return a.svc.Sessions.Usage() }

// Theme reads the user's theme.yaml (decision 0010).
func (a *App) Theme() (app.ThemeFile, error) { return a.svc.Theme() }

// UsageReport sums the pages' tokens and cost for the usage dashboard.
func (a *App) UsageReport(q store.UsageQuery) (store.UsageReport, error) { return a.svc.Store.Usage(q) }

func (a *App) OpenFolder(path string) error { return app.OpenFolder(path) }

// OpenFile opens a file from a page: in the editor at line (sessions that
// link to an IDE) or with the default app (documents only).
func (a *App) OpenFile(sessionID, path string, line int) error {
	return a.svc.OpenFile(sessionID, path, line)
}

// OpenPath follows a link in an answer to a file (opened as OpenFile does)
// or a folder (shown).
func (a *App) OpenPath(sessionID, path string, line int) error {
	return a.svc.OpenPath(sessionID, path, line)
}

// RevealFile shows a file in its folder.
func (a *App) RevealFile(path string) error { return a.svc.RevealFile(path) }

// ArtifactFiles lists what is in a session's artifacts folder now.
func (a *App) ArtifactFiles(sessionID string) ([]artifacts.File, error) {
	return a.svc.ArtifactFiles(sessionID)
}

// ReadArtifact returns an artifact's content (a version by hash, or the
// file in the folder by path), base64.
func (a *App) ReadArtifact(sessionID string, ref app.ArtifactRef) (app.ArtifactContent, error) {
	return a.svc.ReadArtifact(sessionID, ref)
}

// ArtifactPath is where an artifact is on disk now.
func (a *App) ArtifactPath(sessionID, path string) (string, error) {
	return a.svc.ArtifactPath(sessionID, path)
}

// Transcripts lists the conversations the CLI has saved, for importing.
func (a *App) Transcripts() ([]app.TranscriptEntry, error) { return a.svc.Transcripts() }

// ImportTranscript imports a saved conversation as a session of a type (empty: guessed).
func (a *App) ImportTranscript(id, profileID string) (session.ImportedSession, error) {
	return a.svc.ImportTranscript(id, profileID)
}

// Editors lists the editors UNCLI knows and which are installed.
func (a *App) Editors() []ide.Editor { return a.svc.Editors() }

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
