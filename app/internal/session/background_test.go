package session

import (
	"bytes"
	"strings"
	"testing"
	"time"
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
