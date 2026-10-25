package remote

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	qrcode "github.com/skip2/go-qrcode"
)

// Pairing (decision 0022): the server's window shows a QR code and a link
// carrying a pairing code that lasts a few minutes and works once. The
// device opens it, gives its name and asks to pair (POST /pair/request);
// the window asks "Allow <name>?"; the device waits for the answer
// (POST /pair/claim) and, if allowed, gets its own token. The code rides
// in the link's fragment (#pair=…), which browsers never send to a
// server, so it isn't in any request line or log. There is no other way
// in: no shared token, no automatic pairing from this machine.

const (
	pairLife   = 5 * time.Minute // how long a code works
	answerLife = 5 * time.Minute // how long a request waits for Allow
	claimWait  = 25 * time.Second
	maxBadTry  = 10 // wrong codes a minute before /pair/request pauses
	// codeAlphabet leaves out letters that read alike (0/O, 1/I/L).
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLength   = 10
	maxNameLen   = 40
)

// Pairing is what the window shows while a code is open.
type Pairing struct {
	Code    string    `json:"code"`
	URL     string    `json:"url"`     // the link, code in its fragment ("" when not on the tailnet)
	QR      string    `json:"qr"`      // the link as a QR code, a PNG data URL
	Expires time.Time `json:"expires"` // when the code stops working
}

// PairRequest is a device asking to pair, waiting for Allow. Name is what
// the device calls itself; where it's calling from is what Tailscale says
// (Machine, User), or this machine (Local), or unknown (all empty).
type PairRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Machine string `json:"machine,omitempty"`
	User    string `json:"user,omitempty"`
	Local   bool   `json:"local,omitempty"`
	// Expires is when the request stops waiting for an answer.
	Expires time.Time `json:"expires"`
}

type pending struct {
	PairRequest
	claim   string // the device's secret for collecting the answer
	expires time.Time
	answer  chan bool // closed after the answer is set
	allowed bool
	token   string
	device  Device
	err     error
}

type pairing struct {
	devices *Devices
	now     func() time.Time
	// asked tells the window a device is waiting for Allow.
	asked func(PairRequest)

	mu      sync.Mutex
	code    string
	expires time.Time
	waiting map[string]*pending
	// bad is each caller's wrong codes in the last minute, by address: one
	// caller's guessing can't keep another (the real phone) out.
	bad map[string][]time.Time
}

func newPairing(d *Devices) *pairing {
	return &pairing{devices: d, now: time.Now, waiting: map[string]*pending{}, bad: map[string][]time.Time{}}
}

// newCode is codeLength letters from codeAlphabet, uniformly (bytes that
// would bias the choice are drawn again).
func newCode() string {
	out := make([]byte, 0, codeLength)
	limit := byte(256 - 256%len(codeAlphabet))
	var b [1]byte
	for len(out) < codeLength {
		_, _ = rand.Read(b[:])
		if b[0] < limit {
			out = append(out, codeAlphabet[int(b[0])%len(codeAlphabet)])
		}
	}
	return string(out)
}

// start opens a new code, replacing any open one. base is the address
// devices reach the server at ("" when there's none yet).
func (p *pairing) start(base string) (Pairing, error) {
	p.mu.Lock()
	p.code = newCode()
	p.expires = p.now().Add(pairLife)
	out := Pairing{Code: p.code, Expires: p.expires}
	p.mu.Unlock()
	if base == "" {
		return out, nil
	}
	out.URL = base + "/m/#pair=" + out.Code
	png, err := qrcode.Encode(out.URL, qrcode.Medium, 320)
	if err != nil {
		return Pairing{}, err
	}
	out.QR = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	return out, nil
}

// cancel closes the open code.
func (p *pairing) cancel() {
	p.mu.Lock()
	p.code = ""
	p.mu.Unlock()
}

var (
	errBadCode   = errors.New("that pairing code isn't valid any more: show a new one on the server")
	errTooMany   = errors.New("too many wrong codes: wait a minute")
	errNoRequest = errors.New("no such pairing request")
)

// request is a device, calling from an address, asking to pair with code
// under a name. The code is spent whatever the answer. Wrong codes count
// against the caller only while a code is open: with none open there's
// nothing to guess.
func (p *pairing) request(from, code, name string, who PairRequest) (id, claim string, err error) {
	name = cleanName(name)
	if name == "" {
		return "", "", errors.New("give this device a name")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	p.forgetLocked(now)
	if len(p.bad[from]) >= maxBadTry {
		return "", "", errTooMany
	}
	open := p.code != "" && now.Before(p.expires)
	if !open || subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(code))), []byte(p.code)) != 1 {
		if open {
			p.bad[from] = append(p.bad[from], now)
		}
		return "", "", errBadCode
	}
	p.code = "" // one use
	who.ID, who.Name, who.Expires = randomHex(8), name, now.Add(answerLife)
	r := &pending{PairRequest: who, claim: randomHex(32), expires: who.Expires, answer: make(chan bool)}
	p.waiting[r.ID] = r
	if p.asked != nil {
		go p.asked(r.PairRequest)
	}
	return r.ID, r.claim, nil
}

// forgetLocked drops wrong codes over a minute old, and requests past
// their time. A device allowed but never collected (the phone gave up
// waiting) is revoked, so the list holds no device nobody has the token
// of.
func (p *pairing) forgetLocked(now time.Time) {
	for from, times := range p.bad {
		if times = slices.DeleteFunc(times, func(t time.Time) bool { return now.Sub(t) >= time.Minute }); len(times) == 0 {
			delete(p.bad, from)
		} else {
			p.bad[from] = times
		}
	}
	for id, r := range p.waiting {
		if now.Before(r.expires.Add(claimWait)) {
			continue
		}
		delete(p.waiting, id)
		if r.allowed {
			_ = p.devices.Revoke(r.device.ID)
		}
	}
}

// answer is the window's Allow or Deny.
func (p *pairing) answer(id string, allow bool) error {
	p.mu.Lock()
	r, ok := p.waiting[id]
	if ok && p.now().After(r.expires) {
		delete(p.waiting, id)
		ok = false
	}
	if !ok {
		p.mu.Unlock()
		return errNoRequest
	}
	select {
	case <-r.answer:
		p.mu.Unlock()
		return errors.New("that request was already answered")
	default:
	}
	if allow {
		r.device, r.token, r.err = p.devices.add(r.Name)
		r.allowed = r.err == nil
	}
	close(r.answer)
	p.mu.Unlock()
	return r.err
}

// Claim results.
var (
	errDenied  = errors.New("the server didn't allow this device")
	errPending = errors.New("waiting for Allow on the server")
)

// claim waits (up to claimWait) for the answer to a request. Allowed, it
// hands over the token once and forgets the request.
func (p *pairing) claim(ctx context.Context, id, claim string) (Device, string, error) {
	p.mu.Lock()
	r, ok := p.waiting[id]
	if !ok || subtle.ConstantTimeCompare([]byte(claim), []byte(r.claim)) != 1 {
		p.mu.Unlock()
		return Device{}, "", errNoRequest
	}
	p.mu.Unlock()
	t := time.NewTimer(claimWait)
	defer t.Stop()
	select {
	case <-r.answer:
	case <-t.C:
		return Device{}, "", errPending
	case <-ctx.Done():
		return Device{}, "", errPending
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, still := p.waiting[id]; !still {
		return Device{}, "", errNoRequest
	}
	delete(p.waiting, id)
	if !r.allowed {
		return Device{}, "", errDenied
	}
	return r.device, r.token, nil
}

// cleanName keeps a device name to one plain line of maxNameLen.
func cleanName(s string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r < 32 || r == 127 }), " ")
	s = strings.Join(strings.Fields(s), " ")
	for utf8.RuneCountInString(s) > maxNameLen {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return s
}

// POST /pair/request {code, name} → {id, claim}
func (s *Server) servePairRequest(w http.ResponseWriter, r *http.Request) {
	var in struct{ Code, Name string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	from, _, _ := net.SplitHostPort(r.RemoteAddr)
	id, claim, err := s.pair.request(from, in.Code, in.Name, s.whoIs(r.Context(), r.RemoteAddr))
	switch {
	case errors.Is(err, errTooMany):
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "claim": claim})
	}
}

// whoIs is where a pairing request comes from: this machine, or the
// tailnet machine and user Tailscale puts behind the address (empty when
// it can't say).
func (s *Server) whoIs(ctx context.Context, remoteAddr string) PairRequest {
	if ap, err := netip.ParseAddrPort(remoteAddr); err == nil && ap.Addr().IsLoopback() {
		return PairRequest{Local: true}
	}
	ps, ok := s.certs.(PeerSource)
	if !ok {
		return PairRequest{}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	p, err := ps.WhoIs(ctx, remoteAddr)
	if err != nil {
		return PairRequest{}
	}
	return PairRequest{Machine: p.Machine, User: p.User}
}

// POST /pair/claim {id, claim} → 200 {device, token}, 202 still waiting,
// 403 denied, 404 unknown or already collected.
func (s *Server) servePairClaim(w http.ResponseWriter, r *http.Request) {
	var in struct{ ID, Claim string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	d, token, err := s.pair.claim(r.Context(), in.ID, in.Claim)
	switch {
	case errors.Is(err, errPending):
		writeJSON(w, http.StatusAccepted, map[string]string{"error": err.Error()})
	case errors.Is(err, errDenied):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"device": d, "token": token})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
