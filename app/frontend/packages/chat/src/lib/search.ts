// Search across sessions: what the store's index returns, shaped for the
// sidebar. Snippets mark matches with \x01 … \x02 (never HTML), so they
// are split into parts and highlighted without injecting markup.
import type { SearchHit } from './api'

export const MARK_OPEN = '\x01'
export const MARK_CLOSE = '\x02'

export type SearchRange = 'any' | 'week' | 'month' | 'year'
export const RANGE_DAYS: Record<SearchRange, number> = { any: 0, week: 7, month: 30, year: 365 }

export interface SnippetPart {
  text: string
  mark: boolean
}

export function snippetParts(snippet: string): SnippetPart[] {
  const out: SnippetPart[] = []
  let mark = false
  let buf = ''
  for (const ch of cleanMarkdown(snippet)) {
    if (ch === MARK_OPEN || ch === MARK_CLOSE) {
      if (buf) out.push({ text: buf, mark })
      buf = ''
      mark = ch === MARK_OPEN
    } else {
      buf += ch
    }
  }
  if (buf) out.push({ text: buf, mark })
  return out
}

// cleanMarkdown makes an answer's markdown read as plain text in a
// one-line snippet: no emphasis, code ticks, heading or quote markers, or
// link targets, and no line breaks.
export function cleanMarkdown(s: string): string {
  return s
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/(\*\*|__|`+|~~)/g, '')
    .replace(/\|?[\s:]*-{3,}[\s:|-]*/g, ' ') // table separator rows
    .replace(/\s*\|\s*/g, ' · ') // table cells read as a list
    .replace(/^ · | · $/g, '')
    .replace(/(^|\n)\s*(#{1,6}|>|[-*+]|\d+\.)\s+/g, '$1')
    .replace(/\s+/g, ' ')
    .trim()
}

// fold lowercases and drops accents, as the index does.
export function fold(s: string): string {
  return s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()
}

// blockMatching is the first block whose text holds any of the terms (a
// word, or a phrase), or -1.
export function blockMatching(texts: readonly string[], terms: readonly string[]): number {
  const want = terms.map(fold).filter(Boolean)
  if (want.length === 0) return -1
  return texts.findIndex(t => {
    const f = fold(t)
    return want.some(w => f.includes(w))
  })
}

export interface HitGroup {
  sessionId: string
  title: string
  profileId: string
  hits: SearchHit[]
}

// groupHits groups results by session, keeping the best-first order: a
// session appears where its best page ranks.
export function groupHits(hits: readonly SearchHit[]): HitGroup[] {
  const groups: HitGroup[] = []
  const at = new Map<string, HitGroup>()
  for (const h of hits) {
    let g = at.get(h.sessionId)
    if (!g) {
      g = { sessionId: h.sessionId, title: h.sessionTitle, profileId: h.profileId, hits: [] }
      at.set(h.sessionId, g)
      groups.push(g)
    }
    g.hits.push(h)
  }
  return groups
}
