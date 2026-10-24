package remote

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func (r *rig) post(path string, body any) (int, map[string]any) {
	r.t.Helper()
	b, _ := json.Marshal(body)
	resp, err := r.https.Post(r.base()+path, "application/json", bytes.NewReader(b))
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (r *rig) rpcAs(token, method string) int {
	r.t.Helper()
	req, _ := http.NewRequest("POST", r.base()+"/rpc", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":[]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.https.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// A phone pairs only with Allow at the server, gets a token of its own
// (stored hashed), and loses access the moment it's revoked.
func TestPairing(t *testing.T) {
	var mu sync.Mutex
	var asked []PairRequest
	var lists [][]PairedDevice
	r := newRigWith(t, Config{
		OnPairRequest: func(p PairRequest) { mu.Lock(); asked = append(asked, p); mu.Unlock() },
		OnDevices:     func(d []PairedDevice) { mu.Lock(); lists = append(lists, d); mu.Unlock() },
	})
	p, err := r.srv.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.URL, r.base()+"/m/#pair=") || !strings.HasPrefix(p.QR, "data:image/png;base64,") || len(p.Code) != codeLength {
		t.Errorf("pairing = %+v", p)
	}

	// A wrong code gets nowhere.
	if code, _ := r.post("/pair/request", map[string]string{"code": "WRONGCODE2", "name": "Evil"}); code != http.StatusForbidden {
		t.Errorf("wrong code = %d", code)
	}
	code, req := r.post("/pair/request", map[string]string{"code": strings.ToLower(p.Code), "name": "  Pixel\n9  "})
	if code != http.StatusOK {
		t.Fatalf("request = %d %v", code, req)
	}
	id, claim := req["id"].(string), req["claim"].(string)
	// The code worked once.
	if code, _ := r.post("/pair/request", map[string]string{"code": p.Code, "name": "Again"}); code != http.StatusForbidden {
		t.Errorf("reused code = %d", code)
	}
	mu.Lock()
	if len(asked) != 1 || asked[0].Name != "Pixel 9" || asked[0].ID != id {
		t.Errorf("window asked = %+v", asked)
	}
	mu.Unlock()
	// Someone without the claim can't collect it.
	if code, _ := r.post("/pair/claim", map[string]string{"id": id, "claim": "guess"}); code != http.StatusNotFound {
		t.Errorf("claim without the secret = %d", code)
	}

	// Allow, while the phone waits.
	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := r.srv.AnswerPairing(id, true); err != nil {
			t.Error(err)
		}
	}()
	code, got := r.post("/pair/claim", map[string]string{"id": id, "claim": claim})
	if code != http.StatusOK {
		t.Fatalf("claim = %d %v", code, got)
	}
	token := got["token"].(string)
	if len(token) != 64 {
		t.Errorf("token = %q", token)
	}
	// Collected once.
	if code, _ := r.post("/pair/claim", map[string]string{"id": id, "claim": claim}); code != http.StatusNotFound {
		t.Errorf("second claim = %d", code)
	}
	// Stored as a hash only.
	raw, _ := r.svc.Store.Setting(devicesKey)
	if strings.Contains(raw, token) || !strings.Contains(raw, hashToken(token)) {
		t.Errorf("stored = %s", raw)
	}
	if c := r.rpcAs(token, "Bootstrap"); c != http.StatusOK {
		t.Errorf("paired call = %d", c)
	}
	devs := r.srv.Devices()
	if len(devs) != 1 || devs[0].Name != "Pixel 9" {
		t.Fatalf("devices = %+v", devs)
	}

	// Revoke: refused at once, and its event stream ends.
	req2, _ := http.NewRequest("GET", r.base()+"/events", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	stream, err := (&http.Client{Transport: r.https.Transport}).Do(req2)
	if err != nil || stream.StatusCode != http.StatusOK {
		t.Fatalf("stream = %v, %v", stream, err)
	}
	ended := make(chan struct{})
	go func() {
		var b [4096]byte
		for {
			if _, err := stream.Body.Read(b[:]); err != nil {
				close(ended)
				return
			}
		}
	}()
	if err := r.srv.RevokeDevice(devs[0].ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Error("the revoked device's stream stayed open")
	}
	if c := r.rpcAs(token, "Bootstrap"); c != http.StatusUnauthorized {
		t.Errorf("revoked call = %d", c)
	}
	mu.Lock()
	if len(lists) != 2 || len(lists[1]) != 0 {
		t.Errorf("device lists = %+v", lists)
	}
	mu.Unlock()
}

func TestPairingDenied(t *testing.T) {
	r := newRigWith(t, Config{})
	p, _ := r.srv.StartPairing()
	_, req := r.post("/pair/request", map[string]string{"code": p.Code, "name": "Tablet"})
	id, claim := req["id"].(string), req["claim"].(string)
	if err := r.srv.AnswerPairing(id, false); err != nil {
		t.Fatal(err)
	}
	if code, _ := r.post("/pair/claim", map[string]string{"id": id, "claim": claim}); code != http.StatusForbidden {
		t.Errorf("denied claim = %d", code)
	}
	if len(r.srv.Devices()) != 0 {
		t.Error("a denied device was paired")
	}
	// No name, no request.
	p, _ = r.srv.StartPairing()
	if code, _ := r.post("/pair/request", map[string]string{"code": p.Code, "name": " \n "}); code != http.StatusForbidden {
		t.Errorf("nameless = %d", code)
	}
}

func TestPairingCodes(t *testing.T) {
	d := NewDevices(memStore{})
	p := newPairing(d)
	now := time.Now()
	p.now = func() time.Time { return now }
	pr, _ := p.start("")
	now = now.Add(pairLife + time.Second)
	if _, _, err := p.request(pr.Code, "Late"); err != errBadCode {
		t.Errorf("expired code = %v", err)
	}
	pr, _ = p.start("")
	for i := 0; i < maxBadTry; i++ {
		_, _, _ = p.request("NOPE", "x")
	}
	if _, _, err := p.request(pr.Code, "Right"); err != errTooMany {
		t.Errorf("after many wrong codes = %v", err)
	}
	now = now.Add(time.Minute)
	if _, _, err := p.request(pr.Code, "Right"); err != nil {
		t.Errorf("a minute later = %v", err)
	}
	for i := 0; i < 50; i++ {
		c := newCode()
		if len(c) != codeLength || strings.Trim(c, codeAlphabet) != "" {
			t.Fatalf("code %q", c)
		}
	}
	if got := cleanName(strings.Repeat("é", 60)); len([]rune(got)) != maxNameLen {
		t.Errorf("long name = %q", got)
	}
}

type memStore map[string]string

func (m memStore) Setting(k string) (string, error) { return m[k], nil }
func (m memStore) SetSetting(k, v string) error     { m[k] = v; return nil }
