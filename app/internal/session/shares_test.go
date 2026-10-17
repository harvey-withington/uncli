package session

import (
	"slices"
	"strings"
	"testing"

	"uncli/internal/profile"
	"uncli/internal/store"
)

// A chat starts with its type's shares or the ones chosen, keeps the
// choice, and a change restarts the CLI on the conversation at the next
// turn, with the new flags.
func TestSharesChooseAndChange(t *testing.T) {
	h := newHarness(t, "multi-turn-partial")
	both := []string{profile.ShareTools, profile.ShareSkills}
	v, _ := h.m.Create("chat", "", "sonnet")
	if !slices.Equal(v.Shareable, both) || !slices.Equal(v.Shares, both) {
		t.Errorf("default chat: shares %v of %v", v.Shares, v.Shareable)
	}

	v, err := h.m.CreateWith(NewSession{Profile: "chat", Model: "sonnet", Shares: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Shares) != 0 {
		t.Errorf("chosen none, shares %v", v.Shares)
	}
	if rec, _ := h.db.GetSession(v.ID); rec.ShareChoice == nil || len(rec.ShareChoice) != 0 {
		t.Errorf("stored choice = %#v, want empty (not nil: nil is the type's)", rec.ShareChoice)
	}
	h.send(v.ID, "one")
	first := strings.Join(h.rt.starts[0].Args, " ")
	if !strings.Contains(first, "--strict-mcp-config") || !strings.Contains(first, "--disable-slash-commands") {
		t.Errorf("sharing nothing: %s", first)
	}

	if v, err = h.m.SetShares(v.ID, []string{profile.ShareTools}); err != nil || !slices.Equal(v.Shares, []string{profile.ShareTools}) {
		t.Fatalf("SetShares: %v %v", v.Shares, err)
	}
	h.send(v.ID, "two")
	if len(h.rt.starts) != 2 {
		t.Fatalf("a change of shares should respawn: %d starts", len(h.rt.starts))
	}
	second := strings.Join(h.rt.starts[1].Args, " ")
	if strings.Contains(second, "--strict-mcp-config") || !strings.Contains(second, "--resume") || !strings.Contains(second, "ToolSearch") {
		t.Errorf("respawn must resume, sharing the tools: %s", second)
	}
}

// Only an isolated type on this machine has anything to share.
func TestSharesOnlyWhereTheyApply(t *testing.T) {
	h := newHarness(t, "single-turn")
	code, _ := h.m.Create("code", t.TempDir(), "opus")
	if len(code.Shareable) != 0 || len(code.Shares) != 0 {
		t.Errorf("code: %v of %v", code.Shares, code.Shareable)
	}
	if _, err := h.m.SetShares(code.ID, []string{profile.ShareSkills}); err == nil {
		t.Error("a code session can't share skills: it has the user's settings already")
	}
	if _, err := h.m.CreateWith(NewSession{Profile: "chat", Model: "sonnet", Shares: []string{"everything"}}); err == nil {
		t.Error("an unknown share must be refused")
	}
	inContainer := store.Session{ProfileID: "chat", Runtime: RuntimeWSL}
	if got := h.m.shareable(inContainer, h.m.d.Adapter.Capabilities()); len(got) != 0 {
		t.Errorf("in a container: %v", got)
	}
}
