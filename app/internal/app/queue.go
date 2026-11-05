package app

import (
	"context"
	"encoding/json"

	"github.com/harvey-withington/uncli/app/internal/session"
)

// A message written while an answer runs waits in the session's queue
// (session/queue.go) and goes when the answer ends.

// Unqueued is a queued message taken back, for the message box to edit:
// its text and its attachments as they were given.
type Unqueued struct {
	Text        string          `json:"text"`
	Attachments []AttachmentRef `json:"attachments"`
}

// QueueWith queues a message: it's sent now if the session isn't
// answering, else when the answer ends. Its attachments are read now.
func (s *Service) QueueWith(ctx context.Context, sessionID, text string, refs []AttachmentRef, opts SendOptions) error {
	files, err := s.loadAttachments(refs)
	if err != nil {
		return err
	}
	q := session.Queued{Text: text, Context: opts.Context, Files: files}
	for _, r := range refs {
		b, _ := json.Marshal(r)
		q.Refs = append(q.Refs, b)
	}
	return s.Sessions.Queue(ctx, sessionID, q)
}

// Unqueue takes a session's queued message back; an empty one when there
// was none.
func (s *Service) Unqueue(sessionID string) (Unqueued, error) {
	q, err := s.Sessions.Unqueue(sessionID)
	out := Unqueued{Attachments: []AttachmentRef{}}
	if err != nil || q == nil {
		return out, err
	}
	out.Text = q.Text
	for _, b := range q.Refs {
		var r AttachmentRef
		if json.Unmarshal(b, &r) == nil {
			out.Attachments = append(out.Attachments, r)
		}
	}
	return out, nil
}
