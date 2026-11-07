import { describe, expect, it } from 'vitest'
import { deckOf, recordedOf } from './deck'

const W = 'C:\\w'

describe('walkthrough deck', () => {
  it('makes a slide per changed place, numbering and marking each line', () => {
    const deck = deckOf([
      { path: `${W}\\a.go`, hunks: [{ oldStart: 10, newStart: 10, lines: [' x', '-old', '+new', '+more', ' y'] }] },
      { path: `${W}\\b.go`, hunks: [{ oldStart: 0, newStart: 1, lines: ['+one', '+two'] }] },
      { path: `${W}\\A.go`, hunks: [{ oldStart: 40, newStart: 41, lines: [' z', '-gone'] }] }, // the same file, again
    ], [
      { path: `${W}\\a.go`, how: 'edit', hunks: 2 },
      { path: `${W}\\b.go`, how: 'write', hunks: 1 },
      { path: `${W}\\built.js`, how: 'command' },
    ], W)
    expect(deck.files).toBe(2)
    expect(deck.slides.map(s => [s.rel, s.part, s.parts, s.place])).toEqual([['a.go', 1, 2, 'c0h0'], ['a.go', 2, 2, 'c2h0'], ['b.go', 1, 1, 'c1h0']])
    const a = deck.slides[0]!
    expect(a.lines).toEqual([
      { kind: 'ctx', text: 'x', n: 10 },
      { kind: 'del', text: 'old', n: 11 },
      { kind: 'add', text: 'new', n: 11 },
      { kind: 'add', text: 'more', n: 12 },
      { kind: 'ctx', text: 'y', n: 13 },
    ])
    expect([a.first, a.last, a.added, a.removed]).toEqual([11, 12, 2, 1])
    expect([deck.slides[2]!.first, deck.slides[2]!.last]).toEqual([1, 2])
    expect(deck.others).toEqual(['built.js'])
  })

  it('counts what a page recorded', () => {
    expect(recordedOf([{ path: 'a', how: 'edit', hunks: 2 }, { path: 'b', how: 'write', hunks: 1 }, { path: 'c', how: 'command' }])).toEqual({ places: 3, files: 2 })
    expect(recordedOf([{ path: 'c', how: 'command' }])).toEqual({ places: 0, files: 0 })
  })
})
