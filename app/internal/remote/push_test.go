package remote

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"uncli/internal/app"
	"uncli/internal/notify"
	"uncli/internal/push"
)

// A paired device subscribes its browser over the API, chooses approvals
// only, and is pushed to when its session waits for approval, not when the
// turn finishes; revoked, it's told nothing more.
func TestPushOverHTTP(t *testing.T) {
	exe, _ := os.Executable()
	t.Setenv("UNCLI_FAKE_CLI", "1")
	t.Setenv("UNCLI_CLAUDE_BIN", exe)

	var mu sync.Mutex
	var got []http.Header
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		got = append(got, r.Header.Clone())
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer service.Close()
	pushes := func() int { mu.Lock(); defer mu.Unlock(); return len(got) }

	r := newRigWith(t, Config{Auth: fakeAuth{"tok": {ID: "dev1", Name: "Pixel 9"}}})
	p, err := push.New(r.svc.Store)
	if err != nil {
		t.Fatal(err)
	}
	p.Allowed = func(u *url.URL) bool { return strings.HasPrefix(u.String(), service.URL) }
	r.srv.push = p
	r.svc.SetRemoteNotes(func(n notify.Note, approval bool) { go r.srv.Notify(n, approval) })

	var key string
	if err := r.call(&key, "PushKey"); err != nil || key == "" {
		t.Fatalf("key = %q, %v", key, err)
	}
	k, _ := ecdh.P256().GenerateKey(rand.Reader)
	sub := push.Subscription{Endpoint: service.URL + "/send/abc"}
	sub.Keys.P256dh = base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes())
	sub.Keys.Auth = base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef"))
	if err := r.call(nil, "PushSubscribe", sub); err != nil {
		t.Fatal(err)
	}
	// Another device's endpoint choice can't point the server elsewhere.
	bad := sub
	bad.Endpoint = "http://127.0.0.1:1/internal"
	if err := r.call(nil, "PushSubscribe", bad); err == nil {
		t.Error("an endpoint that isn't a push service was accepted")
	}
	if err := r.call(nil, "SetPushNotifications", push.Approvals); err != nil {
		t.Fatal(err)
	}
	var st push.State
	if err := r.call(&st, "PushState"); err != nil || !st.Subscribed || st.Setting != push.Approvals {
		t.Fatalf("state = %+v, %v", st, err)
	}
	if err := r.call(nil, "TestPush"); err != nil || pushes() != 1 {
		t.Fatalf("test push: %v, %d sent", err, pushes())
	}

	var created app.CreatedSession
	if err := r.call(&created, "CreateSession", "code", t.TempDir(), "haiku", "", "", []string{}); err != nil {
		t.Fatal(err)
	}
	id := created.Session.ID
	if err := r.call(nil, "Send", id, "Push it", []app.AttachmentRef{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the approval push", func() bool { return pushes() == 2 })
	mu.Lock()
	if got[1].Get("Urgency") != "high" || got[1].Get("Topic") == "" {
		t.Errorf("approval push headers = %v", got[1])
	}
	mu.Unlock()
	v, _ := r.svc.Sessions.View(id)
	if err := r.call(nil, "AnswerApproval", id, v.Approvals[0].RequestID, "allow", ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the turn to end", func() bool { v, _ := r.svc.Sessions.View(id); return !v.Busy })
	time.Sleep(300 * time.Millisecond)
	if pushes() != 2 {
		t.Errorf("approvals only, but the finished turn pushed too (%d)", pushes())
	}

	// Revoked: the subscription goes with it. (The rig's device isn't in
	// the paired list, so Revoke itself says so; the push side still goes.)
	_ = r.srv.RevokeDevice("dev1")
	if p.StateOf("dev1").Subscribed {
		t.Error("still subscribed after revoking")
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
