package remote

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

// The hub numbers every event the app emits and keeps the latest ones, so
// a client that loses its connection (a phone going to sleep) asks for
// what came after the last number it saw and gets exactly that, or, when
// the hub no longer holds them or restarted since, a reload marker that
// sends it back to fetching state afresh (decision 0021). One sequence
// covers all events: a session's events are in order within it, and one
// number is all a client has to remember.

// Event is one numbered event as it goes on the wire.
type Event struct {
	Seq  uint64          `json:"seq"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// Limits on what the hub keeps for clients catching up.
const (
	keepEvents = 8192
	keepBytes  = 16 << 20
	subBuffer  = 512 // events queued for a slow client before it's dropped (it resumes from the backlog)
)

// localOnly are events only this machine's window acts on.
var localOnly = map[string]bool{"notify:open": true}

type Hub struct {
	epoch string // changes each time UNCLI starts: older numbers mean nothing

	mu    sync.Mutex
	seq   uint64
	ring  []Event
	bytes int
	subs  map[*Subscription]bool
}

// Subscription is one client's live stream.
type Subscription struct {
	Device string
	C      chan Event
	done   chan struct{}
	once   sync.Once
}

// Done is closed when the subscription ends: the client was too slow, its
// device was revoked or the hub closed it.
func (s *Subscription) Done() <-chan struct{} { return s.done }

func (s *Subscription) end() { s.once.Do(func() { close(s.done) }) }

func NewHub() *Hub {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return &Hub{epoch: hex.EncodeToString(b[:]), subs: map[*Subscription]bool{}}
}

// Emit numbers an event and hands it to every client; the app's Emitter.
func (h *Hub) Emit(name string, data any) {
	if localOnly[name] {
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	ev := Event{Seq: h.seq, Name: name, Data: raw}
	h.ring = append(h.ring, ev)
	h.bytes += len(raw)
	for len(h.ring) > keepEvents || (h.bytes > keepBytes && len(h.ring) > 1) {
		h.bytes -= len(h.ring[0].Data)
		h.ring = h.ring[1:]
	}
	for s := range h.subs {
		select {
		case s.C <- ev:
		default: // too far behind: drop it, and it catches up from the ring
			delete(h.subs, s)
			s.end()
		}
	}
}

func (h *Hub) cursorLocked(seq uint64) string { return h.epoch + ":" + strconv.FormatUint(seq, 10) }

// Catchup is what a client gets on (re)connecting: the events it missed,
// or Reload when it must fetch state afresh (a new client, an unknown
// epoch, or missed events the hub no longer holds); and the cursor to
// resume from if no event follows. That is the client's own cursor while
// it still has missed events to read, so a connection that drops partway
// through them resumes where it stopped; after a reload, the latest.
type Catchup struct {
	Missed []Event
	Reload bool
	Cursor string
}

// Subscribe starts a stream for a device after cursor ("" for a new
// client).
func (h *Hub) Subscribe(device, cursor string) (*Subscription, Catchup) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub := &Subscription{Device: device, C: make(chan Event, subBuffer), done: make(chan struct{})}
	h.subs[sub] = true
	c := Catchup{Cursor: h.cursorLocked(h.seq)}
	after, ok := h.parseLocked(cursor)
	switch {
	case !ok:
		c.Reload = true
	case after == h.seq:
	case len(h.ring) == 0 || h.ring[0].Seq > after+1:
		c.Reload = true
	default:
		c.Missed = append([]Event(nil), h.ring[after+1-h.ring[0].Seq:]...)
		c.Cursor = h.cursorLocked(after)
	}
	return sub, c
}

// CursorOf is the cursor of an event.
func (h *Hub) CursorOf(ev Event) string { return h.epoch + ":" + strconv.FormatUint(ev.Seq, 10) }

func (h *Hub) parseLocked(cursor string) (uint64, bool) {
	epoch, n, ok := strings.Cut(cursor, ":")
	if !ok || epoch != h.epoch {
		return 0, false
	}
	seq, err := strconv.ParseUint(n, 10, 64)
	if err != nil || seq > h.seq {
		return 0, false
	}
	return seq, true
}

// Unsubscribe ends a stream.
func (h *Hub) Unsubscribe(s *Subscription) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
	s.end()
}

// CloseDevice ends every stream of a device (it was revoked).
func (h *Hub) CloseDevice(device string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if s.Device == device {
			delete(h.subs, s)
			s.end()
		}
	}
}

// CloseAll ends every stream (remote access turned off).
func (h *Hub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		delete(h.subs, s)
		s.end()
	}
}
