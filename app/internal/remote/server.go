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
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"uncli/internal/app"
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
	// OnStatus is told whenever the status changes (the bridge passes it
	// on to the window).
	OnStatus func(Status)
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

	onStatus func(Status)

	mu       sync.Mutex
	settings Settings
	status   Status
	net      *network // the listeners while on
}

func New(c Config) *Server {
	s := &Server{api: c.API, svc: c.Service, hub: c.Hub, store: c.Store, certs: c.Certs, auth: c.Auth,
		links: newLinks(), pwa: c.PWA, onStatus: c.OnStatus}
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
	if s.pwa != nil {
		mux.Handle("GET /m/", http.StripPrefix("/m/", http.FileServerFS(s.pwa)))
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

// serveUploadPost stores a file sent as the request body (POST
// /upload?name=photo.jpg) and returns what it would attach as.
func (s *Server) serveUploadPost(w http.ResponseWriter, r *http.Request, _ Device) {
	up, err := s.svc.SaveUpload(r.URL.Query().Get("name"), r.Body)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(up)
}
