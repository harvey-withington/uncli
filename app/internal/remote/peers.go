package remote

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/harvey-withington/uncli/app/internal/secrets"
)

// Peers is this UNCLI as a paired device of other UNCLIs: the desktop
// showing a home server's sessions beside its own (decision 0021, phase 2).
// It pairs the way a phone does, keeps each server's token in the
// credential store, makes calls and keeps each server's event stream open,
// handing the events up to the window. The window never fetches a server
// itself: a server refuses other origins, and the token stays out of the
// window's storage (decision 0031).

// PeerServer is a paired server.
type PeerServer struct {
	Key    string `json:"key"`    // host and port: how the window names the server in session ids
	URL    string `json:"url"`    // https://host:port
	Name   string `json:"name"`   // the machine's name, to show
	Device string `json:"device"` // this PC's device id there
}

// Peer connection states.
const (
	PeerConnecting = "connecting"
	PeerOnline     = "online"
	PeerOffline    = "offline"
	PeerUnpaired   = "unpaired" // the server refused the token: revoked there
)

// PeerState is a paired server and its connection, for Settings and the
// sidebar.
type PeerState struct {
	PeerServer
	State      string     `json:"state"`
	Error      string     `json:"error,omitempty"`
	LastOnline *time.Time `json:"lastOnline,omitempty"`
	App        string     `json:"app,omitempty"` // the server's UNCLI version, from /version
	Protocol   int        `json:"protocol,omitempty"`
}

// Tokens keeps each server's token (the credential store; a map in tests).
type Tokens interface {
	Get(name string) (string, error)
	Set(name, value string) error
}

type credentialTokens struct{}

func (credentialTokens) Get(name string) (string, error) { return secrets.Get(name) }
func (credentialTokens) Set(name, value string) error    { return secrets.Set(name, value) }

// PeersConfig is what Peers is built from.
type PeersConfig struct {
	Store  Store
	Tokens Tokens       // nil: the credential store
	Client *http.Client // nil: a client for the tailnet
	// OnEvent is each event a server sends (hello included, with its reload
	// flag); OnStatus the servers whenever one's state changes.
	OnEvent  func(server, name string, data json.RawMessage)
	OnStatus func([]PeerState)
}

const peersKey = "peers.servers"

type Peers struct {
	store    Store
	tokens   Tokens
	client   *http.Client // calls
	streamer *http.Client // event streams: no overall timeout
	onEvent  func(string, string, json.RawMessage)
	onStatus func([]PeerState)

	mu      sync.Mutex
	peers   map[string]*peer
	order   []string // keys, in the order they were added
	adding  context.CancelFunc
	started bool
	closed  bool
}

type peer struct {
	state  PeerState
	token  string
	cancel context.CancelFunc // the event stream
}

func NewPeers(c PeersConfig) *Peers {
	p := &Peers{store: c.Store, tokens: c.Tokens, client: c.Client, onEvent: c.OnEvent, onStatus: c.OnStatus,
		peers: map[string]*peer{}}
	if p.tokens == nil {
		p.tokens = credentialTokens{}
	}
	if p.client == nil {
		p.client = &http.Client{Timeout: 30 * time.Second}
	}
	p.streamer = &http.Client{Transport: p.client.Transport}
	var saved []PeerServer
	if raw, err := c.Store.Setting(peersKey); err == nil && raw != "" {
		_ = json.Unmarshal([]byte(raw), &saved)
	}
	for _, s := range saved {
		token, _ := p.tokens.Get(tokenName(s.Key))
		st := PeerState{PeerServer: s, State: PeerOffline}
		if token == "" {
			st.State, st.Error = PeerUnpaired, "This PC's pairing was lost: remove the server and pair again."
		}
		p.peers[s.Key] = &peer{state: st, token: token}
		p.order = append(p.order, s.Key)
	}
	return p
}

func tokenName(key string) string { return "peer:" + key }

// Start opens every paired server's event stream.
func (p *Peers) Start() {
	p.mu.Lock()
	p.started = true
	for _, k := range p.order {
		p.connectLocked(p.peers[k])
	}
	p.mu.Unlock()
	p.changed()
}

// Close ends every stream and any pairing under way.
func (p *Peers) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	if p.adding != nil {
		p.adding()
	}
	for _, pr := range p.peers {
		if pr.cancel != nil {
			pr.cancel()
		}
	}
}

// List is the paired servers, in the order they were added.
func (p *Peers) List() []PeerState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.listLocked()
}

func (p *Peers) listLocked() []PeerState {
	out := make([]PeerState, 0, len(p.order))
	for _, k := range p.order {
		out = append(out, p.peers[k].state)
	}
	return out
}

func (p *Peers) changed() {
	if p.onStatus != nil {
		p.onStatus(p.List())
	}
}

// ParsePairLink reads a server's pairing link (https://host:port/m/#pair=CODE):
// its address and the code.
func ParsePairLink(link string) (base, code string, err error) {
	u, err := url.Parse(strings.TrimSpace(link))
	bad := errors.New("paste the whole pairing link the server shows in Settings → Remote access → Pair a device")
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", "", bad
	}
	frag, _ := url.ParseQuery(u.Fragment)
	code = strings.ToUpper(strings.TrimSpace(frag.Get("pair")))
	if code == "" {
		return "", "", bad
	}
	return "https://" + u.Host, code, nil
}

// peerName is what a server is shown as: its machine name (machineName),
// else its address.
func peerName(host string) string {
	if n := machineName(host); n != "" {
		return n
	}
	return host
}

// Add pairs with the server whose pairing link this is, as name (this PC),
// and waits for Allow there. Cancel, or a new Add, stops waiting.
func (p *Peers) Add(ctx context.Context, link, name string) (PeerState, error) {
	base, code, err := ParsePairLink(link)
	if err != nil {
		return PeerState{}, err
	}
	key := strings.TrimPrefix(base, "https://")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.mu.Lock()
	if p.adding != nil {
		p.adding()
	}
	p.adding = cancel
	p.mu.Unlock()

	info, err := p.version(ctx, base)
	if err != nil {
		return PeerState{}, err
	}
	if info.Protocol != Protocol {
		return PeerState{}, fmt.Errorf("that server runs UNCLI %s, which this version can't talk to: update both to the same version", info.App)
	}
	var req struct {
		ID, Claim, Error string
	}
	if code, err := p.post(ctx, base, "/pair/request", map[string]string{"code": code, "name": name}, &req); err != nil {
		return PeerState{}, err
	} else if code != http.StatusOK {
		return PeerState{}, errors.New(orStatus(req.Error, code))
	}
	var claim struct {
		Device Device
		Token  string
		Error  string
	}
	for {
		status, err := p.post(ctx, base, "/pair/claim", map[string]string{"id": req.ID, "claim": req.Claim}, &claim)
		if err != nil {
			return PeerState{}, err
		}
		if status == http.StatusAccepted {
			continue // still waiting for Allow
		}
		if status != http.StatusOK {
			return PeerState{}, errors.New(orStatus(claim.Error, status))
		}
		break
	}
	if err := p.tokens.Set(tokenName(key), claim.Token); err != nil {
		return PeerState{}, fmt.Errorf("couldn't keep the server's token: %w", err)
	}
	srv := PeerServer{Key: key, URL: base, Name: peerName(key), Device: claim.Device.ID}
	p.mu.Lock()
	if old, ok := p.peers[key]; ok && old.cancel != nil {
		old.cancel()
	} else if !ok {
		p.order = append(p.order, key)
	}
	pr := &peer{state: PeerState{PeerServer: srv, State: PeerConnecting, App: info.App, Protocol: info.Protocol}, token: claim.Token}
	p.peers[key] = pr
	err = p.saveLocked()
	if p.started {
		p.connectLocked(pr)
	}
	st := pr.state
	p.mu.Unlock()
	p.changed()
	return st, err
}

// CancelAdd stops waiting for a pairing.
func (p *Peers) CancelAdd() {
	p.mu.Lock()
	if p.adding != nil {
		p.adding()
	}
	p.mu.Unlock()
}

// Remove forgets a server: its stream closes and its token goes. The
// server still lists this PC until it's revoked there.
func (p *Peers) Remove(key string) error {
	p.mu.Lock()
	pr, ok := p.peers[key]
	if !ok {
		p.mu.Unlock()
		return errors.New("no such server")
	}
	if pr.cancel != nil {
		pr.cancel()
	}
	delete(p.peers, key)
	p.order = slices.DeleteFunc(p.order, func(k string) bool { return k == key })
	err := p.saveLocked()
	p.mu.Unlock()
	if terr := p.tokens.Set(tokenName(key), ""); err == nil {
		err = terr
	}
	p.changed()
	return err
}

func (p *Peers) saveLocked() error {
	list := make([]PeerServer, 0, len(p.order))
	for _, k := range p.order {
		list = append(list, p.peers[k].state.PeerServer)
	}
	raw, _ := json.Marshal(list)
	return p.store.SetSetting(peersKey, string(raw))
}

// ErrPeerUnpaired is a call a server refused: this PC was revoked there.
var ErrPeerUnpaired = errors.New("this server no longer accepts this PC: pair it again in Settings → Remote servers")

func (p *Peers) get(key string) (PeerServer, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pr, ok := p.peers[key]
	if !ok {
		return PeerServer{}, "", errors.New("that server isn't paired with this PC")
	}
	if pr.token == "" {
		return PeerServer{}, "", ErrPeerUnpaired
	}
	return pr.state.PeerServer, pr.token, nil
}

// Call runs a method on a server, with positional params, and returns its
// raw result. An error the server's app gave is returned as its message.
func (p *Peers) Call(ctx context.Context, key, method string, params []json.RawMessage) (json.RawMessage, error) {
	srv, token, err := p.get(key)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = []json.RawMessage{}
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/rpc", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return p.reply(key, req, func(r io.Reader) (json.RawMessage, error) {
		var res struct {
			Result json.RawMessage `json:"result"`
			Error  *rpcError       `json:"error"`
		}
		if err := json.NewDecoder(r).Decode(&res); err != nil {
			return nil, err
		}
		if res.Error != nil {
			return nil, errors.New(res.Error.Message)
		}
		return res.Result, nil
	})
}

// Upload sends a file from this PC to a server, to attach to a message
// there, and returns what it attaches as.
func (p *Peers) Upload(ctx context.Context, key, path string) (json.RawMessage, error) {
	srv, token, err := p.get(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/upload?name="+url.QueryEscape(filepath.Base(path)), f)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	cl := &http.Client{Transport: p.client.Transport, Timeout: uploadTime}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, p.unreachable(srv, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		p.unpaired(key)
		return nil, ErrPeerUnpaired
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct{ Error string }
		_ = json.Unmarshal(raw, &e)
		return nil, errors.New(orStatus(e.Error, resp.StatusCode))
	}
	return raw, nil
}

// reply sends a request and reads its JSON reply with read.
func (p *Peers) reply(key string, req *http.Request, read func(io.Reader) (json.RawMessage, error)) (json.RawMessage, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		srv, _, _ := p.get(key)
		return nil, p.unreachable(srv, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		p.unpaired(key)
		return nil, ErrPeerUnpaired
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		return nil, fmt.Errorf("the server sent something unexpected (%s)", resp.Status)
	}
	return read(resp.Body)
}

func (p *Peers) unreachable(srv PeerServer, err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	name := srv.Name
	if name == "" {
		name = "the server"
	}
	return fmt.Errorf("can't reach %s: %w", name, err)
}

// unpaired marks a server that refused this PC's token, and stops its
// stream: retrying wouldn't help.
func (p *Peers) unpaired(key string) {
	p.mu.Lock()
	pr, ok := p.peers[key]
	if ok && pr.state.State != PeerUnpaired {
		pr.state.State, pr.state.Error = PeerUnpaired, ErrPeerUnpaired.Error()
		if pr.cancel != nil {
			pr.cancel()
			pr.cancel = nil
		}
	}
	p.mu.Unlock()
	if ok {
		p.changed()
	}
}

func (p *Peers) version(ctx context.Context, base string) (VersionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/version", nil)
	if err != nil {
		return VersionInfo{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("can't reach %s: %w", peerName(strings.TrimPrefix(base, "https://")), err)
	}
	defer resp.Body.Close()
	var v VersionInfo
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&v) != nil {
		return VersionInfo{}, fmt.Errorf("that address doesn't look like UNCLI's remote access (%s)", resp.Status)
	}
	return v, nil
}

// post sends JSON and decodes the JSON reply into out, whatever its status.
func (p *Peers) post(ctx context.Context, base, path string, in, out any) (int, error) {
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return 0, errors.New("pairing cancelled")
		}
		return 0, fmt.Errorf("can't reach the server: %w", err)
	}
	defer resp.Body.Close()
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(out)
	return resp.StatusCode, nil
}

func orStatus(msg string, code int) string {
	if msg != "" {
		return msg
	}
	return fmt.Sprintf("the server answered %d %s", code, http.StatusText(code))
}

// The event stream, as the phone keeps it (lib/api/http.ts): reconnect with
// backoff, resume from the last cursor, give up on a stream silent for
// longer than the server's pings.
var (
	peerBackoff = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second}
	peerStall   = 45 * time.Second
)

func (p *Peers) connectLocked(pr *peer) {
	if p.closed || pr.token == "" || pr.state.State == PeerUnpaired {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	pr.cancel = cancel
	go p.run(ctx, pr.state.Key)
}

func (p *Peers) setState(key string, f func(*PeerState)) {
	p.mu.Lock()
	pr, ok := p.peers[key]
	if ok {
		f(&pr.state)
	}
	p.mu.Unlock()
	if ok {
		p.changed()
	}
}

func (p *Peers) run(ctx context.Context, key string) {
	cursor := ""
	for attempt := 0; ctx.Err() == nil; attempt++ {
		p.setState(key, func(s *PeerState) { s.State = PeerConnecting })
		online, err := p.stream(ctx, key, &cursor)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, ErrPeerUnpaired) {
			p.unpaired(key)
			return
		}
		if online {
			attempt = 0
		}
		p.setState(key, func(s *PeerState) { s.State, s.Error = PeerOffline, err.Error() })
		wait := peerBackoff[min(attempt, len(peerBackoff)-1)]
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// stream reads one connection's events until it ends; online says it got
// as far as hello.
func (p *Peers) stream(ctx context.Context, key string, cursor *string) (online bool, err error) {
	srv, token, err := p.get(key)
	if err != nil {
		return false, err
	}
	if info, err := p.version(ctx, srv.URL); err == nil {
		p.setState(key, func(s *PeerState) { s.App, s.Protocol = info.App, info.Protocol })
		if info.Protocol != Protocol {
			return false, fmt.Errorf("%s runs UNCLI %s, which this version can't talk to: update both to the same version", srv.Name, info.App)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled atomic.Bool
	stall := time.AfterFunc(peerStall, func() { stalled.Store(true); cancel() })
	defer stall.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if *cursor != "" {
		req.Header.Set("Last-Event-ID", *cursor)
	}
	resp, err := p.streamer.Do(req)
	if err != nil {
		return false, p.unreachable(srv, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return false, ErrPeerUnpaired
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s answered %s", srv.Name, resp.Status)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 32<<20)
	var id, name string
	var data []string
	for sc.Scan() {
		stall.Reset(peerStall)
		line := sc.Text()
		if line != "" {
			field, value, _ := strings.Cut(line, ":")
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "id":
				id = value
			case "event":
				name = value
			case "data":
				data = append(data, value)
			}
			continue
		}
		if name != "" && len(data) > 0 {
			raw := json.RawMessage(strings.Join(data, "\n"))
			if id != "" {
				*cursor = id
			}
			if name == "hello" {
				var h struct{ Cursor string }
				_ = json.Unmarshal(raw, &h)
				*cursor = h.Cursor
				online = true
				now := time.Now()
				p.setState(key, func(s *PeerState) { s.State, s.Error, s.LastOnline = PeerOnline, "", &now })
			}
			if p.onEvent != nil {
				p.onEvent(key, name, raw)
			}
		}
		id, name, data = "", "", nil
	}
	switch err := sc.Err(); {
	case stalled.Load():
		return online, fmt.Errorf("%s stopped answering", srv.Name)
	case err != nil && ctx.Err() == nil:
		return online, p.unreachable(srv, err)
	}
	return online, fmt.Errorf("%s closed the connection", srv.Name)
}
