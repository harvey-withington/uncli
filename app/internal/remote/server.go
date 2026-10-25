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

	"uncli/internal/app"
	"uncli/internal/notify"
	"uncli/internal/push"
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
	net      *network // the listeners while on
}

func New(c Config) *Server {
	s := &Server{api: c.API, svc: c.Service, hub: c.Hub, store: c.Store, certs: c.Certs, auth: c.Auth,
		links: newLinks(), pwa: c.PWA, push: c.Push, onStatus: c.OnStatus}
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", serveVersion)
	mux.Handle("POST /rpc", s.authed(func(w http.ResponseWriter, r *http.Request, d Device) {
		serveRPC(w, r, &client{API: s.api, svc: s.svc, server: s, device: d}, rpcNames)
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
	return guard(hosts, mux)
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
