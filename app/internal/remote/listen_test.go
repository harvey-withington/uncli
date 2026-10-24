package remote

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func dials(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		c.Close()
	}
	return err == nil
}

// Remote access listens on loopback and the tailnet address only, follows
// Tailscale, and stops listening altogether when turned off.
func TestListening(t *testing.T) {
	r := newRig(t)
	loop, tail, other := fmt.Sprintf("127.0.0.1:%d", r.port), fmt.Sprintf("127.0.0.2:%d", r.port), fmt.Sprintf("127.0.0.3:%d", r.port)
	st := r.srv.Status()
	if st.URL != fmt.Sprintf("https://%s:%d", testName, r.port) || st.Loopback != "http://"+loop || st.CertExpires.IsZero() {
		t.Errorf("status = %+v", st)
	}
	if !dials(loop) || !dials(tail) {
		t.Error("not listening on loopback and the tailnet")
	}
	// Any other address (as the LAN's would be) gets nothing: there's no
	// listener on every address.
	if dials(other) {
		t.Error("listening beyond loopback and the tailnet")
	}

	// Tailscale stops: the tailnet listener goes, loopback stays, and the
	// status says why.
	r.certs.set(&Problem{Reason: ReasonStopped, Detail: "Stopped"})
	r.srv.mu.Lock()
	n := r.srv.net
	r.srv.mu.Unlock()
	r.srv.follow(context.Background(), n)
	if st := r.srv.Status(); st.State != StateLoopback || st.Reason != ReasonStopped || st.URL != "" {
		t.Errorf("status = %+v", st)
	}
	if dials(tail) || !dials(loop) {
		t.Error("the tailnet listener didn't follow Tailscale")
	}
	r.certs.set(nil)
	r.srv.follow(context.Background(), n)
	if st := r.srv.Status(); st.State != StateOn || !dials(tail) {
		t.Errorf("back on: %+v", st)
	}

	// Off: nothing answers anywhere, and it stays off after a restart.
	if st, err := r.srv.SetSettings(Settings{Enabled: false, Port: r.port}); err != nil || st.State != StateOff {
		t.Fatalf("off = %+v, %v", st, err)
	}
	if dials(loop) || dials(tail) {
		t.Error("still listening while off")
	}
	again := New(Config{Service: r.svc, Hub: NewHub(), Store: r.svc.Store, Certs: r.certs})
	if st := again.Start(); st.State != StateOff || st.Port != r.port {
		t.Errorf("after a restart = %+v", st)
	}
}

func TestPortInUse(t *testing.T) {
	r := newRig(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port
	st, err := r.srv.SetSettings(Settings{Enabled: true, Port: busy})
	if err != nil || st.State != StateError || st.Reason != ReasonListen {
		t.Errorf("status = %+v, %v", st, err)
	}
	if _, err := r.srv.SetSettings(Settings{Enabled: true, Port: 70000}); err == nil {
		t.Error("accepted port 70000")
	}
}
