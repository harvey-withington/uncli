// Package netcheck says whether the machine is offline, so a turn can fail
// at once instead of the CLI retrying for a minute and then reporting a
// sign-in failure. It only looks names up: no request leaves the machine
// beyond the DNS query the CLI would make anyway.
package netcheck

import (
	"context"
	"errors"
	"net"
	"os"
	"time"
)

// ErrOffline is what a turn fails with when the machine has no connection.
var ErrOffline = errors.New("you're not connected to the internet; check your connection and send again")

// hosts are looked up together; any answer means the machine is online.
var hosts = []string{"www.google.com", "www.microsoft.com", "www.cloudflare.com"}

const timeout = 2 * time.Second

// Resolver looks a host up (net.DefaultResolver).
type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// Online is nil unless the machine is clearly offline. A slow or uncertain
// answer counts as online: the check never stops a turn that might work.
// Behind a proxy the machine may resolve nothing itself, so it isn't
// checked there.
func Online(ctx context.Context) error {
	for _, v := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if os.Getenv(v) != "" {
			return nil
		}
	}
	return check(ctx, net.DefaultResolver)
}

func check(ctx context.Context, r Resolver) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	errs := make(chan error, len(hosts))
	for _, h := range hosts {
		go func() {
			_, err := r.LookupHost(ctx, h)
			errs <- err
		}()
	}
	for range hosts {
		err := <-errs
		if err == nil || !unreachable(err) {
			return nil
		}
	}
	return ErrOffline
}

// unreachable: the lookup failed for want of a network, not for time.
func unreachable(err error) bool {
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return !dns.IsTimeout && (dns.IsNotFound || dns.IsTemporary)
	}
	var op *net.OpError
	return errors.As(err, &op) && !op.Timeout()
}
