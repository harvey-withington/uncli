package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"
)

// Paired devices (decision 0022): each has its own token, shown to it
// once when it's allowed and stored here only as a SHA-256 hash (the
// token is 32 random bytes, so a fast hash is enough). Revoking a device
// forgets it and ends its open streams at once.

const devicesKey = "remote.devices"

// seenEvery is how often a device's last-seen time is saved.
const seenEvery = time.Minute

// PairedDevice is a device as Settings lists it.
type PairedDevice struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Paired   time.Time `json:"paired"`
	LastSeen time.Time `json:"lastSeen"`
}

type deviceRecord struct {
	PairedDevice
	TokenHash string `json:"tokenHash"`
}

// Devices is the paired-device list, kept in the app's settings store.
type Devices struct {
	store   Store
	now     func() time.Time
	changed func() // the list changed (paired, revoked, renamed)

	mu   sync.Mutex
	list []deviceRecord
}

func NewDevices(store Store) *Devices {
	d := &Devices{store: store, now: time.Now}
	if raw, _ := store.Setting(devicesKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &d.list)
	}
	return d
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// saveLocked writes the list. Caller holds d.mu.
func (d *Devices) saveLocked() error {
	b, err := json.Marshal(d.list)
	if err != nil {
		return err
	}
	return d.store.SetSetting(devicesKey, string(b))
}

// Authenticate finds the device a token belongs to, and notes it was seen.
func (d *Devices) Authenticate(token string) (Device, bool) {
	h := hashToken(token)
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.list {
		if subtle.ConstantTimeCompare([]byte(d.list[i].TokenHash), []byte(h)) == 1 {
			r := &d.list[i]
			if now := d.now(); now.Sub(r.LastSeen) > seenEvery {
				r.LastSeen = now
				_ = d.saveLocked()
			}
			return Device{ID: r.ID, Name: r.Name}, true
		}
	}
	return Device{}, false
}

// add pairs a device and returns its token, which is never stored.
func (d *Devices) add(name string) (Device, string, error) {
	token := randomHex(32)
	now := d.now()
	r := deviceRecord{PairedDevice: PairedDevice{ID: randomHex(8), Name: name, Paired: now, LastSeen: now}, TokenHash: hashToken(token)}
	d.mu.Lock()
	d.list = append(d.list, r)
	err := d.saveLocked()
	d.mu.Unlock()
	if err != nil {
		return Device{}, "", err
	}
	d.notify()
	return Device{ID: r.ID, Name: r.Name}, token, nil
}

// List is the paired devices, most recently seen first.
func (d *Devices) List() []PairedDevice {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]PairedDevice, 0, len(d.list))
	for _, r := range d.list {
		out = append(out, r.PairedDevice)
	}
	slices.SortFunc(out, func(a, b PairedDevice) int { return b.LastSeen.Compare(a.LastSeen) })
	return out
}

// Has says whether a device is paired.
func (d *Devices) Has(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.ContainsFunc(d.list, func(r deviceRecord) bool { return r.ID == id })
}

// Revoke forgets a device: its token stops working at once.
func (d *Devices) Revoke(id string) error {
	d.mu.Lock()
	i := slices.IndexFunc(d.list, func(r deviceRecord) bool { return r.ID == id })
	if i < 0 {
		d.mu.Unlock()
		return errors.New("no such device")
	}
	d.list = slices.Delete(d.list, i, i+1)
	err := d.saveLocked()
	d.mu.Unlock()
	d.notify()
	return err
}

func (d *Devices) notify() {
	if d.changed != nil {
		d.changed()
	}
}
