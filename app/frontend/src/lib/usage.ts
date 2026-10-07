// The usage dashboard's logic: periods, a day for every date in the period
// (days without turns are zero, not missing), the chart's measure and its
// axis.
import type { UsageRow, UsageTotals } from './api'

export type Period = '7d' | '30d' | 'all'
export type Measure = 'cost' | 'tokens'

const DAY = 86_400_000

// startOfDay is local midnight of the day ms falls in.
export function startOfDay(ms: number): number {
  const d = new Date(ms)
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

// periodSince is where a period starts: today and the days before it.
export function periodSince(p: Period, now = Date.now()): number | undefined {
  if (p === 'all') return undefined
  const days = p === '7d' ? 7 : 30
  return startOfDay(now) - (days - 1) * DAY
}

// dayKey is a local date as the store groups it: "2026-10-06".
export function dayKey(ms: number): string {
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const empty: UsageTotals = { turns: 0, input: 0, output: 0, cacheRead: 0, cacheWrite: 0, costUsd: 0, durationMs: 0 }

// fillDays gives every day from `from` to `to` (local, inclusive) a row,
// zero where the store had none. For "all time", from is the first page.
export function fillDays(rows: readonly UsageRow[], from: number, to: number): UsageRow[] {
  const have = new Map(rows.map(r => [r.key, r]))
  const out: UsageRow[] = []
  // Step by date, not by 24 h, so a daylight-saving change can't skip or repeat a day.
  const d = new Date(startOfDay(from))
  const end = startOfDay(to)
  while (d.getTime() <= end && out.length < 3660) {
    const key = dayKey(d.getTime())
    out.push(have.get(key) ?? { key, ...empty })
    d.setDate(d.getDate() + 1)
  }
  return out
}

// allTokens is everything a turn sent and received: what was read from the
// cache counts too, since it is most of what a long conversation re-reads.
export function allTokens(t: UsageTotals): number {
  return t.input + t.output + t.cacheRead + t.cacheWrite
}

export function measureOf(t: UsageTotals, m: Measure): number {
  return m === 'cost' ? t.costUsd : allTokens(t)
}

// niceMax rounds an axis maximum up to 1, 2 or 5 × a power of ten, and
// gives the tick step (four ticks above zero, or fewer).
export function niceMax(max: number): { max: number; step: number } {
  if (!(max > 0)) return { max: 1, step: 0.25 }
  const raw = max / 4
  const pow = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map(m => m * pow).find(s => s >= raw) ?? 10 * pow
  return { max: step * Math.ceil(max / step), step }
}

// compact writes a number short: 1,284 / 12.9K / 4.2M.
export function compact(n: number): string {
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e4) return `${(n / 1e3).toFixed(1)}K`
  return Math.round(n).toLocaleString('en')
}

// money writes a cost for the dashboard: cents below $100, whole dollars above.
export function money(usd: number): string {
  if (usd >= 100) return `$${Math.round(usd).toLocaleString('en')}`
  if (usd > 0 && usd < 0.01) return '<$0.01'
  return `$${usd.toFixed(2)}`
}
