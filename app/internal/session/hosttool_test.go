package session

import (
	"testing"

	"github.com/harvey-withington/uncli/app/internal/core"
)

// A host decides about its own MCP servers' tools (decision 0027): run,
// refuse with its reason, or ask the user; anything it leaves, and every
// tool that isn't an MCP server's, is UNCLI's to decide.
func TestHostDecidesItsOwnTools(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	var asked []string
	h.m.d.HostTool = func(id, server, tool string) (string, string) {
		if id != v.ID {
			t.Errorf("asked about session %s", id)
		}
		asked = append(asked, server+"/"+tool)
		switch tool {
		case "get_card":
			return actionRun, ""
		case "update_card":
			return actionBlock, "Chat mode: the board can't be changed."
		case "delete_card":
			return actionPrompt, "Suggest mode"
		}
		return "", ""
	}
	s, _ := h.m.get(v.ID)
	p := s.policyLocked()
	mcp := func(name string) core.ToolAction { return core.ToolAction{Kind: core.ActMCP, Tool: name} }

	if got := s.verdictLocked(p, mcp("mcp__board__get_card")); got.action != actionRun || got.by != "host" {
		t.Errorf("get_card: %+v", got)
	}
	if got := s.verdictLocked(p, mcp("mcp__board__update_card")); got.action != actionBlock || got.reason != "Chat mode: the board can't be changed." {
		t.Errorf("update_card: %+v", got)
	}
	got := s.verdictLocked(p, mcp("mcp__board__delete_card"))
	if got.action != actionPrompt || len(got.why) != 1 || got.why[0].By != "host" || got.why[0].Note != "Suggest mode" {
		t.Errorf("delete_card: %+v", got)
	}
	// Left to UNCLI: decided by its own rules, not the host.
	if got := s.verdictLocked(p, mcp("mcp__board__other")); got.by == "host" {
		t.Errorf("other: the host decided %+v", got)
	}
	// Never asked about the CLI's own tools, or a name that isn't a tool's.
	s.verdictLocked(p, core.ToolAction{Kind: core.ActShell, Tool: "Bash", Command: "ls"})
	s.verdictLocked(p, mcp("mcp__board"))
	if want := []string{"board/get_card", "board/update_card", "board/delete_card", "board/other"}; len(asked) != len(want) {
		t.Errorf("asked about %v, want %v", asked, want)
	}
}

func TestMCPToolNames(t *testing.T) {
	for name, want := range map[string][2]string{
		"mcp__board__get_card":       {"board", "get_card"},
		"mcp__bruv-home__add_blocks": {"bruv-home", "add_blocks"},
	} {
		if s, tool, ok := mcpTool(name); !ok || s != want[0] || tool != want[1] {
			t.Errorf("%s = %s, %s, %v", name, s, tool, ok)
		}
	}
	for _, name := range []string{"Bash", "mcp__board", "mcp____x", "mcp__board__"} {
		if _, _, ok := mcpTool(name); ok {
			t.Errorf("%s read as an MCP tool", name)
		}
	}
}
