package app

import (
	"context"
	"time"

	"uncli/internal/artifacts"
	"uncli/internal/attach"
	"uncli/internal/clipfiles"
	"uncli/internal/core"
	"uncli/internal/ide"
	"uncli/internal/session"
	"uncli/internal/store"
	filewatch "uncli/internal/watch"
)

// API is everything the UI calls, as methods with plain arguments and
// results, so a transport can expose it as it is: the Wails bridge binds
// it for the window, and the remote server dispatches a listed part of it
// over HTTP (decision 0021). Methods that need the window itself (opening
// a link in the browser, the folder dialog, the clipboard) stay in the
// bridge.
type API struct {
	svc *Service
	// ctx is the context long calls run under: the window's while there is
	// one, otherwise the service's own life.
	ctx func() context.Context
}

// NewAPI wraps the service; ctx may be nil (the service's life is used).
func NewAPI(svc *Service, ctx func() context.Context) *API {
	a := &API{svc: svc, ctx: ctx}
	if a.ctx == nil {
		a.ctx = func() context.Context { return svc.life }
	}
	return a
}

func (a *API) Bootstrap() Bootstrap { return a.svc.Bootstrap() }

func (a *API) CLIStatus(fresh bool) CLIStatus { return a.svc.CLIStatus(a.ctx(), fresh) }

func (a *API) InstallCLI() (CLIStatus, error) { return a.svc.InstallCLI(a.ctx()) }

func (a *API) SubmitLoginCode(code string) (CLIStatus, error) {
	return a.svc.SubmitLoginCode(a.ctx(), code)
}

func (a *API) CancelSignIn() { a.svc.CancelSignIn() }

func (a *API) SetCLIVersion(version string) (CLIStatus, error) {
	return a.svc.SetCLIVersion(a.ctx(), version)
}

func (a *API) CLIChannels() (map[string]string, error) { return a.svc.CLIChannels(a.ctx()) }

// Any provider's CLI: status (with the models its account can use, for
// CLIs that list them), install, version and channels.
func (a *API) ProviderStatus(id string, fresh bool) CLIStatus {
	return a.svc.ProviderStatus(a.ctx(), id, fresh)
}

func (a *API) InstallProvider(id string) (CLIStatus, error) {
	return a.svc.InstallProvider(a.ctx(), id)
}

func (a *API) SetProviderVersion(id, version string) (CLIStatus, error) {
	return a.svc.SetProviderVersion(a.ctx(), id, version)
}

func (a *API) ProviderChannels(id string) (map[string]string, error) {
	return a.svc.ProviderChannels(a.ctx(), id)
}

// SignInTerminal opens a provider's CLI in a terminal window to sign in
// (for CLIs that sign in only there); UNCLI notices when it has.
func (a *API) SignInTerminal(id string) error { return a.svc.SignInTerminal(a.ctx(), id) }

func (a *API) CancelDeviceSignIn(id string) { a.svc.CancelDeviceSignIn(id) }

// RevealCLI shows a provider's CLI binary in its folder.
func (a *API) RevealCLI(id string) error { return a.svc.RevealCLI(a.ctx(), id) }

// EnablePlugin lets a provider plugin run, as its manifest is now (hash).
func (a *API) EnablePlugin(id, hash string) error { return a.svc.EnablePlugin(id, hash) }

func (a *API) DisablePlugin(id string) error { return a.svc.DisablePlugin(id) }

// CreateSession creates a session; the choices made are remembered for the
// next new-session dialog and returned with it.
func (a *API) CreateSession(profileID, workdir, model, container, provider string, shares []string) (CreatedSession, error) {
	v, last, err := a.svc.CreateSessionWith(session.NewSession{Profile: profileID, Workdir: workdir, Model: model, Container: container, Provider: provider, Shares: shares})
	return CreatedSession{Session: v, LastNew: last}, err
}

func (a *API) Pages(sessionID string) ([]store.Page, error) { return a.svc.Sessions.Pages(sessionID) }

// Send starts a turn with the text and any attached files.
func (a *API) Send(sessionID, text string, attachments []AttachmentRef) error {
	return a.svc.Send(context.Background(), sessionID, text, attachments)
}

func (a *API) Interrupt(sessionID string) error { return a.svc.Sessions.Interrupt(sessionID) }

// Restart stops a session's CLI and starts it again on its conversation:
// for a frozen CLI, or to pick up a container's changed mounts.
func (a *API) Restart(sessionID string) (session.View, error) {
	return a.svc.Sessions.Restart(context.Background(), sessionID)
}

// AnswerApproval gives the user's decision on a waiting tool use: allow
// (once), safe (and remember it as safe, in scope "project" or "all") or deny.
func (a *API) AnswerApproval(sessionID, requestID, decision, scope string) error {
	return a.svc.Sessions.Answer(sessionID, requestID, decision, scope)
}

// SetUnattended turns a session's unattended mode on or off: anything that
// would prompt is declined, with a note to the model.
func (a *API) SetUnattended(sessionID string, on bool) (session.View, error) {
	return a.svc.Sessions.SetUnattended(sessionID, on)
}

// SetMode sets when a session prompts: always, unsafe or never.
func (a *API) SetMode(sessionID, mode string) (session.View, error) {
	return a.svc.Sessions.SetMode(sessionID, mode)
}

// SafeList lists the safe-list entries for a session's project and for all projects.
func (a *API) SafeList(sessionID string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.SafeList(sessionID)
}

// SetSafeEntry adds or changes a safe-list entry, in scope "project" or "all".
func (a *API) SetSafeEntry(sessionID string, e store.SafeEntry, scope string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.SetSafeEntry(sessionID, e, scope)
}

// DeleteSafeEntry removes a safe-list entry.
func (a *API) DeleteSafeEntry(sessionID string, e store.SafeEntry) ([]store.SafeEntry, error) {
	return a.svc.Sessions.DeleteSafeEntry(sessionID, e)
}

// SetSafeLabel gives a safe-list entry the user's own name for it.
func (a *API) SetSafeLabel(sessionID string, e store.SafeEntry, label string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.SetSafeLabel(sessionID, e, label)
}

// MoveSafeEntry gives a safe-list entry another scope: "project" or "all".
func (a *API) MoveSafeEntry(sessionID string, e store.SafeEntry, scope string) ([]store.SafeEntry, error) {
	return a.svc.Sessions.MoveSafeEntry(sessionID, e, scope)
}

// Teach remembers command classes or tools with a verdict, in scope
// ("This should prompt" teaches unsafe).
func (a *API) Teach(sessionID string, classes []store.SafeClass, verdict, scope string) error {
	return a.svc.Sessions.Teach(sessionID, classes, verdict, scope)
}

// PreviewClasses says what each part of an example command would be remembered as.
func (a *API) PreviewClasses(sessionID, command string) ([]session.ClassPreview, error) {
	return a.svc.Sessions.PreviewClasses(sessionID, command)
}

// KnownTools lists a session's MCP tools, for adding to the safe list.
func (a *API) KnownTools(sessionID string) ([]string, error) {
	return a.svc.Sessions.KnownTools(sessionID)
}

// ExplainCommand says what a session would do with a command now (runs,
// prompts or blocked) and why for each part, without running it.
func (a *API) ExplainCommand(sessionID, command string) (session.Explanation, error) {
	return a.svc.Sessions.ExplainCommand(sessionID, command)
}

// SessionAllowlist is the list of what a session's type counts as safe, as written.
func (a *API) SessionAllowlist(sessionID string) ([]string, error) {
	return a.svc.Sessions.SessionAllowlist(sessionID)
}

// WatchFiles lists the text files in a session's folder whose path contains
// query, most recently changed first (the File tab's switcher).
func (a *API) WatchFiles(sessionID, query string) ([]filewatch.Entry, error) {
	return a.svc.WatchFiles(sessionID, query)
}

// WatchRead reads a file for the File tab; nil when unchanged since modified.
func (a *API) WatchRead(sessionID, path string, since int64) (*filewatch.File, error) {
	return a.svc.WatchRead(sessionID, path, since)
}

// Commands lists the slash commands a session's CLI takes, for the menu
// the message box opens on "/".
func (a *API) Commands(sessionID string) ([]core.CommandInfo, error) {
	return a.svc.Sessions.Commands(sessionID)
}

// MCPSignIn starts signing in to one of a session's MCP servers; the UI
// opens the link it returns (decision 0020).
func (a *API) MCPSignIn(sessionID, server string) (session.MCPSignIn, error) {
	return a.svc.Sessions.MCPSignIn(a.ctx(), sessionID, server)
}

// MCPCallback hands on the address the browser ended up on, pasted by the user.
func (a *API) MCPCallback(sessionID, server, url string) error {
	return a.svc.Sessions.MCPCallback(sessionID, server, url)
}

func (a *API) MCPReconnect(sessionID, server string) error {
	return a.svc.Sessions.MCPReconnect(a.ctx(), sessionID, server)
}

func (a *API) MCPSignOut(sessionID, server string) error {
	return a.svc.Sessions.MCPSignOut(sessionID, server)
}

func (a *API) MCPCancel(sessionID, server string) error {
	return a.svc.Sessions.MCPCancel(sessionID, server)
}

// AnswerAsk answers a question the model asked (allow, with answers by
// question) or a plan (allow; or deny with a note on what to change).
func (a *API) AnswerAsk(sessionID, requestID string, allow bool, answers map[string]string, note string) error {
	return a.svc.Sessions.AnswerAsk(sessionID, requestID, allow, answers, note)
}

// SetShares changes what a session shares of the user's own (their MCP
// servers and connectors, their skills), from its next turn.
func (a *API) SetShares(sessionID string, shares []string) (session.View, error) {
	return a.svc.Sessions.SetShares(sessionID, shares)
}

func (a *API) SetModel(sessionID, model string) error {
	return a.svc.Sessions.SetModel(sessionID, model)
}

func (a *API) ToggleModifier(sessionID, modifierID string, on bool) (session.View, error) {
	return a.svc.Sessions.ToggleModifier(sessionID, modifierID, on)
}

func (a *API) SetSortOrder(sessionID string, order float64) (session.View, error) {
	return a.svc.Sessions.SetSortOrder(sessionID, order)
}

// DescribePaths says which paths dropped onto the window are folders.
func (a *API) DescribePaths(paths []string) []DroppedPath { return DescribePaths(paths) }

// DescribeAttachments says which dropped or pasted files can be attached.
func (a *API) DescribeAttachments(paths []string) []attach.Info {
	return DescribeAttachments(paths)
}

func (a *API) Rename(sessionID, title string) (session.View, error) {
	return a.svc.Sessions.Rename(sessionID, title)
}

func (a *API) Delete(sessionID string) error { return a.svc.Sessions.Delete(sessionID) }

func (a *API) SetBookmark(sessionID, pageID string, on bool) (store.Page, error) {
	return a.svc.Sessions.SetBookmark(sessionID, pageID, on)
}

// SetPinned pins or unpins a page.
func (a *API) SetPinned(sessionID, pageID string, on bool) (store.Page, error) {
	return a.svc.Sessions.SetPinned(sessionID, pageID, on)
}

// Pinned lists the pinned pages of every session.
func (a *API) Pinned() ([]store.PinnedPage, error) { return a.svc.Sessions.Pinned() }

// Archive archives a session (on) or restores it.
func (a *API) Archive(sessionID string, on bool) (session.View, error) {
	return a.svc.Sessions.Archive(sessionID, on)
}

func (a *API) SetPreferences(p Preferences) (Preferences, error) {
	return a.svc.SetPreferences(p)
}

// MarkSafe toggles "This is safe" on a waiting approval card.
func (a *API) MarkSafe(sessionID, requestID string, on bool, scope string) error {
	return a.svc.Sessions.MarkSafe(sessionID, requestID, on, scope)
}

// SetDecisionKey saves the decision model's API key; HasDecisionKey says
// whether one is saved (the key itself never comes back).
func (a *API) SetDecisionKey(key string) error { return a.svc.SetDecisionKey(key) }

func (a *API) HasDecisionKey() bool { return a.svc.HasDecisionKey() }

// TestDecisionModel asks the decision model about a sample command.
func (a *API) TestDecisionModel() (DecisionTest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return a.svc.TestDecisionModel(ctx)
}

// Search finds pages across all sessions (full-text, with filters).
func (a *API) Search(q store.SearchQuery) (store.SearchResult, error) { return a.svc.Store.Search(q) }

// SummarisePage asks the quick-task model for a table of contents of a
// page; blocks are the answer's blocks as the UI numbers them.
func (a *API) SummarisePage(sessionID, pageID string, blocks []string) (store.Page, error) {
	return a.svc.SummarisePage(a.ctx(), sessionID, pageID, blocks)
}

func (a *API) Focus(sessionID string) { a.svc.Sessions.Focus(sessionID) }

// TestNotification shows a sample desktop notification.
func (a *API) TestNotification() error { return a.svc.TestNotification() }

// Containers reports WSL, the containers' sign-in and each container
// profile (decision 0011); changes arrive as containers:changed.
func (a *API) Containers() ContainersInfo { return a.svc.Containers(a.ctx()) }

// InstallWSL turns WSL on; Windows asks for administrator rights once and
// needs a restart afterwards.
func (a *API) InstallWSL() error { return a.svc.InstallWSL(a.ctx()) }

// StopContainers stops UNCLI's own containers (never all of WSL).
func (a *API) StopContainers() error { return a.svc.StopContainers(a.ctx()) }

// BuildContainer builds or rebuilds a container profile's distro.
func (a *API) BuildContainer(id string) error { return a.svc.BuildContainer(id) }

// SaveContainer adds or replaces a container in the user's containers.yaml;
// RemoveContainerConfig takes the user's version out (a built-in one goes
// back to how it ships; one of their own is deleted with its container).
func (a *API) SaveContainer(p ContainerProfile) error { return a.svc.SaveContainer(p) }

func (a *API) RemoveContainerConfig(id string) error {
	return a.svc.RemoveContainerConfig(a.ctx(), id)
}

// RemoveContainer deletes a container profile's distro.
func (a *API) RemoveContainer(id string) error { return a.svc.RemoveContainer(a.ctx(), id) }

// StartContainerSignIn returns the link to approve; FinishContainerSignIn
// takes the code the page shows and keeps the token in the credential store.
func (a *API) StartContainerSignIn() (string, error) { return a.svc.StartContainerSignIn(a.ctx()) }

func (a *API) FinishContainerSignIn(code string) error {
	return a.svc.FinishContainerSignIn(code)
}

func (a *API) CancelContainerSignIn() { a.svc.CancelContainerSignIn() }

func (a *API) SignOutContainers() error { return a.svc.SignOutContainers() }

// The full account sign-in for containers that share connectors.
func (a *API) StartContainerAccountSignIn() (string, error) {
	return a.svc.StartContainerAccountSignIn(a.ctx())
}

func (a *API) FinishContainerAccountSignIn(code string) error {
	return a.svc.FinishContainerAccountSignIn(code)
}

func (a *API) CancelContainerAccountSignIn() { a.svc.CancelContainerAccountSignIn() }

func (a *API) SignOutContainerAccount() error { return a.svc.SignOutContainerAccount() }

func (a *API) Usage() *core.UsageLimit { return a.svc.Sessions.Usage() }

// Theme reads the user's theme.yaml (decision 0010).
func (a *API) Theme() (ThemeFile, error) { return a.svc.Theme() }

// UsageReport sums the pages' tokens and cost for the usage dashboard.
func (a *API) UsageReport(q store.UsageQuery) (store.UsageReport, error) { return a.svc.Store.Usage(q) }

func (a *API) OpenFolder(path string) error { return OpenFolder(path) }

// OpenFile opens a file from a page: in the editor at line (sessions that
// link to an IDE) or with the default app (documents only).
func (a *API) OpenFile(sessionID, path string, line int) error {
	return a.svc.OpenFile(sessionID, path, line)
}

// OpenPath follows a link in an answer to a file (opened as OpenFile does)
// or a folder (shown).
func (a *API) OpenPath(sessionID, path string, line int) error {
	return a.svc.OpenPath(sessionID, path, line)
}

// RevealFile shows a file in its folder.
func (a *API) RevealFile(path string) error { return a.svc.RevealFile(path) }

// ArtifactFiles lists what is in a session's artifacts folder now.
func (a *API) ArtifactFiles(sessionID string) ([]artifacts.File, error) {
	return a.svc.ArtifactFiles(sessionID)
}

// ReadArtifact returns an artifact's content (a version by hash, or the
// file in the folder by path), base64.
func (a *API) ReadArtifact(sessionID string, ref ArtifactRef) (ArtifactContent, error) {
	return a.svc.ReadArtifact(sessionID, ref)
}

// ArtifactPath is where an artifact is on disk now.
func (a *API) ArtifactPath(sessionID, path string) (string, error) {
	return a.svc.ArtifactPath(sessionID, path)
}

// Transcripts lists the conversations the CLI has saved, for importing.
func (a *API) Transcripts() ([]TranscriptEntry, error) { return a.svc.Transcripts() }

// ImportTranscript imports a saved conversation as a session of a type (empty: guessed).
func (a *API) ImportTranscript(id, profileID string) (session.ImportedSession, error) {
	return a.svc.ImportTranscript(id, profileID)
}

// Editors lists the editors UNCLI knows and which are installed.
func (a *API) Editors() []ide.Editor { return a.svc.Editors() }

// ClipboardFiles lists the files on the OS clipboard (copied in Explorer),
// so pasting them can insert their paths; empty when there are none.
func (a *API) ClipboardFiles() ([]string, error) {
	p, err := clipfiles.Paths()
	if p == nil {
		p = []string{}
	}
	return p, err
}
