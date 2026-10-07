package store

import (
	"math"
	"testing"
	"time"
)

func TestUsageSumsPages(t *testing.T) {
	s, _ := open(t)
	_ = s.CreateSession(&Session{ID: "a", Title: "Parser fix", Adapter: "claude", Runtime: "local", ProfileID: "code", Workdir: "/a", Model: "opus"})
	_ = s.CreateSession(&Session{ID: "b", Title: "Lisbon", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: "/b", Model: "sonnet"})
	day := func(d int) int64 {
		return time.Date(2026, 10, d, 12, 0, 0, 0, time.Local).UnixMilli()
	}
	seq := 0
	add := func(id, sess, model string, at int64, in, out int, cost float64, status string) {
		seq++
		p := &Page{ID: id, SessionID: sess, Seq: seq, Question: "q", Model: model, Status: status, StartedAt: at,
			InputTokens: in, OutputTokens: out, CacheRead: 100, CostUSD: cost, DurationMS: 1000}
		if err := s.SavePage(p); err != nil {
			t.Fatal(err)
		}
	}
	add("p1", "a", "opus", day(4), 10, 200, 0.50, "done")
	add("p2", "a", "opus", day(5), 20, 300, 0.75, "done")
	add("p3", "b", "sonnet", day(5), 5, 50, 0.05, "done")
	add("p4", "b", "sonnet", day(6), 5, 50, 0.05, "open") // still answering: not counted
	add("p0", "b", "haiku", day(1), 1, 1, 0.01, "done")   // before the period
	_ = s.SetPageOutline("p3", &PageOutline{Model: "haiku", CostUSD: 0.004, At: day(5), Sections: []OutlineSection{{Block: 0, Title: "x"}}})

	r, err := s.Usage(UsageQuery{Since: day(3)})
	if err != nil {
		t.Fatal(err)
	}
	if r.Totals.Turns != 3 || r.Totals.Output != 550 || r.Totals.CacheRead != 300 || math.Abs(r.Totals.CostUSD-1.30) > 1e-9 || r.Sessions != 2 {
		t.Errorf("totals = %+v, sessions %d", r.Totals, r.Sessions)
	}
	if len(r.ByDay) != 2 || r.ByDay[0].Key != "2026-10-04" || r.ByDay[1].Key != "2026-10-05" || r.ByDay[1].Turns != 2 {
		t.Errorf("by day = %+v", r.ByDay)
	}
	if len(r.ByModel) != 2 || r.ByModel[0].Key != "opus" || math.Abs(r.ByModel[0].CostUSD-1.25) > 1e-9 {
		t.Errorf("by model = %+v", r.ByModel)
	}
	if len(r.ByProfile) != 2 || r.ByProfile[0].Key != "code" {
		t.Errorf("by type = %+v", r.ByProfile)
	}
	if len(r.BySession) != 2 || r.BySession[0].Key != "a" || r.BySession[0].Label != "Parser fix" {
		t.Errorf("by session = %+v", r.BySession)
	}
	if r.QuickTasks != 1 || math.Abs(r.QuickTaskCostUSD-0.004) > 1e-9 {
		t.Errorf("quick tasks = %d, %v", r.QuickTasks, r.QuickTaskCostUSD)
	}
	if r.First != day(4) {
		t.Errorf("first = %d", r.First)
	}
	// All time includes the early page; an empty period has empty lists.
	if all, _ := s.Usage(UsageQuery{}); all.Totals.Turns != 4 {
		t.Errorf("all time = %+v", all.Totals)
	}
	if none, _ := s.Usage(UsageQuery{Since: day(20)}); none.Totals.Turns != 0 || none.ByDay == nil || len(none.ByDay) != 0 {
		t.Errorf("empty = %+v", none)
	}
}
