import { describe, expect, it } from 'vitest'
import type { SearchHit } from './api'
import { blockMatching, cleanMarkdown, fold, groupHits, snippetParts } from './search'

describe('search helpers', () => {
  it('splits a snippet into marked and plain parts, as plain text', () => {
    expect(snippetParts('…at **\x01Belém\x02**, then the [\x01Jerónimos\x02](https://x) Monastery')).toEqual([
      { text: '…at ', mark: false },
      { text: 'Belém', mark: true },
      { text: ', then the ', mark: false },
      { text: 'Jerónimos', mark: true },
      { text: ' Monastery', mark: false },
    ])
    // Markup in the text stays text.
    expect(snippetParts('<b>\x01x\x02</b>')).toEqual([{ text: '<b>', mark: false }, { text: 'x', mark: true }, { text: '</b>', mark: false }])
  })

  it('reads markdown as one line of plain text', () => {
    expect(cleanMarkdown('## Saturday\n- Walk `Alfama`\n> **note**')).toBe('Saturday Walk Alfama note')
    expect(cleanMarkdown('| Time | Where |\n|---|---|\n| 10:00 | Belém |')).toBe('Time · Where · 10:00 · Belém')
  })

  it('finds the first block holding a term, ignoring case and accents', () => {
    const blocks = ['Intro.', 'Then the JERÓNIMOS monastery.', 'More jeronimos.']
    expect(blockMatching(blocks, ['jeronimos'])).toBe(1)
    expect(blockMatching(blocks, ['the jeronimos'])).toBe(1)
    expect(blockMatching(blocks, ['nothing'])).toBe(-1)
    expect(fold('Café')).toBe('cafe')
  })

  it('groups hits by session, in best-first order', () => {
    const hit = (pageId: string, sessionId: string): SearchHit => ({
      pageId, sessionId, sessionTitle: sessionId.toUpperCase(), profileId: 'chat', seq: 1, question: '', field: 'answer', snippet: '', bookmarked: false, startedAt: 0,
    })
    const g = groupHits([hit('1', 'b'), hit('2', 'a'), hit('3', 'b')])
    expect(g.map(x => [x.sessionId, x.hits.map(h => h.pageId)])).toEqual([['b', ['1', '3']], ['a', ['2']]])
  })
})
