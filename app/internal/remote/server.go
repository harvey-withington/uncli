// Package remote serves UNCLI's sessions to paired devices: a phone's PWA
// now, and other UNCLIs later (decision 0021). It's a transport over the
// app's API, beside the Wails bridge: JSON-RPC over HTTP, an event stream
// clients resume after a dropped connection, uploads, signed links and
// /version. It listens only when Remote access is on, and then only on
// loopback and the machine's Tailscale addresses, with HTTPS from the
// machine's own tailnet certificate. Devices authenticate with their own
// token, sent only in the Authorization header (decision 0022).
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/harvey-withington/uncli/app/internal/app"
	"github.com/harvey-withington/uncli/app/internal/notify"
	"github.com/harvey-withington/uncli/app/internal/push"
)

// Device is a paired device.
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Authenticator says which paired device a token belongs to.
type Authenticator interface {
	Authenticate(token string) (Device, bool)
}

// Store keeps the server's settings (the app's settings store).
type Store interface {
	Setting(key string) (string, error)
	SetSetting(key, value string) error
}

// Config is what a Server is built from.
type Config struct {
	API     *app.API
	Service *app.Service
	Hub     *Hub
	Store   Store
	Certs   CertSource
	Auth    Authenticator
	// PWA is the phone app, served at /m/ (nil: none).
	PWA fs.FS
	// Push sends notifications to paired devices' browsers (nil: none).
	Push *push.Pusher
	// OnStatus is told whenever the status changes, OnPairRequest when a
	// device asks to pair, OnDevices when the paired devices change (the
	// bridge passes them on to the window).
	OnStatus      func(Status)
	OnPairRequest func(PairRequest)
	OnDevices     func([]PairedDevice)
	// Methods is what clients may call (nil: a paired device's,
	// rpcMethods). A host's own UNCLI passes EmbeddedMethods.
	Methods []string
}

// Server is the remote transport. Its zero state is off.
type Server struct {
	api   *app.API
	svc   *app.Service
	hub   *Hub
	store Store
	certs CertSource
	auth  Authenticator
	links *links
	pwa   fs.FS
	names map[string]bool // the methods clients may call

	devices *Devices
	pair    *pairing
	push    *push.Pusher

	onStatus func(Status)

	// applying is held while the listeners are turned on or off, so two
	// changes at once can't leave one's listeners running unseen.
	applying sync.Mutex

	mu       sync.Mutex
	settings Settings
	status   Status
	net      *network       // the listeners while on
	streams  map[string]int // device id → its open event streams
}

func New(c Config) *Server {
	s := &Server{api: c.API, svc: c.Service, hub: c.Hub, store: c.Store, certs: c.Certs, auth: c.Auth,
		links: newLinks(), pwa: c.PWA, push: c.Push, onStatus: c.OnStatus, streams: map[string]int{}, names: rpcNames}
	if c.Methods != nil {
		s.names = map[string]bool{}
		for _, m := range c.Methods {
			s.names[m] = true
		}
	}
	if c.Store != nil {
		s.devices = NewDevices(c.Store)
		if c.OnDevices != nil {
			s.devices.changed = func() { c.OnDevices(s.devices.List()) }
		}
		if s.auth == nil {
			s.auth = s.devices
		}
		s.pair = newPairing(s.devices)
		s.pair.asked = c.OnPairRequest
		// Subscriptions of devices no longer paired go.
		if s.push != nil && s.auth == Authenticator(s.devices) {
			_ = s.push.Retain(s.devices.Has)
		}
	}
	s.settings = s.loadSettings()
	s.status = Status{Enabled: false, Port: s.settings.Port, State: StateOff}
	return s
}

// Handler is the server's routes for requests to host (a name and port it
// answers to). Requests for any other host are refused, so a web page
// can't reach UNCLI through a DNS name it controls.
func (s *Server) Handler(hosts func() []string) http.Handler {
	return guard(hosts, s.routes())
}

// EmbeddedHandler is the routes of a host's own UNCLI (decision 0027),
// which the host mounts on its own server or forwards a path to. The
// host's server decides who gets this far; here, only pages from the
// host's origins may send requests, so another site can't drive UNCLI
// through the user's cookies for the host.
func (s *Server) EmbeddedHandler(allowed func(origin string, r *http.Request) bool) http.Handler {
	return embeddedGuard(allowed, s.routes())
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", serveVersion)
	mux.Handle("POST /rpc", s.authed(func(w http.ResponseWriter, r *http.Request, d Device) {
		serveRPC(w, r, &client{API: s.api, svc: s.svc, server: s, device: d}, s.names)
	}))
	mux.Handle("GET /events", s.authed(s.serveEvents))
	mux.Handle("POST /upload", s.authed(s.serveUploadPost))
	mux.HandleFunc("GET /l/artifact", s.serveArtifact)
	mux.HandleFunc("GET /l/upload", s.serveUpload)
	if s.pair != nil {
		mux.HandleFunc("POST /pair/request", s.servePairRequest)
		mux.HandleFunc("POST /pair/claim", s.servePairClaim)
	}
	if s.pwa != nil {
		mux.Handle("GET /m/", servePWA(s.pwa))
		mux.Handle("GET /{$}", http.RedirectHandler("/m/", http.StatusFound))
	}
	return mux
}

// embeddedGuard is guard for a host's own UNCLI. The Host header is the
// host's business (its server answers to its own names, and a forwarded
// request may carry either), so only the Origin is checked: a request a
// page sends must come from one of the host's origins. Requests without
// one (the host's server itself, a same-origin GET) pass. A host's page
// may come from another origin than its server (a desktop app's webview,
// wails.localhost say), so those origins get CORS: the preflight is
// answered, and responses may be read. Never credentials: the host signs
// in with a header (ChatHost.headers) or, same-origin, its own cookie.
func embeddedGuard(allowed func(origin string, r *http.Request) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		o := r.Header.Get("Origin")
		if o != "" && !allowed(o, r) {
			http.Error(w, "Cross-origin requests aren't allowed", http.StatusForbidden)
			return
		}
		if o != "" {
			h.Set("Access-Control-Allow-Origin", o)
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if o == "" {
				http.Error(w, "Cross-origin requests aren't allowed", http.StatusForbidden)
				return
			}
			h.Set("Access-Control-Allow-Methods", "GET, POST")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Last-Event-ID")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// guard checks the Host and Origin of every request and sets the headers
// every response gets. Same-origin pages need no CORS headers, so none are
// ever sent: a page from any other origin gets nothing it can read, and a
// request it sends with its origin is refused before it runs (decision
// 0022).
func guard(hosts func() []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		allowed := hosts()
		if !hostAllowed(r.Host, allowed) {
			http.Error(w, "Unknown host", http.StatusMisdirectedRequest)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !originAllowed(o, allowed) {
			http.Error(w, "Cross-origin requests aren't allowed", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodOptions {
			http.Error(w, "Cross-origin requests aren't allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hostAllowed(host string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(host, a) {
			return true
		}
	}
	return false
}

// originAllowed: the page's own origin, https on the tailnet name or http
// on loopback.
func originAllowed(origin string, hosts []string) bool {
	for _, h := range hosts {
		if strings.EqualFold(origin, "https://"+h) || (isLoopbackHost(h) && strings.EqualFold(origin, "http://"+h)) {
			return true
		}
	}
	return false
}

func isLoopbackHost(h string) bool {
	return strings.HasPrefix(h, "127.0.0.1:") || strings.HasPrefix(h, "localhost:") || strings.HasPrefix(h, "[::1]:")
}

// authed runs a handler for the device whose token is in the
// Authorization header; anything else is refused at once. A token in the
// URL is never looked at.
func (s *Server) authed(h func(http.ResponseWriter, *http.Request, Device)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" || s.auth == nil {
			unauthorized(w)
			return
		}
		d, ok := s.auth.Authenticate(token)
		if !ok {
			unauthorized(w)
			return
		}
		h(w, r, d)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"not paired"}`))
}

// serveEvents streams events as server-sent events, read with fetch (so
// the token stays in a header). The first event, hello, says whether the
// client must reload; then come the events it missed and the live ones,
// each with its cursor as the id. Last-Event-ID resumes.
func (s *Server) serveEvents(w http.ResponseWriter, r *http.Request, d Device) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	sub, c := s.hub.Subscribe(d.ID, r.Header.Get("Last-Event-ID"))
	defer s.hub.Unsubscribe(sub)
	s.streamOpened(d.ID)
	defer s.streamClosed(d.ID)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	hello, _ := json.Marshal(struct {
		Reload bool   `json:"reload"`
		Cursor string `json:"cursor"`
	}{c.Reload, c.Cursor})
	if !writeSSE(w, c.Cursor, "hello", hello) {
		return
	}
	for _, ev := range c.Missed {
		if !writeSSE(w, s.hub.CursorOf(ev), ev.Name, ev.Data) {
			return
		}
	}
	fl.Flush()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-sub.Done():
			return
		case ev := <-sub.C:
			if !writeSSE(w, s.hub.CursorOf(ev), ev.Name, ev.Data) {
				return
			}
			fl.Flush()
		case <-ping.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

// streamOpened and streamClosed count each device's open event streams.
// When its last one closes (the phone slept, lost its network or closed
// the app) it no longer shows any session, so a turn ending there counts
// as unseen. A stream that dies after its replacement opened leaves the
// device's session alone.
func (s *Server) streamOpened(device string) {
	s.mu.Lock()
	s.streams[device]++
	s.mu.Unlock()
}

func (s *Server) streamClosed(device string) {
	s.mu.Lock()
	s.streams[device]--
	last := s.streams[device] <= 0
	if last {
		delete(s.streams, device)
	}
	s.mu.Unlock()
	if last && s.svc != nil && s.svc.Sessions != nil {
		s.svc.Sessions.Showing(device, "")
	}
}

func (s *Server) openStreams(device string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streams[device]
}

func writeSSE(w http.ResponseWriter, id, name string, data []byte) bool {
	_, err := w.Write([]byte("id: " + id + "\nevent: " + name + "\ndata: " + string(data) + "\n\n"))
	return err == nil
}

// uploadTime is the longest an upload may take: 32 MB at well under
// 100 KB/s, a poor mobile connection.
const uploadTime = 10 * time.Minute

// serveUploadPost stores a file sent as the request body (POST
// /upload?name=photo.jpg) and returns what it would attach as.
func (s *Server) serveUploadPost(w http.ResponseWriter, r *http.Request, _ Device) {
	// A body sent slower than this is cut off, not waited on for ever.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(uploadTime))
	up, err := s.svc.SaveUpload(r.URL.Query().Get("name"), r.Body)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(up)
}

// StartPairing opens a pairing code for the window's QR code, replacing
// any open one. Remote access must be on with Tailscale: that's the
// address a device opens.
func (s *Server) StartPairing() (Pairing, error) {
	st := s.Status()
	if st.State != StateOn || st.URL == "" {
		return Pairing{}, errors.New("turn Remote access on, with Tailscale connected, to pair a device")
	}
	return s.pair.start(st.URL)
}

// CancelPairing closes the open code.
func (s *Server) CancelPairing() { s.pair.cancel() }

// AnswerPairing is the window's Allow or Deny for a device waiting to pair.
func (s *Server) AnswerPairing(id string, allow bool) error { return s.pair.answer(id, allow) }

// Devices lists the paired devices.
func (s *Server) Devices() []PairedDevice { return s.devices.List() }

// RevokeDevice unpairs a device, ends its open connections and stops its
// notifications.
func (s *Server) RevokeDevice(id string) error {
	err := s.devices.Revoke(id)
	s.links.revoke(id)
	s.hub.CloseDevice(id)
	if s.push != nil {
		_ = s.push.Forget(id)
	}
	return err
}

// Notify pushes a note about a session to the paired devices that want
// it, while Remote access is on (a device couldn't open the session
// otherwise). The app calls it for every change worth a note.
func (s *Server) Notify(n notify.Note, approval bool) {
	if s.push == nil || !s.Status().Enabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.push.Notify(ctx, push.Note{Title: n.Title, Body: n.Body, Session: n.Key, Tag: n.Key, Approval: approval})
}
