package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/harvey-withington/uncli/app/internal/app"
)

// lockedMap stands in for the credential store (Get, Set) and the
// settings store (Setting, SetSetting), safe for the streams' goroutines.
type lockedMap struct {
	mu sync.Mutex
	m  map[string]string
}

func newLockedMap() *lockedMap { return &lockedMap{m: map[string]string{}} }

func (l *lockedMap) Get(name string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.m[name], nil
}

func (l *lockedMap) Set(name, value string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if value == "" {
		delete(l.m, name)
	} else {
		l.m[name] = value
	}
	return nil
}

func (l *lockedMap) Setting(k string) (string, error) { return l.Get(k) }
func (l *lockedMap) SetSetting(k, v string) error     { return l.Set(k, v) }

type peerEvent struct{ server, name, data string }

// peerRig is a server (the rig) that answers every pairing request with
// allow, and a laptop's Peers pointed at it.
type peerRig struct {
	*rig
	peers  *Peers
	tokens *lockedMap
	store  *lockedMap
	events chan peerEvent
	states chan []PeerState
}

func newPeerRig(t *testing.T, allow bool) *peerRig {
	t.Helper()
	pr := &peerRig{tokens: newLockedMap(), store: newLockedMap(),
		events: make(chan peerEvent, 1024), states: make(chan []PeerState, 1024)}
	pr.rig = newRigWith(t, Config{OnPairRequest: func(req PairRequest) {
		if err := pr.srv.AnswerPairing(req.ID, allow); err != nil {
			t.Error(err)
		}
	}})
	pr.peers = pr.newPeers()
	return pr
}

func (pr *peerRig) newPeers() *Peers {
	p := NewPeers(PeersConfig{Store: pr.store, Tokens: pr.tokens, Client: pr.https,
		OnEvent:  func(s, n string, d json.RawMessage) { pr.events <- peerEvent{s, n, string(d)} },
		OnStatus: func(l []PeerState) { pr.states <- l }})
	pr.t.Cleanup(p.Close)
	return p
}

func (pr *peerRig) link() string {
	pr.t.Helper()
	p, err := pr.srv.StartPairing()
	if err != nil {
		pr.t.Fatal(err)
	}
	return p.URL
}

func (pr *peerRig) waitState(what string, ok func([]PeerState) bool) []PeerState {
	pr.t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case l := <-pr.states:
			if ok(l) {
				return l
			}
		case <-timeout:
			pr.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func (pr *peerRig) waitEvent(what string, ok func(peerEvent) bool) peerEvent {
	pr.t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case e := <-pr.events:
			if ok(e) {
				return e
			}
		case <-timeout:
			pr.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// A laptop pairs with a server as a phone does, keeps the token in the
// credential store, sees the server's events live and drives it over RPC.
func TestPeerPairsAndDrives(t *testing.T) {
	pr := newPeerRig(t, true)
	pr.peers.Start()
	st, err := pr.peers.Add(context.Background(), pr.link(), "Laptop")
	if err != nil {
		t.Fatal(err)
	}
	if st.Key != pr.base()[len("https://"):] || st.Name != "test" || st.Device == "" || st.Protocol != Protocol {
		t.Errorf("state = %+v", st)
	}
	if tok, _ := pr.tokens.Get("peer:" + st.Key); tok == "" {
		t.Error("the token wasn't kept")
	}
	if saved, _ := pr.store.Setting(peersKey); strings.Contains(saved, "token") || !strings.Contains(saved, st.Key) {
		t.Errorf("saved list = %s", saved)
	}
	if ds := pr.srv.Devices(); len(ds) != 1 || ds[0].Name != "Laptop" {
		t.Errorf("server's devices = %+v", ds)
	}

	pr.waitState("online", func(l []PeerState) bool { return len(l) == 1 && l[0].State == PeerOnline })
	pr.waitEvent("hello", func(e peerEvent) bool { return e.server == st.Key && e.name == "hello" })

	res, err := pr.peers.Call(context.Background(), st.Key, "Bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	var boot app.Bootstrap
	if err := json.Unmarshal(res, &boot); err != nil || len(boot.Profiles) == 0 {
		t.Fatalf("Bootstrap = %s (%v)", res, err)
	}
	res, err = pr.peers.Call(context.Background(), st.Key, "CreateSession", raw("code", t.TempDir(), "haiku", "", "", []string{}))
	if err != nil {
		t.Fatal(err)
	}
	var created app.CreatedSession
	_ = json.Unmarshal(res, &created)
	pr.waitEvent("the new session", func(e peerEvent) bool {
		return e.name == "session:changed" && strings.Contains(e.data, created.Session.ID)
	})
	// The app's own errors come back as its message.
	if _, err := pr.peers.Call(context.Background(), st.Key, "Rename", raw("nope", "x")); err == nil {
		t.Error("renaming an unknown session should fail")
	}
	if _, err := pr.peers.Call(context.Background(), st.Key, "InstallCLI", nil); err == nil || !strings.Contains(err.Error(), "doesn't offer") {
		t.Errorf("a method the server keeps to itself = %v", err)
	}

	// A file on this PC goes to the server, to attach there.
	note := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(note, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	up, err := pr.peers.Upload(context.Background(), st.Key, note)
	if err != nil {
		t.Fatal(err)
	}
	var info struct{ ID, Name string }
	if json.Unmarshal(up, &info) != nil || info.ID == "" || info.Name != "notes.txt" {
		t.Errorf("upload = %s", up)
	}

	// The list survives a restart.
	again := pr.newPeers()
	if l := again.List(); len(l) != 1 || l[0].Key != st.Key || l[0].State != PeerOffline {
		t.Errorf("reloaded = %+v", l)
	}

	if err := pr.peers.Remove(st.Key); err != nil {
		t.Fatal(err)
	}
	if tok, _ := pr.tokens.Get("peer:" + st.Key); tok != "" {
		t.Error("Remove kept the token")
	}
	if l := pr.peers.List(); len(l) != 0 {
		t.Errorf("after Remove = %+v", l)
	}
}

// Revoked at the server, the laptop is told at once and stops retrying.
func TestPeerRevoked(t *testing.T) {
	pr := newPeerRig(t, true)
	pr.peers.Start()
	st, err := pr.peers.Add(context.Background(), pr.link(), "Laptop")
	if err != nil {
		t.Fatal(err)
	}
	pr.waitState("online", func(l []PeerState) bool { return len(l) == 1 && l[0].State == PeerOnline })
	if err := pr.srv.RevokeDevice(st.Device); err != nil {
		t.Fatal(err)
	}
	pr.waitState("unpaired", func(l []PeerState) bool { return len(l) == 1 && l[0].State == PeerUnpaired })
	if _, err := pr.peers.Call(context.Background(), st.Key, "Bootstrap", nil); !errors.Is(err, ErrPeerUnpaired) {
		t.Errorf("call after revoke = %v", err)
	}
}

func TestPeerDenied(t *testing.T) {
	pr := newPeerRig(t, false)
	if _, err := pr.peers.Add(context.Background(), pr.link(), "Laptop"); err == nil || !strings.Contains(err.Error(), "didn't allow") {
		t.Errorf("denied = %v", err)
	}
	if l := pr.peers.List(); len(l) != 0 {
		t.Errorf("a denied server was kept: %+v", l)
	}
	// A spent code doesn't work twice.
	link := pr.link()
	pr.srv.CancelPairing()
	if _, err := pr.peers.Add(context.Background(), link, "Laptop"); err == nil {
		t.Error("a cancelled code paired")
	}
}

func TestParsePairLink(t *testing.T) {
	base, code, err := ParsePairLink(" https://workshop.tail0.ts.net:9871/m/#pair=abcde23456 ")
	if err != nil || base != "https://workshop.tail0.ts.net:9871" || code != "ABCDE23456" {
		t.Errorf("= %q %q %v", base, code, err)
	}
	for _, bad := range []string{"", "workshop:9871", "http://workshop:9871/m/#pair=X", "https://workshop:9871/m/"} {
		if _, _, err := ParsePairLink(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if n := peerName("workshop.tail0.ts.net:9871"); n != "workshop" || peerName("100.64.0.1:9871") != "100.64.0.1:9871" {
		t.Errorf("peerName = %q", n)
	}
}

// A server that doesn't answer is described in plain words, whatever the
// network library said, and the cause is still there underneath.
func TestPeerUnreachableWords(t *testing.T) {
	p := &Peers{}
	srv := PeerServer{Name: "deviant"}
	slow := p.unreachable(srv, fmt.Errorf("Post \"https://deviant:9871/rpc\": %w", context.DeadlineExceeded))
	if slow.Error() != "deviant took too long to answer. It may be busy or asleep; try again in a moment." || !errors.Is(slow, context.DeadlineExceeded) {
		t.Errorf("timeout = %q", slow)
	}
	_, dial := net.Dial("tcp", "127.0.0.1:1")
	if dial == nil {
		t.Skip("something listens on port 1")
	}
	gone := p.unreachable(srv, dial)
	if gone.Error() != "Can't reach deviant. Check it's on and connected to Tailscale." {
		t.Errorf("refused = %q", gone)
	}
	if err := p.unreachable(srv, context.Canceled); !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "deviant") {
		t.Errorf("cancelled = %v", err)
	}
}
