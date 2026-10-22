package session

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"uncli/internal/core"
)

// A background sub-agent finishes after the turn that started it: the CLI
// carries on by itself in a turn of its own (fixture subagent-background,
// recorded 2026-10-07). Its answer gets a page of its own, started by the
// CLI, instead of being dropped.
func TestCLIStartedTurnGetsAPage(t *testing.T) {
	h := newHarness(t, "subagent-background")
	// The CLI sends both turns after the one user message.
	h.rt.turns = [][]byte{bytes.Join(h.rt.turns, nil)}
	v, _ := h.m.Create("chat", "", "haiku")
	if err := h.m.Send(t.Context(), v.ID, "start a background agent and wait for it"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var pages []string
	for time.Now().Before(deadline) {
		ps, _ := h.m.Pages(v.ID)
		if len(ps) == 2 && ps[1].Status == "done" {
			if ps[0].Status != "done" || !strings.Contains(ps[0].AnswerMD, "launched a background subagent") {
				t.Errorf("first page = %+v", ps[0])
			}
			if ps[1].Origin != OriginCLI || ps[1].Question != "" || !strings.Contains(ps[1].AnswerMD, "PING") {
				t.Errorf("CLI page = %+v", ps[1])
			}
			if vw, _ := h.m.View(v.ID); vw.Busy || len(vw.Background) != 0 {
				t.Errorf("after both turns: busy %v, background %+v", vw.Busy, vw.Background)
			}
			return
		}
		pages = pages[:0]
		for _, p := range ps {
			pages = append(pages, p.Status+":"+p.Origin)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pages never settled: %v", pages)
}

// While the agent runs, the session says what runs in the background.
func TestBackgroundTasksOnTheView(t *testing.T) {
	h := newHarness(t, "subagent-background")
	h.rt.turns = [][]byte{bytes.Join(h.rt.turns, nil)}
	sink := &bgSink{recSink: h.sink}
	h.m.d.Sink = sink
	v, _ := h.m.Create("chat", "", "haiku")
	_ = h.m.Send(t.Context(), v.ID, "go")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !sink.saw() {
		time.Sleep(10 * time.Millisecond)
	}
	if !sink.saw() {
		t.Fatal("the running sub-agent never showed on the session")
	}
}

type bgSink struct {
	*recSink
	seen bool
}

func (s *bgSink) SessionChanged(v View) {
	s.recSink.SessionChanged(v)
	if len(v.Background) == 1 && v.Background[0].Description == "Simple ping response" && v.Background[0].Kind == "local_agent" {
		s.mu.Lock()
		s.seen = true
		s.mu.Unlock()
	}
}

func (s *bgSink) saw() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen
}

// A sub-agent the main agent waits for is one turn, one page; its own
// lines stay out of the answer (fixture subagent-foreground).
func TestForegroundSubagentIsOnePage(t *testing.T) {
	h := newHarness(t, "subagent-foreground")
	v, _ := h.m.Create("chat", "", "haiku")
	pages := h.send(v.ID, "use an agent")
	if len(pages) != 1 || !strings.Contains(pages[0].AnswerMD, "PONG") || strings.Contains(pages[0].AnswerMD, "Reply with exactly") {
		t.Errorf("pages = %+v", pages)
	}
	if len(pages[0].Trace) != 1 || pages[0].Trace[0].Name != "Agent" || !pages[0].Trace[0].OK {
		t.Errorf("trace = %+v", pages[0].Trace)
	}
}

// A sub-agent still running in the background asks for a tool after the
// turn that started it ended: the request opens the CLI's own page and is
// judged there like any other, instead of being refused for want of a turn.
func TestBackgroundRequestOpensAPage(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	h.send(v.ID, "hi")
	s, _ := h.m.get(v.ID)
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	s.handle(gen, core.NewEvent(core.EvBackground, core.BackgroundTasks{Tasks: []core.BackgroundTask{{ID: "a1", Kind: "local_agent", Description: "Check the board"}}}, nil))
	s.handle(gen, core.NewEvent(core.EvApprovalAsked, core.ApprovalAsked{RequestID: "r1", Tool: "Bash",
		Input: json.RawMessage(`{"command":"rm -rf build"}`), Action: core.ToolAction{Kind: core.ActShell, Tool: "Bash", Command: "rm -rf build"}}, nil))
	for _, l := range h.rt.lines() {
		if strings.Contains(l, "There is no turn") {
			t.Fatalf("refused: %s", l)
		}
	}
	vw, _ := h.m.View(v.ID)
	pages, _ := h.m.Pages(v.ID)
	if !vw.Busy || len(pages) != 2 || pages[1].Origin != OriginCLI {
		t.Errorf("busy %v, pages %d", vw.Busy, len(pages))
	}

	// With nothing in the background, a request outside a turn is still refused.
	h2 := newHarness(t, "single-turn")
	v2, _ := h2.m.Create("chat", "", "haiku")
	h2.send(v2.ID, "hi")
	s2, _ := h2.m.get(v2.ID)
	s2.mu.Lock()
	gen2 := s2.gen
	s2.mu.Unlock()
	s2.handle(gen2, core.NewEvent(core.EvApprovalAsked, core.ApprovalAsked{RequestID: "r2", Tool: "Bash", Action: core.ToolAction{Kind: core.ActShell, Tool: "Bash", Command: "ls"}}, nil))
	if !slices.ContainsFunc(h2.rt.lines(), func(l string) bool { return strings.Contains(l, "There is no turn") }) {
		t.Error("a request with no turn and nothing in the background should be refused")
	}
}
