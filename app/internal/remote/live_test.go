package remote

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveTailnet serves /version with the machine's real tailnet
// certificate and fetches it by name, as a phone would. It needs Tailscale
// running with HTTPS certificates on, so it only runs with
// UNCLI_REMOTE_LIVE=1.
func TestLiveTailnet(t *testing.T) {
	if os.Getenv("UNCLI_REMOTE_LIVE") != "1" {
		t.Skip("set UNCLI_REMOTE_LIVE=1 to serve on the real tailnet")
	}
	s := New(Config{Hub: NewHub(), Certs: &Tailscale{}})
	defer s.Close()
	st, err := s.SetSettings(Settings{Enabled: true, Port: 19871})
	if err != nil || st.State != StateOn {
		t.Fatalf("status = %+v, %v", st, err)
	}
	t.Logf("serving at %s (certificate until %s)", st.URL, st.CertExpires)
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get(st.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil || v.Protocol != Protocol {
		t.Fatalf("version = %+v, %v", v, err)
	}
	// Loopback answers too; the host check refuses a name it doesn't serve.
	resp, err = c.Get(strings.Replace(st.Loopback, "127.0.0.1", "localhost", 1) + "/version")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("loopback = %v, %v", resp, err)
	}
	resp.Body.Close()
	// Nothing on the machine's other addresses (the LAN's).
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.IsLoopback() || ipn.IP.IsLinkLocalUnicast() || strings.HasPrefix(ipn.IP.String(), "100.") || strings.HasPrefix(ipn.IP.String(), "fd7a:") {
			continue
		}
		if dials(net.JoinHostPort(ipn.IP.String(), "19871")) {
			t.Errorf("answering on %s", ipn.IP)
		} else {
			t.Logf("nothing on %s", ipn.IP)
		}
	}
}
