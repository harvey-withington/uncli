package remote

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"uncli/internal/app"
)

// A paired device for tests: one token, one device.
type fakeAuth map[string]Device

func (f fakeAuth) Authenticate(token string) (Device, bool) {
	d, ok := f[token]
	return d, ok
}

const testName = "test.tail0000.ts.net"

// fakeCerts plays Tailscale: the machine is on the tailnet at 127.0.0.2
// (a loopback address standing in for the 100.x one) with a self-signed
// certificate, or not, when err is set.
type fakeCerts struct {
	mu   sync.Mutex
	err  error
	ips  []netip.Addr
	cert *tls.Certificate
	pool *x509.CertPool
}

func newFakeCerts(t *testing.T) *fakeCerts {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: testName},
		DNSNames: []string{testName}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(90 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return &fakeCerts{ips: []netip.Addr{netip.MustParseAddr("127.0.0.2")},
		cert: &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool: pool}
}

func (f *fakeCerts) Lookup(context.Context) (Tailnet, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Tailnet{}, f.err
	}
	return Tailnet{Name: testName, IPs: f.ips}, nil
}

func (f *fakeCerts) Cert(context.Context, string) (*tls.Certificate, error) { return f.cert, nil }

func (f *fakeCerts) set(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

// freePort finds a port free on both loopback addresses.
func freePort(t *testing.T) int {
	t.Helper()
	for i := 0; i < 20; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()
		if ln2, err := net.Listen("tcp", fmt.Sprintf("127.0.0.2:%d", port)); err == nil {
			ln2.Close()
			return port
		}
	}
	t.Fatal("no free port")
	return 0
}

type rig struct {
	t     *testing.T
	svc   *app.Service
	srv   *Server
	certs *fakeCerts
	port  int
	https *http.Client // reaches the tailnet listener by name
}

// newRig is a real app service (on a temp data folder) with Remote access
// on: loopback and the fake tailnet address, one paired device ("tok").
func newRig(t *testing.T) *rig {
	t.Helper()
	return newRigWith(t, Config{Auth: fakeAuth{"tok": {ID: "dev1", Name: "Pixel 9"}}})
}

// newRigWith takes the Config's Auth and callbacks; nil Auth is the real
// paired-device list.
func newRigWith(t *testing.T, c Config) *rig {
	t.Helper()
	dir := t.TempDir()
	hub := NewHub()
	svc, err := app.New(app.Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, hub)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	r := &rig{t: t, svc: svc, certs: newFakeCerts(t), port: freePort(t)}
	c.API, c.Service, c.Hub, c.Store, c.Certs = app.NewAPI(svc, nil), svc, hub, svc.Store, r.certs
	r.srv = New(c)
	t.Cleanup(r.srv.Close)
	if st, err := r.srv.SetSettings(Settings{Enabled: true, Port: r.port}); err != nil || st.State != StateOn {
		t.Fatalf("status = %+v, %v", st, err)
	}
	r.https = &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: r.certs.pool, ServerName: testName},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, _ := net.SplitHostPort(addr)
			return (&net.Dialer{}).DialContext(ctx, network, "127.0.0.2:"+port)
		},
	}}
	return r
}

func (r *rig) base() string { return fmt.Sprintf("https://%s:%d", testName, r.port) }

// call makes an RPC as the paired device and decodes its result into out.
func (r *rig) call(out any, method string, params ...any) error {
	r.t.Helper()
	if params == nil {
		params = []any{}
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest("POST", r.base()+"/rpc", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", r.base())
	resp, err := r.https.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	var res struct {
		Result json.RawMessage
		Error  *rpcError
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		r.t.Fatalf("%s: %v", method, err)
	}
	if res.Error != nil {
		return fmt.Errorf("%s", res.Error.Message)
	}
	if out != nil {
		if err := json.Unmarshal(res.Result, out); err != nil {
			r.t.Fatalf("%s result: %v", method, err)
		}
	}
	return nil
}

type sseEvent struct{ ID, Name, Data string }

// stream reads the event stream, from cursor, into a channel.
func (r *rig) stream(ctx context.Context, cursor string) <-chan sseEvent {
	r.t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", r.base()+"/events", nil)
	req.Header.Set("Authorization", "Bearer tok")
	if cursor != "" {
		req.Header.Set("Last-Event-ID", cursor)
	}
	resp, err := (&http.Client{Transport: r.https.Transport}).Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		r.t.Fatalf("events: %s", resp.Status)
	}
	ch := make(chan sseEvent, 4096)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		var ev sseEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if ev.Name != "" {
					ch <- ev
				}
				ev = sseEvent{}
			case strings.HasPrefix(line, "id: "):
				ev.ID = line[4:]
			case strings.HasPrefix(line, "event: "):
				ev.Name = line[7:]
			case strings.HasPrefix(line, "data: "):
				ev.Data = line[6:]
			}
		}
	}()
	return ch
}

// next waits for the first event matching ok.
func next(t *testing.T, ch <-chan sseEvent, what string, ok func(sseEvent) bool) sseEvent {
	t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case ev, open := <-ch:
			if !open {
				t.Fatalf("stream ended waiting for %s", what)
			}
			if ok(ev) {
				return ev
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// The fake CLI is the test binary itself, run as Claude Code by the
// session (UNCLI_CLAUDE_BIN), speaking the stream-json shapes recorded in
// testdata/streams/claude/2.1.285 (perm-stdio-allow). A message makes it
// ask to push; the push waits for UNCLI's answer, then the turn ends.
func TestMain(m *testing.M) {
	if os.Getenv("UNCLI_FAKE_CLI") == "1" {
		fakeCLI()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeCLI() {
	out := bufio.NewWriter(os.Stdout)
	emit := func(v any) {
		b, _ := json.Marshal(v)
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
	assistant := func(content ...any) map[string]any {
		return map[string]any{"type": "assistant", "session_id": "fake-1",
			"message": map[string]any{"model": "claude-haiku-4-5", "role": "assistant", "content": content}}
	}
	push := map[string]any{"command": "git push origin main"}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var in struct {
			Type      string `json:"type"`
			RequestID string `json:"request_id"`
			Response  struct {
				RequestID string `json:"request_id"`
				Response  struct {
					Behavior string `json:"behavior"`
				} `json:"response"`
			} `json:"response"`
		}
		if json.Unmarshal(sc.Bytes(), &in) != nil {
			continue
		}
		switch in.Type {
		case "control_request":
			emit(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": in.RequestID, "response": map[string]any{}}})
		case "user":
			emit(map[string]any{"type": "system", "subtype": "init", "session_id": "fake-1", "model": "claude-haiku-4-5", "tools": []string{"Bash"}})
			emit(assistant(map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Bash", "input": push}))
			emit(map[string]any{"type": "control_request", "request_id": "perm-1", "request": map[string]any{"subtype": "can_use_tool",
				"tool_name": "Bash", "input": push, "tool_use_id": "toolu_1", "description": "Push"}})
		case "control_response":
			if in.Response.RequestID != "perm-1" {
				continue
			}
			text := "Pushed."
			if in.Response.Response.Behavior != "allow" {
				text = "Not pushed."
			}
			emit(map[string]any{"type": "user", "message": map[string]any{"role": "user",
				"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "ok"}}}})
			emit(assistant(map[string]any{"type": "text", "text": text}))
			emit(map[string]any{"type": "result", "subtype": "success", "is_error": false, "duration_ms": 10, "num_turns": 2, "result": text,
				"session_id": "fake-1", "total_cost_usd": 0.001, "usage": map[string]any{"input_tokens": 5, "output_tokens": 5}})
		}
	}
}
