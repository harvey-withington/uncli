package session

import (
	"encoding/json"

	"uncli/internal/adapter/claude"
	"uncli/internal/core"
)

// Tests speak Claude Code's tool names; the Claude adapter turns them into
// the provider-neutral actions the session judges.
func action(tool string, input json.RawMessage) core.ToolAction { return claude.ActionOf(tool, input) }

func (p policy) judgeTool(tool string, input json.RawMessage) verdict {
	return p.judge(action(tool, input))
}

func bashCommand(input json.RawMessage) string {
	var in struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(input, &in)
	return in.Command
}
