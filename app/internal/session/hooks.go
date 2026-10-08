package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"uncli/internal/core"
)

// Hook approvals (decision 0012): a CLI with HookApprovals asks about each
// tool call by running UNCLI's hook, which reaches this session through
// Deps.Hooks. The call goes through the same approvals as one asked on the
// CLI's output (askLocked), and the answer goes back to the waiting hook
// instead of stdin. Whatever still waits when the turn or the process ends
// is refused, so a hook never outlives its process's turn.

// hookReply is the answer a waiting hook gets.
type hookReply struct {
	allow  bool
	reason string
}

const hookStopped = "The user stopped this turn."

// hookLocked registers the process about to start with the hook server and
// returns the environment that leads its hooks here. Caller holds s.mu.
func (s *Session) hookLocked(spec *core.LaunchSpec, ad core.Adapter) error {
	if !ad.Capabilities().HookApprovals || !spec.Approvals {
		return nil
	}
	ha, ok := ad.(core.HookAdapter)
	if !ok || s.m.d.Hooks == nil {
		return errors.New("this UNCLI can't approve this CLI's tool calls")
	}
	gen := s.gen + 1 // the process about to start (spawn increments it)
	env, unregister := s.m.d.Hooks.Register(func(ctx context.Context, event string, payload []byte) ([]byte, error) {
		return s.hookCall(ctx, gen, ha, event, payload)
	})
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	for k, v := range env {
		spec.Env[k] = v
	}
	s.unhook = unregister
	s.instructions = strings.TrimSpace(strings.Join([]string{spec.SystemPrompt, spec.AppendPrompt}, "\n\n"))
	return nil
}

// hookCall answers one call from the process's hook.
func (s *Session) hookCall(ctx context.Context, gen int, ha core.HookAdapter, event string, payload []byte) ([]byte, error) {
	call, err := ha.ReadHook(event, payload)
	if err != nil {
		return nil, err
	}
	if call.Kind == core.HookInstructions {
		s.mu.Lock()
		ins := s.instructions
		s.mu.Unlock()
		return ha.AnswerHook(call, core.HookAnswer{Instructions: ins})
	}
	if call.Kind != core.HookTool {
		return nil, fmt.Errorf("unknown hook call %q", call.Kind)
	}
	a := call.Approval
	ch := make(chan hookReply, 1)
	s.mu.Lock()
	if gen != s.gen || s.proc == nil {
		s.mu.Unlock()
		return ha.AnswerHook(call, core.HookAnswer{Reason: hookStopped})
	}
	if s.hookWaits == nil {
		s.hookWaits = map[string]chan hookReply{}
	}
	for base, n := a.RequestID, 2; s.hookWaits[a.RequestID] != nil; n++ { // a step asked again
		a.RequestID = fmt.Sprintf("%s#%d", base, n)
	}
	s.hookWaits[a.RequestID] = ch
	s.mu.Unlock()

	// As if the CLI had asked on its output: logged, judged, maybe a card.
	s.handle(gen, core.NewEvent(core.EvApprovalAsked, a, payload))

	select {
	case r := <-ch:
		return ha.AnswerHook(call, core.HookAnswer{Allow: r.allow, Reason: r.reason})
	case <-ctx.Done():
		// The hook gave up (the CLI stopped it): it no longer waits.
		s.mu.Lock()
		delete(s.hookWaits, a.RequestID)
		for i, p := range s.pending {
			if p.RequestID == a.RequestID {
				s.pending = append(s.pending[:i:i], s.pending[i+1:]...)
				s.resumeLocked()
				s.changed()
				break
			}
		}
		s.mu.Unlock()
		return nil, ctx.Err()
	}
}

// replyHookLocked answers a waiting hook; false when none waits for this
// request. Caller holds s.mu.
func (s *Session) replyHookLocked(requestID string, allow bool, reason string) bool {
	ch, ok := s.hookWaits[requestID]
	if !ok {
		return false
	}
	delete(s.hookWaits, requestID)
	ch <- hookReply{allow: allow, reason: reason}
	return true
}

// releaseHooksLocked refuses every waiting hook (the turn or the process
// ended). Caller holds s.mu.
func (s *Session) releaseHooksLocked(reason string) {
	for id := range s.hookWaits {
		s.replyHookLocked(id, false, reason)
	}
}

// unhookLocked ends the process's registration: later calls from its
// hooks are refused. Caller holds s.mu.
func (s *Session) unhookLocked() {
	s.releaseHooksLocked(hookStopped)
	if s.unhook != nil {
		s.unhook()
		s.unhook = nil
	}
}

// missingAdapter stands in for a provider this UNCLI no longer has: the
// session can still be read, but not started.
type missingAdapter struct {
	core.Adapter
	id string
}

func (m missingAdapter) ID() string { return m.id }

func (m missingAdapter) BuildCommand(string, core.LaunchSpec) (core.Command, error) {
	return core.Command{}, fmt.Errorf("this session ran on %q, which this UNCLI doesn't have", m.id)
}
