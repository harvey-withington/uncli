package remote

import (
	"context"
	"errors"
	"sort"
	"strings"

	"uncli/internal/app"
	"uncli/internal/push"
)

// client is what one paired device's RPC calls run on: the app's API, with
// the calls that need to know the device (approval answers record it) or
// that act differently away from the server's screen. Only the methods
// below are reachable; TestRPCMethods pins the list, so adding one is a
// deliberate change.
type client struct {
	*app.API
	svc    *app.Service
	server *Server
	device Device
}

// rpcMethods is everything a paired device may call. It covers what the
// window does with sessions (decision 0021). Left out, and shown disabled
// on remote clients with the reason: opening things on the server's screen
// (folders, files, editors, links, the clipboard, dropped paths), CLI
// installs, versions and sign-in, plugins, container management, the
// theme file, the decision model's key and test, desktop notification
// tests and the desktop's own preferences. Remote access itself (devices,
// pairing, turning it off) is only changed at the server.
var rpcMethods = []string{
	// Start-up and status.
	"Bootstrap", "CLIStatus", "ProviderStatus", "CLIChannels", "ProviderChannels",
	"Containers", "Editors", "HasDecisionKey", "Usage", "UsageReport",
	// Sessions.
	"CreateSession", "Pages", "Send", "Interrupt", "Restart", "Focus", "Rename", "Delete",
	"Archive", "SetSortOrder", "SetModel", "ToggleModifier", "SetShares",
	"SetMode", "SetUnattended", "Search", "SummarisePage",
	"SetBookmark", "SetPinned", "Pinned", "Commands",
	"Transcripts", "ImportTranscript",
	// Approvals and the safe list: the same rules as at the server.
	"AnswerApproval", "AnswerAsk", "MarkSafe", "SafeList", "SetSafeEntry",
	"DeleteSafeEntry", "SetSafeLabel", "MoveSafeEntry", "Teach",
	"PreviewClasses", "KnownTools", "ExplainCommand", "SessionAllowlist",
	// MCP servers of a session.
	"MCPSignIn", "MCPCallback", "MCPReconnect", "MCPSignOut", "MCPCancel",
	// Files, artifacts and folders on the server, read only.
	"WatchFiles", "WatchRead", "ArtifactFiles", "ReadArtifact", "ListFolder",
	"ArtifactLink", "UploadLink",
	// This device's notifications (push).
	"PushKey", "PushState", "PushSubscribe", "PushUnsubscribe", "SetPushNotifications", "TestPush",
}

var rpcNames = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range rpcMethods {
		m[n] = true
	}
	return m
}()

// RPCMethods lists the methods a paired device may call, sorted.
func RPCMethods() []string {
	out := append([]string(nil), rpcMethods...)
	sort.Strings(out)
	return out
}

// AnswerApproval records which device answered (decision 0022).
func (c *client) AnswerApproval(sessionID, requestID, decision, scope string) error {
	return c.svc.Sessions.AnswerFrom(sessionID, requestID, decision, scope, c.device.Name)
}

func (c *client) AnswerAsk(sessionID, requestID string, allow bool, answers map[string]string, note string) error {
	return c.svc.Sessions.AnswerAskFrom(sessionID, requestID, allow, answers, note, c.device.Name)
}

// Focus is the session the device shows ("" for none): it is read, and a
// turn ending there stays read, without taking the window's focus (the
// window's notifications still follow what's shown there). It's forgotten
// when the device's last event stream closes.
func (c *client) Focus(sessionID string) {
	c.svc.Sessions.Showing(c.device.ID, sessionID)
}

// Send takes attachments uploaded to the server or pasted images, never a
// path on the server chosen by the client.
func (c *client) Send(sessionID, text string, attachments []app.AttachmentRef) error {
	for _, r := range attachments {
		if r.Path != "" {
			return errors.New("attach files by uploading them")
		}
	}
	return c.API.Send(sessionID, text, attachments)
}

// A device browses and starts sessions in folders on the server's own
// drives, never on a network path: reaching one makes Windows connect out
// and offer the user's credentials to whatever answers.
var errNetworkPath = errors.New("choose a folder on this PC's own drives, not a network path")

func networkPath(p string) bool {
	return strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//")
}

func (c *client) ListFolder(path string) (app.FolderListing, error) {
	if networkPath(path) {
		return app.FolderListing{}, errNetworkPath
	}
	return c.API.ListFolder(path)
}

func (c *client) CreateSession(profileID, workdir, model, container, provider string, shares []string) (app.CreatedSession, error) {
	if networkPath(workdir) {
		return app.CreatedSession{}, errNetworkPath
	}
	return c.API.CreateSession(profileID, workdir, model, container, provider, shares)
}

// ArtifactLink is a signed link to an artifact, to open it in the
// device's browser.
func (c *client) ArtifactLink(sessionID string, ref app.ArtifactRef) (string, error) {
	if _, err := c.svc.ReadArtifact(sessionID, ref); err != nil {
		return "", err
	}
	return c.server.links.artifact(c.device.ID, sessionID, ref), nil
}

// Push (decision 0021): each device subscribes its own browser and picks
// what it's told; nothing here touches another device's.

var errNoPush = errors.New("notifications aren't available on this server")

func (c *client) pusher() (*push.Pusher, error) {
	if c.server.push == nil {
		return nil, errNoPush
	}
	return c.server.push, nil
}

// PushKey is the key the device's browser subscribes with.
func (c *client) PushKey() (string, error) {
	p, err := c.pusher()
	if err != nil {
		return "", err
	}
	return p.PublicKey(), nil
}

func (c *client) PushState() (push.State, error) {
	p, err := c.pusher()
	if err != nil {
		return push.State{}, err
	}
	return p.StateOf(c.device.ID), nil
}

// PushSubscribe subscribes this device's browser. A call that was already
// on its way when the device was revoked doesn't bring the subscription
// back: revoking marks the device before it forgets its subscription.
func (c *client) PushSubscribe(sub push.Subscription) error {
	p, err := c.pusher()
	if err != nil {
		return err
	}
	if err := p.Subscribe(c.device.ID, sub); err != nil {
		return err
	}
	if c.server.links.isRevoked(c.device.ID) {
		_ = p.Forget(c.device.ID)
		return errors.New("this device is no longer paired")
	}
	return nil
}

func (c *client) PushUnsubscribe() error {
	p, err := c.pusher()
	if err != nil {
		return err
	}
	return p.Forget(c.device.ID)
}

// SetPushNotifications sets what this device is told: all, approvals or off.
func (c *client) SetPushNotifications(setting string) error {
	p, err := c.pusher()
	if err != nil {
		return err
	}
	return p.SetSetting(c.device.ID, setting)
}

// TestPush sends this device a sample notification.
func (c *client) TestPush(ctx context.Context) error {
	p, err := c.pusher()
	if err != nil {
		return err
	}
	return p.Test(ctx, c.device.ID, push.Note{Title: "UNCLI", Body: "Notifications are on. You'll get one like this when a session needs you.", Tag: "test"})
}

// UploadLink is a signed link to a file the device uploaded.
func (c *client) UploadLink(id string) (string, error) {
	if _, err := c.svc.UploadPath(id); err != nil {
		return "", err
	}
	return c.server.links.upload(c.device.ID, id), nil
}
