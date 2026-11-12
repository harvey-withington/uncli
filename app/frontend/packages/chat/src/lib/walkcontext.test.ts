import { describe, expect, it } from 'vitest'
import type { PageExplanation } from './api'
import type { Deck, DeckSlide } from './deck'
import { walkContext } from './walkcontext'

const slide = (place: string, rel: string, first: number, last: number, added: number, removed: number): DeckSlide =>
  ({ place, path: `C:\\work\\${rel}`, rel, lines: [], first, last, added, removed, part: 1, parts: 1 })

const deck: Deck = {
  slides: [slide('c0h0', 'src/parser.ts', 12, 18, 4, 1), slide('c1h0', 'README.md', 3, 3, 1, 0)],
  files: 2,
  others: ['package-lock.json'],
}

describe('a walkthrough handed to a new session', () => {
  it('carries the question, the summary and each place with its note', () => {
    const explanation = {
      model: 'haiku', context: 'page', detail: 'brief', summary: 'Makes the parser skip blank lines.', costUsd: 0.01, at: 0,
      notes: [{ place: 'c0h0', text: ' Skips blank lines before tokenising. ' }],
    } as PageExplanation
    const text = walkContext('Fix the flaky parser test\nIt fails on Windows.', deck, explanation)
    expect(text).toContain('> Fix the flaky parser test\n> It fails on Windows.')
    expect(text).toContain('What the changes do:\nMakes the parser skip blank lines.')
    expect(text).toContain('- src/parser.ts, lines 12-18 (+4 -1): Skips blank lines before tokenising.')
    expect(text).toContain('- README.md, line 3 (+1)\n')
    expect(text).toContain('Also changed (no record of how):\n- package-lock.json')
    expect(text).toMatch(/wait for what I'd like to do next\.$/)
  })

  it('works before anything was explained', () => {
    const text = walkContext('  ', { slides: [slide('c0h0', 'a.go', 1, 2, 0, 2)], files: 1, others: [] }, null)
    expect(text).toContain('> (none given)')
    expect(text).not.toContain('What the changes do')
    expect(text).toContain('- a.go, lines 1-2 (-2)')
    expect(text).not.toContain('Also changed')
  })
})
