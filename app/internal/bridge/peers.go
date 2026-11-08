package bridge

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/remote"
)

// Remote servers (decision 0031): other UNCLIs this PC is paired with. The
// window reaches them only through these calls and events; the tokens stay
// on the Go side.

const (
	EvtPeerEvent  = "peer:event"  // PeerEvent: an event from a paired server
	EvtPeerStatus = "peer:status" // []remote.PeerState
)

// PeerEvent is one event from a paired server, as the server sent it.
type PeerEvent struct {
	Server string          `json:"server"`
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
}

// newPeers passes each server's events to the window, and through the
// desktop's notification rules (app.PeerNotes).
func (a *App) newPeers() *remote.Peers {
	var mu sync.Mutex
	notes := map[string]*app.PeerNotes{}
	return remote.NewPeers(remote.PeersConfig{Store: a.svc.Store,
		OnEvent: func(server, name string, data json.RawMessage) {
			a.Emit(EvtPeerEvent, PeerEvent{server, name, data})
			mu.Lock()
			n := notes[server]
			if n == nil {
				n = a.svc.NewPeerNotes(server)
				notes[server] = n
			}
			mu.Unlock()
			n.Event(name, data)
		},
		OnStatus: func(l []remote.PeerState) { a.Emit(EvtPeerStatus, l) }})
}

// RemoteServers lists the paired servers and their connections.
func (a *App) RemoteServers() []remote.PeerState { return a.peers.List() }

// AddRemoteServer pairs with the server whose pairing link this is, as
// name ("" for this PC's own), and waits for Allow there.
func (a *App) AddRemoteServer(link, name string) (remote.PeerState, error) {
	if name == "" {
		name, _ = os.Hostname()
	}
	return a.peers.Add(context.Background(), link, name)
}

// CancelAddRemoteServer stops waiting for Allow.
func (a *App) CancelAddRemoteServer() { a.peers.CancelAdd() }

// RemoveRemoteServer forgets a server and its token.
func (a *App) RemoveRemoteServer(server string) error { return a.peers.Remove(server) }

// PeerCall runs a method on a paired server.
func (a *App) PeerCall(server, method string, params []json.RawMessage) (json.RawMessage, error) {
	return a.peers.Call(context.Background(), server, method, params)
}

// PeerUpload sends a file from this PC to a paired server, to attach to a
// message there.
func (a *App) PeerUpload(server, path string) (json.RawMessage, error) {
	return a.peers.Upload(context.Background(), server, path)
}

// ThisPCName is the name this PC pairs as, unless the user gives another.
func (a *App) ThisPCName() string {
	n, _ := os.Hostname()
	return n
}
