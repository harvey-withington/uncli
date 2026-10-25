package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type memStore map[string]string

func (m memStore) Setting(k string) (string, error) { return m[k], nil }
func (m memStore) SetSetting(k, v string) error     { m[k] = v; return nil }

// browser plays a device's browser: its subscription keys, and reading
// what its push service received the way the browser would (RFC 8291).
type browser struct {
	key  *ecdh.PrivateKey
	auth []byte
}

func newBrowser(t *testing.T) *browser {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := make([]byte, 16)
	_, _ = rand.Read(a)
	return &browser{key: k, auth: a}
}

func (b *browser) sub(endpoint string) Subscription {
	var s Subscription
	s.Endpoint = endpoint
	s.Keys.P256dh = base64.RawURLEncoding.EncodeToString(b.key.PublicKey().Bytes())
	s.Keys.Auth = base64.RawURLEncoding.EncodeToString(b.auth)
	return s
}

func hmacOf(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// decrypt opens an aes128gcm push message body.
func (b *browser) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	server := body[21 : 21+idlen]
	ct := body[21+idlen:]
	if rs < 18 {
		t.Fatalf("record size %d", rs)
	}
	pub, err := ecdh.P256().NewPublicKey(server)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := b.key.ECDH(pub)
	if err != nil {
		t.Fatal(err)
	}
	info := append(append([]byte("WebPush: info\x00"), b.key.PublicKey().Bytes()...), server...)
	ikm := hmacOf(hmacOf(b.auth, shared), append(info, 1))
	prk := hmacOf(salt, ikm)
	cek := hmacOf(prk, []byte("Content-Encoding: aes128gcm\x00\x01"))[:16]
	nonce := hmacOf(prk, []byte("Content-Encoding: nonce\x00\x01"))[:12]
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	// Strip the padding: the last record ends with 0x02 then zeros.
	plain = bytes.TrimRight(plain, "\x00")
	if len(plain) == 0 || plain[len(plain)-1] != 2 {
		t.Fatalf("no padding delimiter")
	}
	return plain[:len(plain)-1]
}

// service is a fake push service: it records what arrives, and answers
// with status.
type service struct {
	mu     sync.Mutex
	got    [][]byte
	hdr    []http.Header
	status int
}

func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.got = append(s.got, b)
	s.hdr = append(s.hdr, r.Header.Clone())
	code := s.status
	s.mu.Unlock()
	if code == 0 {
		code = http.StatusCreated
	}
	w.WriteHeader(code)
}

func newPusher(t *testing.T, srv *httptest.Server) *Pusher {
	t.Helper()
	p, err := New(memStore{})
	if err != nil {
		t.Fatal(err)
	}
	p.client = srv.Client()
	p.Allowed = func(u *url.URL) bool { return strings.HasPrefix(u.String(), srv.URL) }
	return p
}

func TestNotify(t *testing.T) {
	svc := &service{}
	srv := httptest.NewTLSServer(svc)
	defer srv.Close()
	p := newPusher(t, srv)
	phone, tablet := newBrowser(t), newBrowser(t)
	if err := p.Subscribe("phone", phone.sub(srv.URL+"/phone")); err != nil {
		t.Fatal(err)
	}
	if err := p.Subscribe("tablet", tablet.sub(srv.URL+"/tablet")); err != nil {
		t.Fatal(err)
	}
	if err := p.SetSetting("tablet", Approvals); err != nil {
		t.Fatal(err)
	}

	// Finished: only the phone (the tablet wants approvals only).
	p.Notify(context.Background(), Note{Title: "Fix the parser", Body: "Finished: all green", Session: "s1", Tag: "s1"})
	if len(svc.got) != 1 {
		t.Fatalf("%d pushes", len(svc.got))
	}
	var n Note
	if err := json.Unmarshal(phone.decrypt(t, svc.got[0]), &n); err != nil || n.Title != "Fix the parser" || n.Session != "s1" {
		t.Fatalf("note = %+v, %v", n, err)
	}
	h := svc.hdr[0]
	if h.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(h.Get("Authorization"), "vapid t=") || h.Get("TTL") == "" || h.Get("Topic") != "s1" {
		t.Errorf("headers = %v", h)
	}
	// Needs approval: both, urgently.
	p.Notify(context.Background(), Note{Title: "Fix the parser", Body: "Needs your approval: git push", Session: "s1", Tag: "s1", Approval: true})
	if len(svc.got) != 3 || svc.hdr[1].Get("Urgency") != "high" {
		t.Fatalf("%d pushes, urgency %q", len(svc.got), svc.hdr[1].Get("Urgency"))
	}
	// Off: nothing.
	_ = p.SetSetting("phone", Off)
	_ = p.SetSetting("tablet", Off)
	p.Notify(context.Background(), Note{Title: "x", Approval: true})
	if len(svc.got) != 3 {
		t.Errorf("sent while off")
	}
	// The setting survives a new subscription from the same device.
	_ = p.Subscribe("phone", phone.sub(srv.URL+"/phone2"))
	if st := p.StateOf("phone"); !st.Subscribed || st.Setting != Off {
		t.Errorf("state = %+v", st)
	}
}

func TestGoneSubscriptionsAreForgotten(t *testing.T) {
	svc := &service{status: http.StatusGone}
	srv := httptest.NewTLSServer(svc)
	defer srv.Close()
	p := newPusher(t, srv)
	_ = p.Subscribe("phone", newBrowser(t).sub(srv.URL+"/x"))
	p.Notify(context.Background(), Note{Title: "x"})
	if p.StateOf("phone").Subscribed {
		t.Error("a gone subscription was kept")
	}
	svc.status = http.StatusInternalServerError
	_ = p.Subscribe("phone", newBrowser(t).sub(srv.URL+"/x"))
	if err := p.Test(context.Background(), "phone", Note{Title: "Test"}); err == nil {
		t.Error("a failing push service went unreported")
	}
	if st := p.StateOf("phone"); !st.Subscribed || !strings.Contains(st.LastFail, "500") {
		t.Errorf("state = %+v", st)
	}
}

// The push service saying an old subscription is gone doesn't drop the
// one the device made meanwhile.
func TestGoneKeepsANewerSubscription(t *testing.T) {
	svc := &service{status: http.StatusGone}
	srv := httptest.NewTLSServer(svc)
	defer srv.Close()
	p := newPusher(t, srv)
	b := newBrowser(t)
	_ = p.Subscribe("phone", b.sub(srv.URL+"/old"))
	p.mu.Lock()
	old := p.records[0]
	p.mu.Unlock()
	_ = p.Subscribe("phone", b.sub(srv.URL+"/new"))
	_ = p.deliver(context.Background(), old, Note{Title: "x"})
	if !p.StateOf("phone").Subscribed {
		t.Error("the newer subscription went with the old one")
	}
}

func TestRetain(t *testing.T) {
	srv := httptest.NewTLSServer(&service{})
	defer srv.Close()
	p := newPusher(t, srv)
	_ = p.Subscribe("phone", newBrowser(t).sub(srv.URL+"/a"))
	_ = p.Subscribe("revoked", newBrowser(t).sub(srv.URL+"/b"))
	if err := p.Retain(func(d string) bool { return d == "phone" }); err != nil {
		t.Fatal(err)
	}
	if !p.StateOf("phone").Subscribed || p.StateOf("revoked").Subscribed {
		t.Errorf("phone %+v, revoked %+v", p.StateOf("phone"), p.StateOf("revoked"))
	}
}

func TestKeysAndEndpoints(t *testing.T) {
	st := memStore{}
	p1, _ := New(st)
	p2, _ := New(st)
	if p1.PublicKey() == "" || p1.PublicKey() != p2.PublicKey() {
		t.Error("keys aren't made once and kept")
	}
	for raw, ok := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":           true,
		"https://updates.push.services.mozilla.com/wpush/x": true,
		"https://wns2-par02p.notify.windows.com/w/?token=x": true,
		"https://web.push.apple.com/abc":                    true,
		"http://fcm.googleapis.com/fcm/send/abc":            false,
		"https://127.0.0.1/x":                               false,
		"https://ripped.tail2ebd58.ts.net/x":                false,
		"https://fcm.googleapis.com.evil.example/x":         false,
	} {
		u, _ := url.Parse(raw)
		if KnownService(u) != ok {
			t.Errorf("%s allowed = %v", raw, !ok)
		}
	}
	if err := p1.Subscribe("d", Subscription{Endpoint: "https://127.0.0.1:8080/x"}); err == nil {
		t.Error("a local endpoint was accepted")
	}
	if topicOf("deviant.ts.net:9871~a1b2-c3") != "devianttsnet9871a1b2-c3" {
		t.Errorf("topic = %q", topicOf("deviant.ts.net:9871~a1b2-c3"))
	}
}
