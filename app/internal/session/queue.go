package session

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/harvey-withington/uncli/app/internal/core"
)

// A message written while an answer is running waits in the session's
// queue and goes when the turn ends, so the user needn't watch for the end
// to send it. It's UNCLI's queue, not the CLI's: the same for every
// provider, and seen by every device. Queuing again adds to it, so one
// message goes rather than several turns in a row. It goes by itself only
// after an answer that finished: after Stop, an error, or a turn cut off,
// it's held for the user to send or take back. It lives as long as UNCLI
// runs.

// Queued is a message waiting in a session's queue. Refs are the
// attachments as the caller sent them, handed back when the user takes the
// message back to edit it; Files are the same attachments, loaded.
type Queued struct {
	Text    string            `json:"text"`
	Context string            `json:"-"`
	Files   []core.Attachment `json:"-"`
	Refs    []json.RawMessage `json:"-"`
}

// QueuedView is the queue as the UI shows it.
type QueuedView struct {
	Text  string   `json:"text"`
	Files []string `json:"files,omitempty"` // the attachments' names
	Held  bool     `json:"held,omitempty"`  // waiting for the user: the last turn didn't finish
}

func (s *Session) queuedViewLocked() *QueuedView {
	q := s.queued
	if q == nil {
		return nil
	}
	v := &QueuedView{Text: q.Text, Held: s.queueHeld}
	for _, f := range q.Files {
		v.Files = append(v.Files, f.Name)
	}
	return v
}

func (q *Queued) add(more Queued) {
	switch {
	case q.Text == "":
		q.Text = more.Text
	case more.Text != "":
		q.Text += "\n\n" + more.Text
	}
	if more.Context != "" {
		q.Context = more.Context // what's on the host's screen now
	}
	q.Files = append(q.Files, more.Files...)
	q.Refs = append(q.Refs, more.Refs...)
}

// Queue sends a message when the session's answer ends: now if it isn't
// answering (with anything held in the queue before it), else after.
func (m *Manager) Queue(ctx context.Context, id string, q Queued) error {
	q.Text = strings.TrimSpace(q.Text)
	if q.Text == "" && len(q.Files) == 0 {
		return errors.New("nothing to send")
	}
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.rec.Archived {
		s.mu.Unlock()
		return errors.New("this session is archived; restore it to continue")
	}
	if s.queued == nil {
		s.queued = &Queued{}
	}
	s.queued.add(q)
	if s.page != nil {
		s.changed()
		s.mu.Unlock()
		return nil
	}
	return m.sendQueuedLocked(ctx, s)
}

// SendQueued sends a session's queued message now; for one held after a
// turn that didn't finish.
func (m *Manager) SendQueued(ctx context.Context, id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.queued == nil {
		s.mu.Unlock()
		return nil
	}
	if s.page != nil {
		s.mu.Unlock()
		return errors.New("this session is still answering; the message goes when it's done")
	}
	return m.sendQueuedLocked(ctx, s)
}

// sendQueuedLocked takes the queue and sends it. If it can't be sent, it
// goes back, held, with why. Called with s.mu held; it unlocks.
func (m *Manager) sendQueuedLocked(ctx context.Context, s *Session) error {
	q := s.queued
	s.queued, s.queueHeld = nil, false
	s.changed()
	s.mu.Unlock()
	err := m.SendWith(ctx, s.rec.ID, q.Text, q.Context, q.Files...)
	if err != nil {
		s.mu.Lock()
		if s.queued != nil {
			q.add(*s.queued) // what was queued meanwhile goes after
		}
		s.queued, s.queueHeld = q, true
		s.changed()
		s.mu.Unlock()
	}
	return err
}

// Unqueue takes a session's queued message back, for the user to edit; nil
// when there's none.
func (m *Manager) Unqueue(id string) (*Queued, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queued
	s.queued, s.queueHeld = nil, false
	if q != nil {
		s.changed()
	}
	return q, nil
}

// queueAfterLocked follows a turn's end: an answer that finished sends the
// queue; any other end holds it. Caller holds s.mu.
func (s *Session) queueAfterLocked(status string) {
	if s.queued == nil {
		return
	}
	if status != "done" || s.queueHeld {
		s.queueHeld = true
		return
	}
	s.queueHeld = true // until it's taken, so a second end can't send it twice
	go func() {
		s.mu.Lock()
		if s.queued == nil || s.page != nil {
			s.mu.Unlock()
			return
		}
		_ = s.m.sendQueuedLocked(context.Background(), s)
	}()
}
