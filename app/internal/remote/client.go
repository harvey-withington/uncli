package remote

import (
	"errors"
	"sort"

	"uncli/internal/app"
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
	"CreateSession", "Pages", "Send", "Interrupt", "Focus", "Rename", "Delete",
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

// Focus marks a session read without taking the window's focus: the
// window's notifications still follow what's shown there.
func (c *client) Focus(sessionID string) {
	if sessionID != "" {
		c.svc.Sessions.MarkRead(sessionID)
	}
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

// ArtifactLink is a signed link to an artifact, to open it in the
// device's browser.
func (c *client) ArtifactLink(sessionID string, ref app.ArtifactRef) (string, error) {
	if _, err := c.svc.ReadArtifact(sessionID, ref); err != nil {
		return "", err
	}
	return c.server.links.artifact(sessionID, ref), nil
}

// UploadLink is a signed link to a file the device uploaded.
func (c *client) UploadLink(id string) (string, error) {
	if _, err := c.svc.UploadPath(id); err != nil {
		return "", err
	}
	return c.server.links.upload(id), nil
}
