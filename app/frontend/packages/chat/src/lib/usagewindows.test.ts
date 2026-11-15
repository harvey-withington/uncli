import { describe, expect, it } from 'vitest'
import { windowLabel } from './usagewindows'

describe('a usage window’s name', () => {
  it('is the one UNCLI gives it', () => {
    expect(windowLabel('five_hour')).toBe('Current session (5 hours)')
    expect(windowLabel('seven_day')).toBe('This week')
    expect(windowLabel('seven_day_overage_included')).toBe('This week (with extra usage)')
  })

  it('is made from its parts when UNCLI has none, never the raw key', () => {
    expect(windowLabel('seven_day_haiku')).toBe('This week (haiku)')
    expect(windowLabel('five_hour_burst')).toBe('Current session (5 hours) (burst)')
    expect(windowLabel('monthly_cap')).toBe('Monthly cap')
  })
})
