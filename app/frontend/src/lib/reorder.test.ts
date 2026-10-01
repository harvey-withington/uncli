import { describe, expect, it } from 'vitest'
import { orderAt } from './reorder'
import { tintStyle } from './tint'

const list = [
  { id: 'a', sortOrder: 4 },
  { id: 'b', sortOrder: 3 },
  { id: 'c', sortOrder: 2 },
  { id: 'd', sortOrder: 1 },
]

const apply = (id: string, to: number) => {
  const o = orderAt(list, id, to)
  if (o === null) return null
  return list.map(s => (s.id === id ? { ...s, sortOrder: o } : s)).sort((x, y) => y.sortOrder - x.sortOrder).map(s => s.id).join('')
}

describe('orderAt', () => {
  it('moves down, up, to the top and to the bottom', () => {
    expect(apply('a', 2)).toBe('bacd') // drop between b and c
    expect(apply('d', 1)).toBe('adbc')
    expect(apply('c', 0)).toBe('cabd')
    expect(apply('a', 4)).toBe('bcda')
  })

  it('does nothing when dropped in place', () => {
    expect(orderAt(list, 'b', 1)).toBeNull()
    expect(orderAt(list, 'b', 2)).toBeNull()
    expect(orderAt(list, 'x', 0)).toBeNull()
  })
})

describe('tintStyle', () => {
  it('draws a hue at full saturation with the theme lightness', () => {
    expect(tintStyle(205)).toBe('--tint: hsl(205 100% var(--tint-l)); --tint-soft: hsl(205 100% var(--tint-l) / 0.14);')
    expect(tintStyle(0)).toBe('')
    expect(tintStyle(undefined)).toBe('')
  })
})
