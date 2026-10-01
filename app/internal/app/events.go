package app

import (
	"encoding/json"
	"sync"
	"time"

	"uncli/internal/core"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Event names the frontend subscribes to.
const (
	EvtSession     = "session:event"   // SessionEventMsg
	EvtSessionView = "session:changed" // session.View
	EvtPage        = "page:changed"    // store.Page
	EvtCLIProgress = "cli:progress"    // Progress
	EvtCLIStatus   = "cli:status"      // CLIStatus
)

// Emitter delivers named events to the UI. The bridge implements it with
// Wails; tests record.
type Emitter interface {
	Emit(name string, data any)
}

type SessionEventMsg struct {
	SessionID string     `json:"sessionId"`
	Event     core.Event `json:"event"`
}

type Progress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// coalescer is the session Sink: it forwards events to the UI, merging
// text deltas so the UI gets at most one per session every interval.
type coalescer struct {
	out      Emitter
	interval time.Duration

	mu      sync.Mutex
	pending map[string]*pendingDelta
}

type pendingDelta struct {
	ev    core.Event
	delta core.TextDelta
	timer *time.Timer
}

func newCoalescer(out Emitter, interval time.Duration) *coalescer {
	return &coalescer{out: out, interval: interval, pending: map[string]*pendingDelta{}}
}

func (c *coalescer) SessionEvent(id string, ev core.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ev.Kind == core.EvTextDelta {
		d, _ := core.Decode[core.TextDelta](ev)
		if p := c.pending[id]; p != nil {
			if p.delta.Index == d.Index && p.ev.TurnSeq == ev.TurnSeq {
				p.delta.Text += d.Text
				return
			}
			c.flushLocked(id)
		}
		p := &pendingDelta{ev: ev, delta: d}
		p.timer = time.AfterFunc(c.interval, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.pending[id] == p {
				c.flushLocked(id)
			}
		})
		c.pending[id] = p
		return
	}
	c.flushLocked(id)
	c.out.Emit(EvtSession, SessionEventMsg{SessionID: id, Event: ev})
}

func (c *coalescer) flushLocked(id string) {
	p := c.pending[id]
	if p == nil {
		return
	}
	delete(c.pending, id)
	p.timer.Stop()
	ev := p.ev
	ev.Data, _ = json.Marshal(p.delta)
	c.out.Emit(EvtSession, SessionEventMsg{SessionID: id, Event: ev})
}

func (c *coalescer) SessionChanged(v session.View) { c.out.Emit(EvtSessionView, v) }
func (c *coalescer) PageChanged(p store.Page)      { c.out.Emit(EvtPage, p) }
