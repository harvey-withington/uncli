import { describe, expect, it } from 'vitest'
import { tintStyle } from './tint'

describe('tintStyle', () => {
  it('draws a hue at full saturation with the theme lightness', () => {
    expect(tintStyle(205)).toBe('--tint: hsl(205 100% var(--tint-l)); --tint-soft: hsl(205 100% var(--tint-l) / 0.14);')
    expect(tintStyle(0)).toBe('')
    expect(tintStyle(undefined)).toBe('')
  })
})
