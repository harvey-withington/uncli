// Package hook carries a CLI's hook calls to UNCLI (decision 0012). Some
// CLIs ask about tool calls by running a command (a hook) instead of on
// their own output. UNCLI makes itself that command: in hook mode it reads
// the CLI's JSON on stdin, sends it here over localhost with the token the
// CLI's process was given, waits for the answer and prints it.
//
// The server knows nothing about any CLI: it hands the event name and the
// payload to whoever registered the token, and returns what they answer.
// A hook that can't reach UNCLI prints nothing and fails, which the CLI
// takes as a refusal.
package hook

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Environment the CLI's process is given, which it passes on to its hooks.
const (
	AddrEnv  = "UNCLI_HOOK_ADDR"
	TokenEnv = "UNCLI_HOOK_TOKEN"
)

const maxPayload = 8 << 20

// Handler answers one hook call. ctx ends when the hook gives up (the CLI
// stopped it) or the server closes.
type Handler = func(ctx context.Context, event string, payload []byte) ([]byte, error)

// Server listens on localhost for hook calls.
type Server struct {
	ln  net.Listener
	srv *http.Server

	mu       sync.Mutex
	handlers map[string]Handler
}

// Listen starts a server on a free localhost port.
func Listen() (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln, handlers: map[string]Handler{}}
	s.srv = &http.Server{Handler: http.HandlerFunc(s.serve), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// Addr is where hooks reach the server.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Close stops the server; calls still waiting end with an error.
func (s *Server) Close() error { return s.srv.Close() }

// Register gives a CLI process its own token. The environment returned goes
// into the process; unregister when the process is gone.
func (s *Server) Register(h Handler) (env map[string]string, unregister func()) {
	token := newToken()
	s.mu.Lock()
	s.handlers[token] = h
	s.mu.Unlock()
	return map[string]string{AddrEnv: s.Addr(), TokenEnv: token}, func() {
		s.mu.Lock()
		delete(s.handlers, token)
		s.mu.Unlock()
	}
}

func (s *Server) handler(token string) (Handler, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for t, h := range s.handlers {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			return h, true
		}
	}
	return nil, false
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	event, ok := strings.CutPrefix(r.URL.Path, "/hook/")
	// Only hooks: no browser can send this header without a preflight,
	// which this server never answers.
	if r.Method != http.MethodPost || !ok || event == "" || r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h, ok := s.handler(r.Header.Get("X-UNCLI-Token"))
	if !ok {
		http.Error(w, "unknown token", http.StatusForbidden)
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxPayload))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h(r.Context(), event, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

func newToken() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Run is hook mode: it sends what the CLI wrote on stdin to UNCLI and
// prints the answer. It returns the exit code; anything but 0 (with no
// output) makes the CLI refuse the call.
func Run(event string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if err := run(event, stdin, stdout, getenv); err != nil {
		fmt.Fprintln(stderr, "UNCLI hook:", err)
		return 1
	}
	return 0
}

func run(event string, stdin io.Reader, stdout io.Writer, getenv func(string) string) error {
	addr, token := getenv(AddrEnv), getenv(TokenEnv)
	if addr == "" || token == "" {
		return errors.New("not started by UNCLI")
	}
	if host, _, err := net.SplitHostPort(addr); err != nil || host != "127.0.0.1" {
		return fmt.Errorf("refusing a non-local address %q", addr)
	}
	payload, err := io.ReadAll(io.LimitReader(stdin, maxPayload))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+addr+"/hook/"+event, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UNCLI-Token", token)
	// No timeout: a tool call waits as long as the user takes. The CLI stops
	// the hook at its own limit, which refuses the call.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, maxPayload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(out)))
	}
	_, err = stdout.Write(out)
	return err
}
