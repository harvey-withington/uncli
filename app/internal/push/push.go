// Package push sends Web Push notifications to paired devices' browsers
// (decision 0021): a session needs approval, finished, or failed, with the
// app closed. It keeps the server's VAPID keys (made once) and one
// subscription per device, each with that device's Notifications setting,
// in the app's settings store. Payloads are encrypted end to end to the
// browser (RFC 8291), so the push service sees nothing but their size.
// BRUV's internal/push is the model.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Store keeps the keys and subscriptions (the app's settings store).
type Store interface {
	Setting(key string) (string, error)
	SetSetting(key, value string) error
}

const (
	vapidKey = "push.vapid"
	subsKey  = "push.subscriptions"
	// subject is who the push services can write to about this sender. The
	// spec wants a mailto: or https: URL; UNCLI has neither, so it names
	// itself on an address that never resolves.
	subject = "mailto:uncli-push@uncli.invalid"
	ttl     = 24 * 60 * 60 // how long a push service keeps a note for an offline device
)

// Notification settings, per device, as on the desktop.
const (
	All       = "all"
	Approvals = "approvals"
	Off       = "off"
)

func ValidSetting(s string) bool { return s == All || s == Approvals || s == Off }

// Subscription is a device's browser push subscription (the browser's
// PushSubscription.toJSON()) and what it wants to be told.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type record struct {
	Device   string       `json:"device"`
	Sub      Subscription `json:"sub"`
	Setting  string       `json:"setting"`
	Since    time.Time    `json:"since"`
	LastFail string       `json:"lastFail,omitempty"`
}

// State is a device's push state, as its Settings shows it.
type State struct {
	Subscribed bool   `json:"subscribed"`
	Setting    string `json:"setting"` // all, approvals or off
	LastFail   string `json:"lastFail,omitempty"`
}

// Note is what a device is told: the payload its service worker shows.
type Note struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Session  string `json:"session,omitempty"` // opens it when tapped
	Tag      string `json:"tag,omitempty"`     // a newer note about the same session replaces the older
	Approval bool   `json:"-"`                 // a session waiting for approval (sent under "approvals only" too)
}

// Pusher sends notes to subscribed devices.
type Pusher struct {
	store Store
	// Allowed says whether an endpoint is a push service UNCLI will post
	// to. A device chooses its endpoint, so this keeps the server from
	// being pointed at anything else (another machine on the tailnet, a
	// local service).
	Allowed func(*url.URL) bool
	client  *http.Client

	mu      sync.Mutex
	pub     string
	priv    string
	records []record
}

// New loads the keys (making them the first time) and subscriptions.
func New(store Store) (*Pusher, error) {
	p := &Pusher{store: store, Allowed: KnownService, client: &http.Client{Timeout: 15 * time.Second}}
	var keys struct{ Public, Private string }
	if raw, _ := store.Setting(vapidKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &keys)
	}
	if keys.Public == "" || keys.Private == "" {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, fmt.Errorf("push: making keys: %w", err)
		}
		keys.Public, keys.Private = pub, priv
		b, _ := json.Marshal(keys)
		if err := store.SetSetting(vapidKey, string(b)); err != nil {
			return nil, err
		}
	}
	p.pub, p.priv = keys.Public, keys.Private
	if raw, _ := store.Setting(subsKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &p.records)
	}
	return p, nil
}

// PublicKey is the VAPID key a browser subscribes with.
func (p *Pusher) PublicKey() string { return p.pub }

// knownHosts are the push services of the browsers UNCLI's phone app runs
// in: Chrome and Edge on Android (FCM), Firefox, Edge on Windows, Safari.
var knownHosts = []string{"fcm.googleapis.com", "push.services.mozilla.com", "notify.windows.com", "push.apple.com"}

// KnownService allows https endpoints on the browsers' push services only.
func KnownService(u *url.URL) bool {
	if u.Scheme != "https" || net.ParseIP(u.Hostname()) != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	for _, k := range knownHosts {
		if h == k || strings.HasSuffix(h, "."+k) {
			return true
		}
	}
	return false
}

func (p *Pusher) saveLocked() error {
	b, err := json.Marshal(p.records)
	if err != nil {
		return err
	}
	return p.store.SetSetting(subsKey, string(b))
}

func (p *Pusher) find(device string) int {
	for i, r := range p.records {
		if r.Device == device {
			return i
		}
	}
	return -1
}

// Subscribe sets a device's subscription (replacing any it had), keeping
// its setting.
func (p *Pusher) Subscribe(device string, sub Subscription) error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || !p.Allowed(u) {
		return errors.New("that isn't a push service this server sends to")
	}
	if sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		return errors.New("the subscription has no keys")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	r := record{Device: device, Sub: sub, Setting: All, Since: time.Now().UTC()}
	if i := p.find(device); i >= 0 {
		r.Setting = p.records[i].Setting
		p.records[i] = r
	} else {
		p.records = append(p.records, r)
	}
	return p.saveLocked()
}

// Forget drops a device's subscription (it turned notifications off in
// its browser, or was revoked).
func (p *Pusher) Forget(device string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	i := p.find(device)
	if i < 0 {
		return nil
	}
	p.records = append(p.records[:i], p.records[i+1:]...)
	return p.saveLocked()
}

// SetSetting sets what a subscribed device is told.
func (p *Pusher) SetSetting(device, setting string) error {
	if !ValidSetting(setting) {
		return fmt.Errorf("unknown notification setting %q", setting)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	i := p.find(device)
	if i < 0 {
		return errors.New("notifications aren't on for this device yet")
	}
	p.records[i].Setting = setting
	return p.saveLocked()
}

// StateOf is a device's push state.
func (p *Pusher) StateOf(device string) State {
	p.mu.Lock()
	defer p.mu.Unlock()
	if i := p.find(device); i >= 0 {
		return State{Subscribed: true, Setting: p.records[i].Setting, LastFail: p.records[i].LastFail}
	}
	return State{Setting: All}
}

// wants says whether a setting lets a note through.
func wants(setting string, n Note) bool {
	return setting == All || (setting == Approvals && n.Approval)
}

// Notify sends a note to every subscribed device whose setting wants it.
// Failures are kept per device (its Settings shows the last); a device
// whose subscription the push service says is gone is forgotten.
func (p *Pusher) Notify(ctx context.Context, n Note) {
	p.mu.Lock()
	targets := []record{}
	for _, r := range p.records {
		if wants(r.Setting, n) {
			targets = append(targets, r)
		}
	}
	p.mu.Unlock()
	for _, r := range targets {
		p.deliver(ctx, r, n)
	}
}

// Test sends a sample note to one device, whatever its setting.
func (p *Pusher) Test(ctx context.Context, device string, n Note) error {
	p.mu.Lock()
	i := p.find(device)
	var r record
	if i >= 0 {
		r = p.records[i]
	}
	p.mu.Unlock()
	if i < 0 {
		return errors.New("notifications aren't on for this device yet")
	}
	return p.deliver(ctx, r, n)
}

// errGone is a subscription the push service no longer has.
var errGone = errors.New("the browser's subscription has ended; turn notifications on again")

func (p *Pusher) deliver(ctx context.Context, r record, n Note) error {
	err := p.send(ctx, r.Sub, n)
	p.mu.Lock()
	defer p.mu.Unlock()
	i := p.find(r.Device)
	if i < 0 {
		return err
	}
	switch {
	case errors.Is(err, errGone):
		p.records = append(p.records[:i], p.records[i+1:]...)
	case err != nil:
		p.records[i].LastFail = err.Error()
	default:
		p.records[i].LastFail = ""
	}
	_ = p.saveLocked()
	return err
}

func (p *Pusher) send(ctx context.Context, sub Subscription, n Note) error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || !p.Allowed(u) {
		return errors.New("that isn't a push service this server sends to")
	}
	payload, err := json.Marshal(n)
	if err != nil {
		return err
	}
	ws := &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth}}
	urgency := webpush.UrgencyNormal
	if n.Approval {
		urgency = webpush.UrgencyHigh // someone is waiting on it: wake the phone
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, ws, &webpush.Options{
		HTTPClient: p.client, Subscriber: subject, VAPIDPublicKey: p.pub, VAPIDPrivateKey: p.priv,
		TTL: ttl, Urgency: urgency, Topic: topicOf(n.Tag),
	})
	if err != nil {
		return fmt.Errorf("couldn't reach the push service: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return errGone
	}
	return fmt.Errorf("the push service said %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// topicOf makes a tag a push Topic (up to 32 URL-safe characters), so a
// newer note about a session replaces an older one still waiting on the
// push service.
func topicOf(tag string) string {
	out := make([]byte, 0, 32)
	for i := 0; i < len(tag) && len(out) < 32; i++ {
		c := tag[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			out = append(out, c)
		}
	}
	return string(out)
}
