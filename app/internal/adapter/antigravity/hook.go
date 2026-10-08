package antigravity

import (
	"encoding/json"
	"fmt"

	"uncli/internal/core"
)

// The CLI's hook calls (recorded in testdata/streams/antigravity/<version>/
// *.hook.jsonl). Every call carries the conversation; a PreToolUse call
// has the tool call and its step's index, the same index as on the stream.

// Hook events, as hooks.json passes them to UNCLI's hook command.
const (
	EventPreTool       = "pre-tool"
	EventPreInvocation = "pre-invocation"
)

type hookCall struct {
	ConversationID string `json:"conversationId"`
	StepIdx        int    `json:"stepIdx"`
	ToolCall       struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
}

func (a *Adapter) ReadHook(event string, payload []byte) (core.HookCall, error) {
	switch event {
	case EventPreInvocation:
		return core.HookCall{Kind: core.HookInstructions}, nil
	case EventPreTool:
		var c hookCall
		if err := json.Unmarshal(payload, &c); err != nil {
			return core.HookCall{}, fmt.Errorf("hook call: %w", err)
		}
		if c.ToolCall.Name == "" {
			return core.HookCall{}, fmt.Errorf("hook call names no tool")
		}
		id := ToolID(c.ConversationID, c.StepIdx)
		return core.HookCall{Kind: core.HookTool, Approval: core.ApprovalAsked{
			RequestID:   id,
			Tool:        c.ToolCall.Name,
			Input:       c.ToolCall.Args,
			Description: summary(c.ToolCall.Name, c.ToolCall.Args),
			ToolUseID:   id,
			Action:      ActionOf(c.ToolCall.Name, c.ToolCall.Args),
		}}, nil
	}
	return core.HookCall{}, fmt.Errorf("unknown hook event %q", event)
}

// AnswerHook writes UNCLI's answer as the CLI reads it: a decision for a
// tool call, and the instructions as a message the model sees before each
// call (it isn't kept in the conversation, so it applies afresh each time).
func (a *Adapter) AnswerHook(call core.HookCall, ans core.HookAnswer) ([]byte, error) {
	switch call.Kind {
	case core.HookTool:
		if ans.Allow {
			return json.Marshal(map[string]string{"decision": "allow"})
		}
		reason := ans.Reason
		if reason == "" {
			reason = "Denied in UNCLI."
		}
		return json.Marshal(map[string]string{"decision": "deny", "reason": reason})
	case core.HookInstructions:
		if ans.Instructions == "" {
			return []byte("{}"), nil
		}
		return json.Marshal(map[string]any{"injectSteps": []any{map[string]string{"ephemeralMessage": ans.Instructions}}})
	}
	return nil, fmt.Errorf("unknown hook call %q", call.Kind)
}
