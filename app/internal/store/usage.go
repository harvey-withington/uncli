package store

import (
	"database/sql"
	"fmt"
)

// Usage sums what pages record of each turn (tokens, cost, time) for the
// usage dashboard: totals, by day, by model, by session type and by
// session. Nothing new is recorded for it. Open pages aren't counted yet.

// UsageQuery picks a period by when pages started, in ms; zero is open.
type UsageQuery struct {
	Since int64 `json:"since,omitempty"`
	Until int64 `json:"until,omitempty"`
}

type UsageTotals struct {
	Turns      int     `json:"turns"`
	Input      int64   `json:"input"`
	Output     int64   `json:"output"`
	CacheRead  int64   `json:"cacheRead"`
	CacheWrite int64   `json:"cacheWrite"`
	CostUSD    float64 `json:"costUsd"`
	DurationMS int64   `json:"durationMs"`
}

// UsageRow is one group: a day ("2026-10-06", local time), a model, a
// session type (profile id) or a session (Label is its title).
type UsageRow struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
	UsageTotals
}

type UsageReport struct {
	Totals    UsageTotals `json:"totals"`
	ByDay     []UsageRow  `json:"byDay"`     // oldest first; days without turns are left out
	ByModel   []UsageRow  `json:"byModel"`   // costliest first
	ByProfile []UsageRow  `json:"byProfile"` // costliest first
	BySession []UsageRow  `json:"bySession"` // costliest first, at most SessionsShown
	Sessions  int         `json:"sessions"`  // sessions with turns in the period
	// Quick tasks (page summaries) are paid apart from turns.
	QuickTasks       int     `json:"quickTasks"`
	QuickTaskCostUSD float64 `json:"quickTaskCostUsd"`
	First            int64   `json:"first,omitempty"` // when the first counted page started, ms
}

// SessionsShown is how many sessions the report lists.
const SessionsShown = 10

const usageSums = `COUNT(*), COALESCE(SUM(p.input_tokens),0), COALESCE(SUM(p.output_tokens),0),
	COALESCE(SUM(p.cache_read),0), COALESCE(SUM(p.cache_write),0), COALESCE(SUM(p.cost_usd),0), COALESCE(SUM(p.duration_ms),0)`

func (s *Store) Usage(q UsageQuery) (UsageReport, error) {
	where := `p.status != 'open' AND COALESCE(p.started_at,0) >= ? AND (? = 0 OR p.started_at < ?)`
	args := []any{q.Since, q.Until, q.Until}
	from := ` FROM pages p JOIN sessions s ON s.id = p.session_id WHERE ` + where
	r := UsageReport{ByDay: []UsageRow{}, ByModel: []UsageRow{}, ByProfile: []UsageRow{}, BySession: []UsageRow{}}

	row := s.db.QueryRow(`SELECT `+usageSums+`, COUNT(DISTINCT p.session_id), COALESCE(MIN(p.started_at),0)`+from, args...)
	if err := row.Scan(&r.Totals.Turns, &r.Totals.Input, &r.Totals.Output, &r.Totals.CacheRead, &r.Totals.CacheWrite,
		&r.Totals.CostUSD, &r.Totals.DurationMS, &r.Sessions, &r.First); err != nil {
		return r, err
	}
	groups := []struct {
		into  *[]UsageRow
		key   string
		label string
		order string
	}{
		{&r.ByDay, `date(p.started_at/1000, 'unixepoch', 'localtime')`, `''`, `1`},
		{&r.ByModel, `p.model`, `''`, `7 DESC, 3 DESC`},
		{&r.ByProfile, `s.profile_id`, `''`, `7 DESC, 3 DESC`},
		{&r.BySession, `p.session_id`, `COALESCE(MAX(s.title),'')`, fmt.Sprintf(`7 DESC, 3 DESC LIMIT %d`, SessionsShown)},
	}
	for _, g := range groups {
		rows, err := s.db.Query(`SELECT `+g.key+`, `+g.label+`, `+usageSums+from+` GROUP BY 1 ORDER BY `+g.order, args...)
		if err != nil {
			return r, err
		}
		if err := scanUsage(rows, g.into); err != nil {
			return r, err
		}
	}
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(json_extract(outline, '$.costUsd')),0) FROM pages
		WHERE outline IS NOT NULL AND outline != '' AND COALESCE(json_extract(outline, '$.at'),0) >= ? AND (? = 0 OR json_extract(outline, '$.at') < ?)`,
		args...).Scan(&r.QuickTasks, &r.QuickTaskCostUSD)
	return r, err
}

func scanUsage(rows *sql.Rows, into *[]UsageRow) error {
	defer rows.Close()
	for rows.Next() {
		var u UsageRow
		var key sql.NullString
		if err := rows.Scan(&key, &u.Label, &u.Turns, &u.Input, &u.Output, &u.CacheRead, &u.CacheWrite, &u.CostUSD, &u.DurationMS); err != nil {
			return err
		}
		u.Key = key.String
		*into = append(*into, u)
	}
	return rows.Err()
}
