import { describe, expect, it } from 'vitest'
import { back, forward, isTyping, nextBookmark, prevBookmark } from './nav'

const pages = (marks: number[], n = 6) => Array.from({ length: n }, (_, i) => ({ bookmarked: marks.includes(i) }))

describe('page navigation', () => {
  it('moves one page and stops at the ends', () => {
    expect(back(0)).toBe(0)
    expect(back(3)).toBe(2)
    expect(forward(2, 3)).toBe(2)
    expect(forward(0, 3)).toBe(1)
    expect(forward(0, 0)).toBe(0)
  })

  it('jumps to the nearest bookmark in each direction', () => {
    const p = pages([1, 4])
    expect(prevBookmark(p, 3)).toBe(1)
    expect(nextBookmark(p, 3)).toBe(4)
    expect(nextBookmark(p, 1)).toBe(4)
    expect(prevBookmark(p, 4)).toBe(1)
  })

  it('falls back to the first and last page', () => {
    const p = pages([2])
    expect(prevBookmark(p, 2)).toBe(0)
    expect(nextBookmark(p, 2)).toBe(5)
    expect(prevBookmark(pages([]), 3)).toBe(0)
    expect(nextBookmark(pages([]), 3)).toBe(5)
    expect(nextBookmark([], 0)).toBe(0)
  })

  it('ignores shortcuts while typing', () => {
    expect(isTyping(document.createElement('textarea'))).toBe(true)
    expect(isTyping(document.createElement('input'))).toBe(true)
    expect(isTyping(document.createElement('div'))).toBe(false)
    expect(isTyping(null)).toBe(false)
  })
})
