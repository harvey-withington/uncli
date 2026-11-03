// The outline of an answer: its headings, pointing at the rendered block
// that holds each one, so the side panel can jump to and track them.
import type { AutoSummary } from './api'
import type { MdBlock } from './render/markdown'
import { classifySection, isKind, type SectionKind } from './sections'

export interface OutlineEntry {
  block: number // index of the heading's block in the answer
  level: number // 1 = top level, after normalising
  text: string
  kind: SectionKind
}

// outlineOf lists the headings of an answer. Levels are normalised so the
// shallowest heading used is level 1 (answers often start at ##).
// Paragraphs acting as headings ("**1. Point.** …") sit one level below the
// real heading before them, or at the top level when there is none.
export function outlineOf(blocks: readonly MdBlock[]): OutlineEntry[] {
  const raw = blocks.flatMap((b, i) => (b.heading ? [{ block: i, ...b.heading }] : []))
  const realLevels = raw.filter(e => !e.pseudo).map(e => e.level)
  const top = Math.min(...realLevels)
  let parent = 0 // normalised level of the last real heading
  return raw.map((e, n) => {
    let level: number
    if (e.pseudo) {
      level = parent + 1
    } else {
      level = e.level - top + 1
      parent = level
    }
    return {
      block: e.block,
      text: e.text,
      level: Math.min(4, level),
      kind: classifySection(e.text, blocks.slice(e.block + 1, raw[n + 1]?.block ?? blocks.length)),
    }
  })
}

// activeEntry is the entry being read: the last heading whose top has
// scrolled up to the reading line. -1 means none yet (still on the
// question or the text before the first heading).
export function activeEntry(tops: readonly number[], readingLine: number): number {
  let active = -1
  tops.forEach((top, i) => {
    if (top <= readingLine) active = i
  })
  return active
}

// The side panel's width, one for both its tabs (On this page and
// Artifacts), so switching tabs never moves the answer column. Wide enough
// at its narrowest for the tab strip; the artifact viewer gets room by
// dragging the panel wider.
export const OUTLINE_MIN = 240
export const OUTLINE_MAX = 960
export const OUTLINE_DEFAULT = 360

export function clampWidth(w: number): number {
  return Math.round(Math.min(OUTLINE_MAX, Math.max(OUTLINE_MIN, w)))
}

// Panel layout is a per-viewer convenience, so it lives in localStorage;
// if storage is unavailable the defaults apply.
const KEY = 'uncli-outline'

export interface OutlineLayout {
  open: boolean
  width: number
}

export function loadLayout(): OutlineLayout {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null') as Partial<OutlineLayout> | null
    if (v) return { open: v.open !== false, width: clampWidth(v.width ?? OUTLINE_DEFAULT) }
  } catch {
    // fall through to defaults
  }
  return { open: true, width: OUTLINE_DEFAULT }
}

export function saveLayout(l: OutlineLayout) {
  try {
    localStorage.setItem(KEY, JSON.stringify(l))
  } catch {
    // not persisted; still applied for this run
  }
}

// What the quick-task model is shown of each block when asked for a
// summary: markdown blocks as written, code blocks only described (their
// content costs tokens and rarely names a section).
export function summaryBlocks(blocks: readonly MdBlock[]): string[] {
  return blocks.map(b => {
    if (b.kind !== 'code') return b.source
    const lines = (b.code ?? '').split('\n').length
    return `(code block${b.lang ? `: ${b.lang}` : ''}, ${lines} line${lines === 1 ? '' : 's'})`
  })
}

// summaryEntries turns a stored summary into outline entries, dropping any
// section that no longer points at a block. The model's kind wins; a
// summary made before kinds existed gets the local guess.
export function summaryEntries(sections: readonly { block: number; title: string; kind?: string }[], blocks: readonly MdBlock[]): OutlineEntry[] {
  const kept = sections.filter(s => s.block >= 0 && s.block < blocks.length)
  return kept.map((s, n) => ({
    block: s.block,
    level: 1,
    text: s.title,
    kind: isKind(s.kind) ? s.kind : classifySection(s.title, blocks.slice(s.block, kept[n + 1]?.block ?? blocks.length)),
  }))
}

// Automatic summaries (a preference): none, long answers only, or every
// answer. Even "always" skips a one-block answer, which has nothing to
// outline. The summary is the same one the button makes.
export const AUTO_SUMMARY_MIN_WORDS = 150

// autoSummaryFor says what to do with a finished answer: summarise it, or
// leave it because it's too short (the outline says so), or nothing at all
// when automatic summaries are off.
export function autoSummaryFor(markdown: string, blockCount: number, mode: AutoSummary): 'summarise' | 'short' | 'off' {
  if (mode !== 'long' && mode !== 'always') return 'off'
  if (blockCount < 2) return 'short'
  if (mode === 'long' && wordCount(markdown) < AUTO_SUMMARY_MIN_WORDS) return 'short'
  return 'summarise'
}

export function wordCount(markdown: string): number {
  return markdown.match(/[\p{L}\p{N}]+/gu)?.length ?? 0
}
