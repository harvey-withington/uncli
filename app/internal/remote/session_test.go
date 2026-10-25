package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"uncli/internal/app"
	"uncli/internal/session"
	"uncli/internal/store"
)

// A paired device drives a session over HTTPS from start to finish: it
// creates the session, sends a message, loses its connection while the
// turn waits on an approval, reconnects and catches up on what it missed,
// answers the approval, and sees the turn finish, with the trace saying
// which device answered.
func TestSessionOverHTTP(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNCLI_FAKE_CLI", "1")
	t.Setenv("UNCLI_CLAUDE_BIN", exe)
	r := newRig(t)

	ctx, cancel := context.WithCancel(context.Background())
	events := r.stream(ctx, "")
	hello := next(t, events, "hello", func(e sseEvent) bool { return e.Name == "hello" })
	if !strings.Contains(hello.Data, `"reload":true`) {
		t.Errorf("a new client should reload first: %s", hello.Data)
	}

	var boot app.Bootstrap
	if err := r.call(&boot, "Bootstrap"); err != nil || len(boot.Profiles) == 0 {
		t.Fatalf("Bootstrap: %v", err)
	}
	var created app.CreatedSession
	if err := r.call(&created, "CreateSession", "code", t.TempDir(), "haiku", "", "", []string{}); err != nil {
		t.Fatal(err)
	}
	id := created.Session.ID
	if err := r.call(nil, "Send", id, "Push it", []app.AttachmentRef{}); err != nil {
		t.Fatal(err)
	}
	// The stream shows the session waiting; the client keeps its cursor.
	waiting := next(t, events, "an approval", func(e sseEvent) bool {
		return e.Name == "session:changed" && strings.Contains(e.Data, `"state":"needs_approval"`)
	})
	cancel() // the phone goes to sleep

	// While it's away the window's view of the session keeps going: the
	// approval waits. Back, it resumes from its cursor and is told nothing
	// it already had.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	events = r.stream(ctx2, waiting.ID)
	hello = next(t, events, "hello", func(e sseEvent) bool { return e.Name == "hello" })
	if strings.Contains(hello.Data, `"reload":true`) {
		t.Errorf("resuming from a recent cursor shouldn't reload: %s", hello.Data)
	}
	var v session.View
	if err := json.Unmarshal([]byte(waiting.Data), &v); err != nil || len(v.Approvals) != 1 {
		t.Fatalf("waiting view = %s (%v)", waiting.Data, err)
	}

	if err := r.call(nil, "AnswerApproval", id, v.Approvals[0].RequestID, "allow", ""); err != nil {
		t.Fatal(err)
	}
	next(t, events, "the finished page", func(e sseEvent) bool {
		return e.Name == "page:changed" && strings.Contains(e.Data, `"status":"done"`)
	})
	var pages []store.Page
	if err := r.call(&pages, "Pages", id); err != nil || len(pages) != 1 {
		t.Fatalf("pages = %v, %v", pages, err)
	}
	p := pages[0]
	if !strings.Contains(p.AnswerMD, "Pushed.") || len(p.Trace) != 1 || p.Trace[0].Approved != "you" || p.Trace[0].Device != "Pixel 9" {
		t.Errorf("page = %q, trace %+v", p.AnswerMD, p.Trace)
	}

	// The same session is the window's too: the app sees what the device did.
	if got, _ := r.svc.Sessions.View(id); got.State == session.NeedsApproval {
		t.Errorf("still waiting: %+v", got)
	}
}

// What a device can't do or reach.
func TestRefusals(t *testing.T) {
	r := newRig(t)
	post := func(token, origin, host, body string) int {
		t.Helper()
		req, _ := http.NewRequest("POST", r.base()+"/rpc", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := r.https.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	call := `{"jsonrpc":"2.0","id":1,"method":"Bootstrap","params":[]}`
	if c := post("tok", "", "", call); c != http.StatusOK {
		t.Errorf("paired = %d", c)
	}
	if c := post("", "", "", call); c != http.StatusUnauthorized {
		t.Errorf("no token = %d", c)
	}
	if c := post("nope", "", "", call); c != http.StatusUnauthorized {
		t.Errorf("unknown token = %d", c)
	}
	// A token in the URL is never read.
	req, _ := http.NewRequest("POST", r.base()+"/rpc?token=tok", strings.NewReader(call))
	if resp, err := r.https.Do(req); err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("token in URL = %v, %v", resp.StatusCode, err)
	}
	if c := post("tok", "https://evil.example", "", call); c != http.StatusForbidden {
		t.Errorf("other origin = %d", c)
	}
	if c := post("tok", "", "evil.example:1234", call); c != http.StatusMisdirectedRequest {
		t.Errorf("other host = %d", c)
	}
	// Methods kept for the server's own screen.
	for _, m := range []string{"OpenFolder", "SetPreferences", "SetDecisionKey"} {
		if err := r.call(nil, m, "x"); err == nil || !strings.Contains(err.Error(), "doesn't offer") {
			t.Errorf("%s = %v", m, err)
		}
	}
	// A path on the server isn't an attachment a device can send.
	var created app.CreatedSession
	if err := r.call(&created, "CreateSession", "chat", "", "haiku", "", "", []string{}); err != nil {
		t.Fatal(err)
	}
	err := r.call(nil, "Send", created.Session.ID, "hi", []app.AttachmentRef{{Path: `C:\Windows\win.ini`}})
	if err == nil || !strings.Contains(err.Error(), "uploading") {
		t.Errorf("path attachment = %v", err)
	}
	// Nor is a network path a folder it can browse or start a session in.
	for _, p := range []string{`\\attacker\share`, "//attacker/share"} {
		if err := r.call(nil, "ListFolder", p); err == nil || !strings.Contains(err.Error(), "network path") {
			t.Errorf("ListFolder(%s) = %v", p, err)
		}
		if err := r.call(nil, "CreateSession", "code", p, "haiku", "", "", []string{}); err == nil || !strings.Contains(err.Error(), "network path") {
			t.Errorf("CreateSession(%s) = %v", p, err)
		}
	}
}

func TestVersion(t *testing.T) {
	r := newRig(t)
	resp, err := r.https.Get(r.base() + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil || v.Protocol != Protocol || len(v.Capabilities) == 0 {
		t.Errorf("version = %+v, %v", v, err)
	}
}

// Uploads become attachments by id, and a signed link opens one for a
// while, on its own route only.
func TestUploadsAndLinks(t *testing.T) {
	r := newRig(t)
	req, _ := http.NewRequest("POST", r.base()+"/upload?name=..%5Cnotes.txt", bytes.NewReader([]byte("hello")))
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := r.https.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var up app.Upload
	_ = json.NewDecoder(resp.Body).Decode(&up)
	resp.Body.Close()
	if up.ID == "" || up.Name != "notes.txt" || up.Kind != "text" {
		t.Fatalf("upload = %+v", up)
	}
	var link string
	if err := r.call(&link, "UploadLink", up.ID); err != nil || !strings.HasPrefix(link, "/l/upload?") {
		t.Fatalf("link = %q, %v", link, err)
	}
	get := func(u string) (int, string) {
		resp, err := r.https.Get(r.base() + u)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		return resp.StatusCode, b.String()
	}
	if code, body := get(link); code != http.StatusOK || body != "hello" {
		t.Errorf("link = %d %q", code, body)
	}
	if code, _ := get(strings.Replace(link, "/l/upload", "/l/artifact", 1)); code != http.StatusForbidden {
		t.Errorf("the link on another route = %d", code)
	}
	if code, _ := get(strings.Replace(link, "e=", "e=1", 1)); code != http.StatusForbidden {
		t.Errorf("a changed expiry = %d", code)
	}
	if code, _ := get(strings.Replace(link, "d=dev1", "d=dev2", 1)); code != http.StatusForbidden {
		t.Errorf("the link for another device = %d", code)
	}
	// Revoking the device ends its links at once.
	_ = r.srv.RevokeDevice("dev1")
	if code, _ := get(link); code != http.StatusForbidden {
		t.Errorf("a revoked device's link = %d", code)
	}
	r.srv.links.revoked = map[string]bool{}
	r.srv.links.now = func() time.Time { return time.Now().Add(linkLife + time.Minute) }
	if code, _ := get(link); code != http.StatusForbidden {
		t.Errorf("an expired link = %d", code)
	}
}
