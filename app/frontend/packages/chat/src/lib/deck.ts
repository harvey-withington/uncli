// The walkthrough of a page's changes (decision 0029): its recorded hunks
// as slides, one changed place a slide, each line numbered and marked
// added, removed or unchanged. Pure; deckhtml.ts renders it.
import type { FileChange, TouchedFile } from './api'
import { relPath } from './files'

export interface DeckLine {
  kind: 'add' | 'del' | 'ctx'
  text: string
  n: number // its line number: in the new file, or the old one for a removed line
}

export interface DeckSlide {
  place: string // c<change>h<hunk>: the change's index in pageChanges and the hunk's in it, as an explanation's notes name it
  path: string
  rel: string // relative to the session folder
  lines: DeckLine[]
  first: number // the first changed line, in the new file
  last: number
  added: number
  removed: number
  part: number // which of its file's changed places, from 1
  parts: number
}

export interface Deck {
  slides: DeckSlide[]
  files: number // how many files the slides cover
  others: string[] // files changed without a record (by a command, or past the cap), relative
}

// deckOf turns a page's recorded changes into slides: files in the order
// they were first changed, each file's places in order.
export function deckOf(changes: FileChange[], touched: TouchedFile[], workdir: string): Deck {
  const byFile = new Map<string, { c: FileChange; n: number }[]>()
  changes.forEach((c, n) => {
    const key = c.path.toLowerCase()
    byFile.set(key, [...(byFile.get(key) ?? []), { c, n }])
  })
  const slides: DeckSlide[] = []
  for (const list of byFile.values()) {
    const hunks = list.flatMap(({ c, n }) => c.hunks.map((h, k) => ({ ...h, place: `c${n}h${k}` })))
    hunks.forEach((h, i) => {
      let oldN = h.oldStart
      let newN = h.newStart
      const lines: DeckLine[] = []
      let first = 0
      let last = 0
      let added = 0
      let removed = 0
      for (const raw of h.lines) {
        const mark = raw[0]
        const text = raw.slice(1)
        if (mark === '+') {
          lines.push({ kind: 'add', text, n: newN })
          first ||= newN
          last = newN
          newN++
          added++
        } else if (mark === '-') {
          lines.push({ kind: 'del', text, n: oldN })
          first ||= newN
          last = Math.max(last, newN)
          oldN++
          removed++
        } else {
          lines.push({ kind: 'ctx', text: mark === ' ' ? text : raw, n: newN })
          oldN++
          newN++
        }
      }
      const path = list[0]!.c.path
      slides.push({ place: h.place, path, rel: relPath(workdir, path), lines, first: first || h.newStart, last: last || first || h.newStart, added, removed, part: i + 1, parts: hunks.length })
    })
  }
  const recorded = new Set(changes.map(c => c.path.toLowerCase()))
  const others = touched.filter(f => !recorded.has(f.path.toLowerCase())).map(f => relPath(workdir, f.path))
  return { slides, files: byFile.size, others }
}

// How many changed places a page has recorded, and in how many files.
export function recordedOf(touched: TouchedFile[]): { places: number; files: number } {
  const withHunks = touched.filter(f => (f.hunks ?? 0) > 0)
  return { places: withHunks.reduce((n, f) => n + (f.hunks ?? 0), 0), files: withHunks.length }
}
