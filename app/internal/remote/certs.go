package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"tailscale.com/client/local"
)

// The machine's tailnet name, addresses and certificate come from the
// Tailscale app already running, through its local API (decision 0021;
// the 2026-10-24 spike showed a tray app running as the user can fetch
// the certificate). The source is an interface so another one (tests, or
// a `tailscale serve` mode) can stand in.

// Tailnet is this machine on the tailnet.
type Tailnet struct {
	Name string       // e.g. deviant.tail2ebd58.ts.net
	IPs  []netip.Addr // its 100.x and fd7a: addresses
}

// CertSource finds the machine on the tailnet and fetches its certificate.
type CertSource interface {
	Lookup(ctx context.Context) (Tailnet, error)
	Cert(ctx context.Context, name string) (*tls.Certificate, error)
}

// Why there's no tailnet address or certificate, as Settings explains it.
const (
	ReasonNoTailscale = "no-tailscale" // the Tailscale app isn't installed or running
	ReasonStopped     = "stopped"      // Tailscale is installed but not connected
	ReasonHTTPSOff    = "https-off"    // HTTPS certificates are off in the tailnet
	ReasonCertFailed  = "cert-failed"  // fetching the certificate failed (Detail says how)
	ReasonListen      = "listen"       // the port couldn't be opened (Detail says why)
)

// Problem is a reason with the detail behind it.
type Problem struct {
	Reason string
	Detail string
}

func (p *Problem) Error() string {
	if p.Detail == "" {
		return p.Reason
	}
	return p.Reason + ": " + p.Detail
}

// Tailscale is the CertSource backed by the running Tailscale app.
type Tailscale struct{ lc local.Client }

func (t *Tailscale) Lookup(ctx context.Context) (Tailnet, error) {
	st, err := t.lc.StatusWithoutPeers(ctx)
	if err != nil {
		return Tailnet{}, &Problem{Reason: ReasonNoTailscale, Detail: err.Error()}
	}
	if st.BackendState != "Running" || st.Self == nil {
		return Tailnet{}, &Problem{Reason: ReasonStopped, Detail: st.BackendState}
	}
	name := strings.TrimSuffix(st.Self.DNSName, ".")
	if len(st.CertDomains) == 0 || name == "" {
		return Tailnet{}, &Problem{Reason: ReasonHTTPSOff}
	}
	return Tailnet{Name: name, IPs: st.Self.TailscaleIPs}, nil
}

func (t *Tailscale) Cert(ctx context.Context, name string) (*tls.Certificate, error) {
	certPEM, keyPEM, err := t.lc.CertPair(ctx, name)
	if err != nil {
		return nil, &Problem{Reason: ReasonCertFailed, Detail: err.Error()}
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, &Problem{Reason: ReasonCertFailed, Detail: fmt.Sprintf("unreadable certificate: %v", err)}
	}
	return &pair, nil
}

// problemOf is err as a Problem, with reason as the fallback.
func problemOf(err error, reason string) *Problem {
	var p *Problem
	if errors.As(err, &p) {
		return p
	}
	return &Problem{Reason: reason, Detail: err.Error()}
}
