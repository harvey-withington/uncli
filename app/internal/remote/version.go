package remote

import (
	"encoding/json"
	"net/http"

	"uncli/internal/buildinfo"
)

// GET /version needs no token: a client checks it before pairing and on
// every connect. Protocol changes when a call or event changes in a way
// an older client would misread; a client on another protocol shows a
// banner (decision 0021).

// Protocol is the remote API's version.
const Protocol = 1

// Capabilities are optional features this server has, for clients newer
// or older than it to check rather than guess.
var Capabilities = []string{"events-resume", "uploads", "signed-links", "folders"}

// VersionInfo is what /version reports.
type VersionInfo struct {
	App          string   `json:"app"`
	Protocol     int      `json:"protocol"`
	Capabilities []string `json:"capabilities"`
	Commit       string   `json:"commit,omitempty"`
	CommitDate   string   `json:"commitDate,omitempty"` // when the build's commit was made (RFC 3339)
}

func versionInfo() VersionInfo {
	// The app's version is the release's (buildinfo), or "dev".
	v := VersionInfo{App: buildinfo.Version, Protocol: Protocol, Capabilities: Capabilities}
	v.Commit, v.CommitDate = buildinfo.Commit()
	return v
}

func serveVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(versionInfo())
}
