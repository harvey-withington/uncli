// Package oauthcatch catches one OAuth redirect on this machine: the
// browser comes back to http://127.0.0.1:<port>/callback?code=…&state=…,
// and the address is handed on (decision 0020). It's for CLIs whose own
// listener the browser can't reach, such as one in WSL or a container.
package oauthcatch

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"sync"
	"time"
)

// Catcher listens for one redirect.
type Catcher struct {
	URL  string // the redirect to ask for
	got  chan string
	once sync.Once
	srv  *http.Server
}

// Start listens on a free port of 127.0.0.1.
func Start() (*Catcher, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	c := &Catcher{URL: fmt.Sprintf("http://127.0.0.1:%d/callback", l.Addr().(*net.TCPAddr).Port), got: make(chan string, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		title, body := "Signed in", "You can close this tab and go back to UNCLI."
		if e := q.Get("error"); e != "" {
			title, body = "Sign-in failed", "The server said: "+e+" "+q.Get("error_description")
		} else if q.Get("code") == "" {
			http.Error(w, "No sign-in code here.", http.StatusBadRequest)
			return
		}
		c.once.Do(func() { c.got <- "http://" + r.Host + r.URL.RequestURI() })
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>%[1]s</title><body style="font:16px system-ui;margin:15vh auto;max-width:28em;text-align:center"><h1 style="font-size:22px">%[1]s</h1><p>%[2]s</p>`,
			html.EscapeString(title), html.EscapeString(body))
	})
	c.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = c.srv.Serve(l) }()
	return c, nil
}

// ErrClosed: the catcher was closed before the browser came back.
var ErrClosed = errors.New("stopped waiting for the sign-in")

// Wait returns the address the browser came back to.
func (c *Catcher) Wait(ctx context.Context) (string, error) {
	select {
	case u := <-c.got:
		return u, nil
	case <-ctx.Done():
		return "", ErrClosed
	}
}

// Close stops listening.
func (c *Catcher) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.srv.Shutdown(ctx)
}
