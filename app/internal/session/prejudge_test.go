package session

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"uncli/internal/core"
	"uncli/internal/store"
)

// batchJudge records each server batch and judges every tool safe.
type batchJudge struct {
	mu      sync.Mutex
	batches []ToolsQuery
}

func (b *batchJudge) judge(_ context.Context, q ToolsQuery) (map[string]store.Judgement, error) {
	b.mu.Lock()
	b.batches = append(b.batches, q)
	b.mu.Unlock()
	out := map[string]store.Judgement{}
	for _, t := range q.Tools {
		out[t] = store.Judgement{Level: RiskRoutine, Model: "fake"}
	}
	return out, nil
}

func (b *batchJudge) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.batches)
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPrejudgeOneBatchPerServerAndAgainOnNewVersion(t *testing.T) {
	h := newHarness(t, "single-turn")
	b := &batchJudge{}
	h.m.d.JudgeTools = b.judge
	h.m.d.Judge = func(context.Context, JudgeQuery) (store.Judgement, error) {
		t.Error("a tool was judged on its own")
		return store.Judgement{}, nil
	}
	hints := map[string]core.ToolHint{
		"mcp__notes__touch_note": {Server: "notes", ServerVersion: "1.0.0"},
		"mcp__notes__tag_note":   {Server: "notes", ServerVersion: "1.0.0"},
		"mcp__notes__read_note":  {ReadOnly: true, Server: "notes", ServerVersion: "1.0.0"}, // labelled: not asked
		"mcp__mail__send":        {OpenWorld: true, Server: "mail", ServerVersion: "2"},     // labelled
		"mcp__cal__sync":         {Server: "cal", ServerVersion: "0.3"},
	}
	h.m.prejudge(hints, h.dir)
	waitFor(t, "two batches", func() bool { return b.count() == 2 })
	for _, q := range b.batches {
		switch q.Server {
		case "notes":
			if len(q.Tools) != 2 || q.Tools[0] != "mcp__notes__tag_note" || q.Version != "1.0.0" {
				t.Errorf("notes batch = %+v", q)
			}
		case "cal":
			if len(q.Tools) != 1 {
				t.Errorf("cal batch = %+v", q)
			}
		default:
			t.Errorf("unexpected batch %+v", q)
		}
	}
	waitFor(t, "judgements stored", func() bool {
		_, ok := h.m.judged(mcpJudgeKey("mcp__notes__touch_note", hints["mcp__notes__touch_note"]))
		return ok
	})

	// The same versions again: nothing to ask.
	h.m.prejudge(hints, h.dir)
	time.Sleep(50 * time.Millisecond)
	if b.count() != 2 {
		t.Errorf("asked again for the same version: %d batches", b.count())
	}

	// The tool's request is answered from the batch's judgement.
	p := policy{mode: ModeUnsafe, unknown: UnknownModel, workdir: h.dir, hints: hints, judged: h.m.judged}
	v := p.judge(core.ToolAction{Kind: core.ActMCP, Tool: "mcp__notes__touch_note", Input: json.RawMessage(`{}`)})
	if v.action != actionRun || v.why[0].By != "safe" || v.why[0].Judged != "fake" {
		t.Errorf("verdict = %+v", v)
	}

	// The notes server updates: its tools are judged again.
	next := map[string]core.ToolHint{
		"mcp__notes__touch_note": {Server: "notes", ServerVersion: "1.1.0"},
		"mcp__notes__tag_note":   {Server: "notes", ServerVersion: "1.1.0"},
	}
	h.m.prejudge(next, h.dir)
	waitFor(t, "a batch for the new version", func() bool { return b.count() == 3 })
	p.hints = next
	if v := p.judge(core.ToolAction{Kind: core.ActMCP, Tool: "mcp__notes__touch_note"}); v.action != actionJudge && v.action != actionRun {
		t.Errorf("after the update = %+v", v)
	}
}
