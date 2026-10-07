// A usage report for the mock: 45 days of made-up but steady history
// (quiet weekends, a busy week), so the dashboard can be seen and tested
// without a database. The same numbers every run.
import { dayKey, startOfDay } from '../usage'
import type { UsageQuery, UsageReport, UsageRow, UsageTotals } from './types'

const DAY = 86_400_000
const zero = (): UsageTotals => ({ turns: 0, input: 0, output: 0, cacheRead: 0, cacheWrite: 0, costUsd: 0, durationMs: 0 })

function add(a: UsageTotals, b: UsageTotals) {
  a.turns += b.turns; a.input += b.input; a.output += b.output; a.cacheRead += b.cacheRead
  a.cacheWrite += b.cacheWrite; a.costUsd += b.costUsd; a.durationMs += b.durationMs
}

const MODELS = [['opus', 0.55], ['sonnet', 0.35], ['haiku', 0.1]] as const
const TYPES = [['code', 0.6], ['chat', 0.25], ['cowork', 0.15]] as const
const SESSIONS = [
  ['s-code', 'Fix the flaky parser test', 0.34], ['s-chat', 'Plan a weekend in Lisbon', 0.18],
  ['s-cowork', 'Summarise the Q3 planning notes', 0.12], ['s-old', 'Compare three standing desks', 0.08],
] as const

export function mockUsageReport(q: UsageQuery, now = Date.now()): UsageReport {
  const today = startOfDay(now)
  const totals = zero()
  const byDay: UsageRow[] = []
  let first = 0
  for (let i = 44; i >= 0; i--) {
    const at = today - i * DAY + 10 * 3600_000
    if ((q.since && at < q.since) || (q.until && at >= q.until)) continue
    const weekend = [0, 6].includes(new Date(at).getDay())
    const wave = 1 + Math.sin(i / 3)
    const turns = weekend ? (i % 3) : Math.round(6 + 5 * wave + (i < 9 && i > 3 ? 8 : 0))
    if (turns === 0) continue
    const t: UsageTotals = {
      turns, input: turns * 900, output: turns * 1400, cacheRead: turns * 42_000, cacheWrite: turns * 3_100,
      costUsd: Math.round(turns * (0.09 + 0.03 * wave) * 100) / 100, durationMs: turns * 21_000,
    }
    add(totals, t)
    byDay.push({ key: dayKey(at), ...t })
    first ||= at
  }
  const share = (rows: readonly (readonly [string, number] | readonly [string, string, number])[]): UsageRow[] =>
    rows.map(r => {
      const f = r[r.length - 1] as number
      const row: UsageRow = { key: r[0], ...zero() }
      if (r.length === 3) row.label = r[1] as string
      for (const k of Object.keys(totals) as (keyof UsageTotals)[]) row[k] = k === 'costUsd' ? Math.round(totals[k] * f * 100) / 100 : Math.round(totals[k] * f)
      return row
    })
  return {
    totals, byDay, byModel: share(MODELS), byProfile: share(TYPES), bySession: share(SESSIONS),
    sessions: totals.turns ? SESSIONS.length : 0, quickTasks: Math.round(totals.turns / 9),
    quickTaskCostUsd: Math.round(totals.turns * 0.0007 * 100) / 100, first: first || undefined,
  }
}
