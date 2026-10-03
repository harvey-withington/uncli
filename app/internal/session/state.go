package session

import (
	"uncli/internal/core"
	"uncli/internal/store"
)

// State is a session's activity, as shown in the sidebar.
type State string

const (
	Idle          State = "idle"
	Starting      State = "starting"
	Thinking      State = "thinking"
	Writing       State = "writing"
	RunningTools  State = "running_tools"
	NeedsApproval State = "needs_approval"
	Unread        State = "unread"
	Errored       State = "error"
	Exited        State = "exited"
)

// View is what the UI sees of a session.
type View struct {
	store.Session
	State   State  `json:"state"`
	Running bool   `json:"running"` // a CLI process is alive
	Busy    bool   `json:"busy"`    // a turn is in flight
	Error   string `json:"error,omitempty"`
	// Approvals are tool uses waiting for the user's answer, oldest first.
	Approvals []Approval `json:"approvals"`
	// Unattended: requests that would wait for the user are declined.
	Unattended bool `json:"unattended"`
}

// Sink receives everything the UI needs. Implementations must not call
// back into the manager and must not block.
type Sink interface {
	SessionEvent(sessionID string, ev core.Event)
	SessionChanged(v View)
	PageChanged(p store.Page)
}

// next applies the activity state machine to one event. runningTools is
// the number of tools started but not finished on the open page.
func next(cur State, ev core.EventKind, runningTools int, turnOpen bool) State {
	switch ev {
	case core.EvSessionReady, core.EvTurnStarted, core.EvThinking:
		if !turnOpen {
			return cur
		}
		if cur == Starting || cur == Idle || cur == Unread || cur == Errored {
			return Thinking
		}
		return cur
	case core.EvTextDelta, core.EvTextBlock:
		if turnOpen && runningTools == 0 {
			return Writing
		}
	case core.EvToolStarted:
		return RunningTools
	case core.EvToolFinished:
		if runningTools == 0 {
			return Thinking
		}
		return RunningTools
	case core.EvApprovalAsked:
		return NeedsApproval
	}
	return cur
}
