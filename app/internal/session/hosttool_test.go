package session

import (
	"encoding/json"
	"testing"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/store"
)

// A host decides about its own MCP servers' tools (decision 0027), seeing
// each call's arguments: run, refuse with its reason, or ask the user with
// its own words for the card; anything it leaves, and every tool that isn't
// an MCP server's, is UNCLI's to decide.
func TestHostDecidesItsOwnTools(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	var calls []HostToolCall
	h.m.d.HostTool = func(c HostToolCall) HostDecision {
		if c.SessionID != v.ID {
			t.Errorf("asked about session %s", c.SessionID)
		}
		calls = append(calls, c)
		switch c.Tool {
		case "get_card":
			return HostDecision{Decision: actionRun}
		case "update_card":
			return HostDecision{Decision: actionBlock, Reason: "Chat mode: the board can't be changed."}
		case "update_checklist":
			return HostDecision{Decision: actionPrompt, Reason: "Suggest mode", Prompt: &HostPrompt{Title: " Update checklist 'Preparation to Leave' ", Detail: "Adds: Passport\nTicks: Book taxi"}}
		case "delete_card":
			return HostDecision{Decision: actionPrompt, Prompt: &HostPrompt{Title: "  "}} // no words: the tool as UNCLI shows it
		}
		return HostDecision{}
	}
	s, _ := h.m.get(v.ID)
	p := s.policyLocked()
	mcp := func(name, input string) core.ToolAction {
		return core.ToolAction{Kind: core.ActMCP, Tool: name, Input: json.RawMessage(input)}
	}

	if got := s.verdictLocked(p, mcp("mcp__board__get_card", `{"id":"42"}`)); got.action != actionRun || got.by != "host" {
		t.Errorf("get_card: %+v", got)
	}
	if string(calls[0].Input) != `{"id":"42"}` {
		t.Errorf("the host didn't see the arguments: %s", calls[0].Input)
	}
	if got := s.verdictLocked(p, mcp("mcp__board__update_card", `{}`)); got.action != actionBlock || got.reason != "Chat mode: the board can't be changed." {
		t.Errorf("update_card: %+v", got)
	}
	got := s.verdictLocked(p, mcp("mcp__board__update_checklist", `{"checklist":"Preparation to Leave"}`))
	if got.action != actionPrompt || got.host == nil || got.host.Title != "Update checklist 'Preparation to Leave'" || got.host.Detail != "Adds: Passport\nTicks: Book taxi" {
		t.Errorf("update_checklist: %+v %+v", got, got.host)
	}
	// The card carries the host's words, and offers "This is safe" for the tool.
	var a Approval
	a.update(got)
	if a.Host == nil || a.Host.Title != "Update checklist 'Preparation to Leave'" {
		t.Errorf("the card has %+v", a.Host)
	}
	if len(a.Learn) != 1 || a.Learn[0].Kind != store.KindTool || a.Learn[0].Words != "mcp__board__update_checklist" {
		t.Errorf("This is safe would remember %+v", a.Learn)
	}
	if blank := s.verdictLocked(p, mcp("mcp__board__delete_card", `{}`)); blank.action != actionPrompt || blank.host != nil {
		t.Errorf("a prompt with no title: %+v", blank)
	}
	// Left to UNCLI: decided by its own rules, not the host.
	if got := s.verdictLocked(p, mcp("mcp__board__other", `{}`)); got.by == "host" {
		t.Errorf("other: the host decided %+v", got)
	}
	// Never asked about the CLI's own tools, or a name that isn't a tool's.
	n := len(calls)
	s.verdictLocked(p, core.ToolAction{Kind: core.ActShell, Tool: "Bash", Command: "ls"})
	s.verdictLocked(p, mcp("mcp__board", `{}`))
	if len(calls) != n {
		t.Errorf("asked about %d more", len(calls)-n)
	}
}

// The user's own "This is safe" on a host's card lasts: the tool, once on
// their safe list, runs without the host asking again. A host's refusal
// still wins.
func TestTheUsersSafeListBeatsAHostsAsk(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	refuse := false
	h.m.d.HostTool = func(c HostToolCall) HostDecision {
		if refuse {
			return HostDecision{Decision: actionBlock, Reason: "Chat mode"}
		}
		return HostDecision{Decision: actionPrompt, Prompt: &HostPrompt{Title: "Update the card"}}
	}
	if err := h.db.SetSafeEntry(store.SafeEntry{Kind: store.KindTool, Words: "mcp__board__update_card", Verdict: "safe"}); err != nil {
		t.Fatal(err)
	}
	s, _ := h.m.get(v.ID)
	call := core.ToolAction{Kind: core.ActMCP, Tool: "mcp__board__update_card", Input: json.RawMessage(`{}`)}
	if got := s.verdictLocked(s.policyLocked(), call); got.action != actionRun || got.by != "listed" {
		t.Errorf("a safe-listed tool the host asks about: %+v", got)
	}
	refuse = true
	if got := s.verdictLocked(s.policyLocked(), call); got.action != actionBlock {
		t.Errorf("the host refused, but %+v", got)
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
