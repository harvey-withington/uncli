import { describe, expect, it } from 'vitest'
import { compact, dayKey, fillDays, money, niceMax, periodSince } from './usage'

const at = (y: number, m: number, d: number, h = 12) => new Date(y, m - 1, d, h).getTime()

describe('periods and days', () => {
  it('starts a period at local midnight, today included', () => {
    const now = at(2026, 10, 7, 15)
    expect(dayKey(periodSince('7d', now)!)).toBe('2026-10-01')
    expect(new Date(periodSince('7d', now)!).getHours()).toBe(0)
    expect(dayKey(periodSince('30d', now)!)).toBe('2026-09-08')
    expect(periodSince('all', now)).toBeUndefined()
  })
  it('gives every day a row, zero where there were no turns', () => {
    const rows = fillDays([{ key: '2026-10-05', turns: 2, input: 1, output: 2, cacheRead: 0, cacheWrite: 0, costUsd: 0.5, durationMs: 0 }],
      at(2026, 10, 3), at(2026, 10, 7))
    expect(rows.map(r => r.key)).toEqual(['2026-10-03', '2026-10-04', '2026-10-05', '2026-10-06', '2026-10-07'])
    expect(rows.map(r => r.costUsd)).toEqual([0, 0, 0.5, 0, 0])
  })
  it('steps by date across a clock change', () => {
    // Late March: Europe's clocks go forward; whatever the test machine's zone, no day repeats or goes missing.
    const rows = fillDays([], at(2026, 3, 27), at(2026, 4, 2))
    expect(rows.map(r => r.key)).toEqual(['2026-03-27', '2026-03-28', '2026-03-29', '2026-03-30', '2026-03-31', '2026-04-01', '2026-04-02'])
  })
})

describe('axis and numbers', () => {
  it('rounds the axis to clean ticks', () => {
    expect(niceMax(0)).toEqual({ max: 1, step: 0.25 })
    expect(niceMax(3.7)).toEqual({ max: 4, step: 1 })
    expect(niceMax(130)).toEqual({ max: 150, step: 50 })
    expect(niceMax(1_840_000)).toEqual({ max: 2_000_000, step: 500_000 })
  })
  it('writes numbers and money short', () => {
    expect(compact(1284)).toBe('1,284')
    expect(compact(12_900)).toBe('12.9K')
    expect(compact(4_200_000)).toBe('4.2M')
    expect(money(0)).toBe('$0.00')
    expect(money(0.004)).toBe('<$0.01')
    expect(money(12.345)).toBe('$12.35')
    expect(money(1234.5)).toBe('$1,235')
  })
})
