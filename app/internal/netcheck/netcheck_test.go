package netcheck

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fake func(host string) error

func (f fake) LookupHost(_ context.Context, host string) ([]string, error) {
	if err := f(host); err != nil {
		return nil, err
	}
	return []string{"192.0.2.1"}, nil
}

func notFound(h string) error { return &net.DNSError{Err: "no such host", Name: h, IsNotFound: true} }

func TestCheck(t *testing.T) {
	for name, tc := range map[string]struct {
		r       fake
		offline bool
	}{
		"every lookup answers": {fake(func(string) error { return nil }), false},
		"one lookup answers": {fake(func(h string) error {
			if h == hosts[1] {
				return nil
			}
			return notFound(h)
		}), false},
		"no host is found":       {fake(notFound), true},
		"the DNS server is down": {fake(func(h string) error { return &net.DNSError{Name: h, IsTemporary: true} }), true},
		"no route to anything":   {fake(func(string) error { return &net.OpError{Op: "dial", Err: errors.New("network is unreachable")} }), true},
		"lookups time out":       {fake(func(h string) error { return &net.DNSError{Name: h, IsTimeout: true, IsTemporary: true} }), false},
		"something else fails":   {fake(func(string) error { return errors.New("odd") }), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := check(context.Background(), tc.r)
			if tc.offline != errors.Is(err, ErrOffline) {
				t.Fatalf("offline = %v, want %v", err, tc.offline)
			}
		})
	}
}

// A proxy may do the machine's lookups for it, so it isn't checked there.
func TestOnlineSkipsTheCheckBehindAProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:8080")
	if err := Online(context.Background()); err != nil {
		t.Fatal(err)
	}
}
