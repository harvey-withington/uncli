package oauthcatch

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCatch(t *testing.T) {
	c, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !strings.HasPrefix(c.URL, "http://127.0.0.1:") || !strings.HasSuffix(c.URL, "/callback") {
		t.Fatalf("URL = %s", c.URL)
	}
	// No code: refused, and not taken.
	if r, err := http.Get(c.URL + "?state=s"); err != nil || r.StatusCode != http.StatusBadRequest {
		t.Fatalf("no code: %v %v", r, err)
	}
	r, err := http.Get(c.URL + "?code=abc&state=s1")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(page), "You can close this tab") {
		t.Errorf("page = %s", page)
	}
	// A second visit (a reload) doesn't replace the first.
	_, _ = http.Get(c.URL + "?code=other&state=s2")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := c.Wait(ctx)
	if err != nil || got != c.URL+"?code=abc&state=s1" {
		t.Errorf("got %q %v", got, err)
	}
}

func TestCatchError(t *testing.T) {
	c, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	r, _ := http.Get(c.URL + "?error=access_denied&error_description=No+thanks&state=s")
	page, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(page), "Sign-in failed") || !strings.Contains(string(page), "access_denied No thanks") {
		t.Errorf("page = %s", page)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if got, _ := c.Wait(ctx); !strings.Contains(got, "error=access_denied") {
		t.Errorf("a refusal is handed on too, so the CLI can say so: %q", got)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := c.Wait(ctx2); err != ErrClosed {
		t.Errorf("cancelled: %v", err)
	}
}
