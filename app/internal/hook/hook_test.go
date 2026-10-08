package hook

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	s, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var got string
	env, unregister := s.Register(func(_ context.Context, event string, payload []byte) ([]byte, error) {
		got = event + " " + string(payload)
		return []byte(`{"decision":"allow"}`), nil
	})
	getenv := func(k string) string { return env[k] }

	var out, errOut bytes.Buffer
	if code := Run("pre-tool", strings.NewReader(`{"toolCall":{"name":"view_file"}}`), &out, &errOut, getenv); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if out.String() != `{"decision":"allow"}` || got != `pre-tool {"toolCall":{"name":"view_file"}}` {
		t.Errorf("out = %q, handler saw %q", out.String(), got)
	}

	// Once unregistered, the token no longer works: the hook fails, printing nothing.
	unregister()
	out.Reset()
	if code := Run("pre-tool", strings.NewReader(`{}`), &out, &errOut, getenv); code == 0 || out.Len() != 0 {
		t.Errorf("after unregister: exit %d, out %q", code, out.String())
	}
}

// Every way a hook can fail prints nothing and exits 1, which the CLI
// takes as a refusal (decision 0012).
func TestFailsClosed(t *testing.T) {
	s, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	env, _ := s.Register(func(context.Context, string, []byte) ([]byte, error) { return nil, errors.New("no session") })
	cases := map[string]func(string) string{
		"no environment":    func(string) string { return "" },
		"handler error":     func(k string) string { return env[k] },
		"wrong token":       func(k string) string { return map[string]string{AddrEnv: env[AddrEnv], TokenEnv: "nope"}[k] },
		"non-local address": func(k string) string { return map[string]string{AddrEnv: "10.0.0.1:80", TokenEnv: env[TokenEnv]}[k] },
		"nothing listening": func(k string) string { return map[string]string{AddrEnv: "127.0.0.1:1", TokenEnv: "x"}[k] },
	}
	for name, getenv := range cases {
		var out, errOut bytes.Buffer
		if code := Run("pre-tool", strings.NewReader(`{}`), &out, &errOut, getenv); code != 1 || out.Len() != 0 {
			t.Errorf("%s: exit %d, out %q", name, code, out.String())
		}
	}
}

// A call waits for its answer, and its handler learns when the hook gives up.
func TestWaitsAndCancels(t *testing.T) {
	s, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	gaveUp := make(chan struct{})
	env, _ := s.Register(func(ctx context.Context, _ string, _ []byte) ([]byte, error) {
		<-ctx.Done()
		close(gaveUp)
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+env[AddrEnv]+"/hook/pre-tool", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-UNCLI-Token", env[TokenEnv])
	go func() { _, _ = http.DefaultClient.Do(req) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case <-gaveUp:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler never learned the hook gave up")
	}
}

// Only hook calls are served: no GETs, no other paths, no form posts.
func TestOnlyHookCalls(t *testing.T) {
	s, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	env, _ := s.Register(func(context.Context, string, []byte) ([]byte, error) { return []byte("{}"), nil })
	for _, c := range []struct{ method, path, ctype string }{
		{"GET", "/hook/pre-tool", "application/json"},
		{"POST", "/other", "application/json"},
		{"POST", "/hook/pre-tool", "text/plain"},
	} {
		req, _ := http.NewRequest(c.method, "http://"+env[AddrEnv]+c.path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", c.ctype)
		req.Header.Set("X-UNCLI-Token", env[TokenEnv])
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("%s %s (%s) was served", c.method, c.path, c.ctype)
		}
	}
}
