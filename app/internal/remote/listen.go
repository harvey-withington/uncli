package remote

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Listening: off by default. When on, UNCLI listens on loopback (plain
// HTTP, for this machine) and on each of the machine's Tailscale addresses
// (HTTPS with the tailnet certificate), on one port. Never on the LAN or
// every address (decision 0021). A watcher follows Tailscale: when its
// address changes or it stops, the tailnet listeners follow, and the
// certificate is fetched again well before it expires.

// DefaultPort is the suggested port, clear of BRUV's 9870.
const DefaultPort = 9871

// Settings are Remote access's settings.
type Settings struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

const settingKey = "remote.access"

// States of Remote access.
const (
	StateOff      = "off"
	StateOn       = "on"       // on loopback and the tailnet
	StateLoopback = "loopback" // on loopback only; Reason says why not the tailnet
	StateError    = "error"    // not listening at all; Reason says why
)

// Status is what Settings → Remote access shows.
type Status struct {
	Enabled     bool      `json:"enabled"`
	Port        int       `json:"port"`
	State       string    `json:"state"`
	URL         string    `json:"url,omitempty"`      // https://<machine>.<tailnet>.ts.net:<port>
	Loopback    string    `json:"loopback,omitempty"` // http://127.0.0.1:<port>
	Reason      string    `json:"reason,omitempty"`
	Detail      string    `json:"detail,omitempty"`
	CertExpires time.Time `json:"certExpires,omitzero"`
}

// How often the watcher looks at Tailscale, and how often it fetches the
// certificate again (Tailscale renews it within its last third; fetching
// keeps the served one current).
var (
	watchEvery   = 30 * time.Second
	refreshEvery = 12 * time.Hour
)

// idleTimeout closes a kept-alive connection that has sent nothing for a
// while. There's no overall read timeout: it would cut the event stream.
const idleTimeout = 2 * time.Minute

// network is the listeners while Remote access is on.
type network struct {
	port     int
	loopback *http.Server
	tailnet  map[netip.Addr]*http.Server
	name     string
	cert     atomic.Pointer[tls.Certificate]
	fetched  time.Time
	stop     context.CancelFunc
	done     chan struct{}
	hosts    atomic.Pointer[[]string]
	mu       sync.Mutex // guards tailnet while the watcher changes it
}

func (s *Server) loadSettings() Settings {
	st := Settings{Port: DefaultPort}
	if s.store != nil {
		if raw, _ := s.store.Setting(settingKey); raw != "" {
			_ = json.Unmarshal([]byte(raw), &st)
		}
	}
	if st.Port <= 0 || st.Port > 65535 {
		st.Port = DefaultPort
	}
	return st
}

// Status is the current state of Remote access.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Server) setStatus(st Status) {
	s.mu.Lock()
	changed := s.status != st
	s.status = st
	s.mu.Unlock()
	if changed && s.onStatus != nil {
		s.onStatus(st)
	}
}

// Start applies the saved settings (UNCLI starting).
func (s *Server) Start() Status {
	s.applying.Lock()
	defer s.applying.Unlock()
	s.mu.Lock()
	st := s.settings
	running := s.net != nil
	s.mu.Unlock()
	if st.Enabled && !running {
		s.on(st.Port)
	}
	return s.Status()
}

// SetSettings saves Remote access's settings and applies them.
func (s *Server) SetSettings(st Settings) (Status, error) {
	if st.Port <= 0 || st.Port > 65535 {
		return s.Status(), fmt.Errorf("choose a port from 1 to 65535")
	}
	s.applying.Lock()
	defer s.applying.Unlock()
	b, _ := json.Marshal(st)
	if s.store != nil {
		if err := s.store.SetSetting(settingKey, string(b)); err != nil {
			return s.Status(), err
		}
	}
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	s.off()
	if st.Enabled {
		s.on(st.Port)
	} else {
		s.setStatus(Status{Port: st.Port, State: StateOff})
	}
	return s.Status(), nil
}

// Close turns the listeners off without changing the settings (UNCLI quitting).
func (s *Server) Close() {
	s.applying.Lock()
	defer s.applying.Unlock()
	s.off()
}

// off closes the listeners. The event streams end first: they never
// finish by themselves, so Shutdown would otherwise wait them out; and
// whatever is still open when Shutdown gives up is closed outright.
func (s *Server) off() {
	s.mu.Lock()
	n := s.net
	s.net = nil
	s.mu.Unlock()
	if n == nil {
		return
	}
	n.stop()
	<-n.done
	s.hub.CloseAll()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	n.mu.Lock()
	servers := append([]*http.Server{n.loopback}, slices.Collect(maps.Values(n.tailnet))...)
	n.mu.Unlock()
	for _, srv := range servers {
		if srv.Shutdown(ctx) != nil {
			_ = srv.Close()
		}
	}
}

func (s *Server) on(port int) {
	n := &network{port: port, tailnet: map[netip.Addr]*http.Server{}, done: make(chan struct{})}
	n.hosts.Store(&[]string{})
	handler := s.Handler(func() []string { return *n.hosts.Load() })
	n.loopback = &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: idleTimeout}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		close(n.done)
		s.setStatus(Status{Enabled: true, Port: port, State: StateError, Reason: ReasonListen, Detail: err.Error()})
		return
	}
	go func() { _ = n.loopback.Serve(ln) }()
	ctx, stop := context.WithCancel(context.Background())
	n.stop = stop
	s.mu.Lock()
	s.net = n
	s.mu.Unlock()
	s.follow(ctx, n) // the first look happens before Start returns
	go func() {
		defer close(n.done)
		t := time.NewTicker(watchEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.follow(ctx, n)
			}
		}
	}()
}

// follow looks at Tailscale and makes the tailnet listeners match: the
// machine's current addresses, with a current certificate.
func (s *Server) follow(ctx context.Context, n *network) {
	st := Status{Enabled: true, Port: n.port, Loopback: "http://127.0.0.1:" + strconv.Itoa(n.port)}
	lctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tn, err := s.certs.Lookup(lctx)
	if err == nil && (n.name != tn.Name || n.cert.Load() == nil || time.Since(n.fetched) > refreshEvery) {
		var cert *tls.Certificate
		if cert, err = s.certs.Cert(lctx, tn.Name); err == nil {
			n.cert.Store(cert)
			n.name, n.fetched = tn.Name, time.Now()
		}
	}
	want := []netip.Addr{}
	if err == nil {
		want = tn.IPs
	}
	problems := s.listenTailnet(n, want)
	hosts := []string{"127.0.0.1:" + strconv.Itoa(n.port), "localhost:" + strconv.Itoa(n.port)}
	switch {
	case err != nil:
		p := problemOf(err, ReasonNoTailscale)
		st.State, st.Reason, st.Detail = StateLoopback, p.Reason, p.Detail
	case len(want) == 0:
		st.State, st.Reason = StateLoopback, ReasonStopped
	case len(problems) == len(want):
		st.State, st.Reason, st.Detail = StateLoopback, ReasonListen, errors.Join(problems...).Error()
	default:
		st.State = StateOn
		st.URL = "https://" + tn.Name + ":" + strconv.Itoa(n.port)
		hosts = append(hosts, tn.Name+":"+strconv.Itoa(n.port))
		if c := n.cert.Load(); c != nil && c.Leaf != nil {
			st.CertExpires = c.Leaf.NotAfter
		}
	}
	n.hosts.Store(&hosts)
	if ctx.Err() == nil {
		s.setStatus(st)
	}
}

// listenTailnet opens a listener on each wanted address that hasn't one
// and closes those on addresses the machine no longer has.
func (s *Server) listenTailnet(n *network, want []netip.Addr) []error {
	n.mu.Lock()
	defer n.mu.Unlock()
	for addr, srv := range n.tailnet {
		if !slices.Contains(want, addr) {
			_ = srv.Close()
			delete(n.tailnet, addr)
		}
	}
	var problems []error
	for _, addr := range want {
		if _, ok := n.tailnet[addr]; ok {
			continue
		}
		ln, err := net.Listen("tcp", netip.AddrPortFrom(addr, uint16(n.port)).String())
		if err != nil {
			problems = append(problems, err)
			continue
		}
		srv := &http.Server{
			Handler:           s.Handler(func() []string { return *n.hosts.Load() }),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       idleTimeout,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
					if c := n.cert.Load(); c != nil {
						return c, nil
					}
					return nil, errors.New("no certificate yet")
				},
			},
		}
		n.tailnet[addr] = srv
		go func() { _ = srv.ServeTLS(ln, "", "") }()
	}
	return problems
}
