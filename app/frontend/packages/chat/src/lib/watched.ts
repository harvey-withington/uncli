// The File tab's choices for each session, kept in localStorage: the file
// it shows, the ones shown lately, and the ones the viewer starred. Storage
// that fails only means they aren't kept.
import type { WatchEntry } from './api'
import { artifactLang } from './artifacts'
import { relPath } from './files'

const PREFIX = 'uncli-watch:'
export const RECENT_MAX = 8

export interface Watched {
  current: string // the file shown ('' for none), relative to the session folder
  recent: string[] // newest first, the current one included
  starred: string[]
}

const empty = (): Watched => ({ current: '', recent: [], starred: [] })

const strings = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string' && x !== '') : [])

export function loadWatched(sessionId: string): Watched {
  try {
    const v = JSON.parse(localStorage.getItem(PREFIX + sessionId) ?? 'null')
    if (v && typeof v === 'object') {
      return { current: typeof v.current === 'string' ? v.current : '', recent: strings(v.recent).slice(0, RECENT_MAX), starred: strings(v.starred) }
    }
  } catch {
    // fall through
  }
  return empty()
}

export function saveWatched(sessionId: string, w: Watched) {
  try {
    if (!w.current && !w.recent.length && !w.starred.length) localStorage.removeItem(PREFIX + sessionId)
    else localStorage.setItem(PREFIX + sessionId, JSON.stringify(w))
  } catch {
    // not kept; still there for this run
  }
}

// pick shows a file, moving it to the front of the recent ones.
export function pick(w: Watched, path: string): Watched {
  return { ...w, current: path, recent: [path, ...w.recent.filter(p => p !== path)].slice(0, RECENT_MAX) }
}

// forget takes a file out of the recent ones (and stops showing it).
export function forget(w: Watched, path: string): Watched {
  return { ...w, current: w.current === path ? '' : w.current, recent: w.recent.filter(p => p !== path) }
}

export function toggleStar(w: Watched, path: string): Watched {
  return { ...w, starred: w.starred.includes(path) ? w.starred.filter(p => p !== path) : [...w.starred, path] }
}

export type Mark = 'starred' | 'recent' | ''

export interface Choice {
  path: string
  modified?: number // when it last changed, if the folder listing has it
  mark: Mark
}

// choices is the switcher's list. With nothing typed: starred files, then
// recent ones, then the rest of the folder, newest first. With a search:
// every match newest first (as listed), so "todo" puts the latest TODO file
// at the top; starred and recent ones are only marked.
export function choices(query: string, entries: WatchEntry[], w: Watched): Choice[] {
  const when = new Map(entries.map(e => [e.path, e.modified]))
  const mark = (p: string): Mark => (w.starred.includes(p) ? 'starred' : w.recent.includes(p) ? 'recent' : '')
  if (query.trim()) return entries.map(e => ({ path: e.path, modified: e.modified, mark: mark(e.path) }))
  const seen = new Set<string>()
  const out: Choice[] = []
  const add = (path: string) => {
    if (seen.has(path)) return
    seen.add(path)
    out.push({ path, modified: when.get(path), mark: mark(path) })
  }
  w.starred.forEach(add)
  w.recent.forEach(add)
  entries.forEach(e => add(e.path))
  return out
}

// pageKind: shown as a page (in a sandboxed frame) rather than as code,
// unless the viewer asks for the source; '' for anything else.
export function pageKind(path: string): 'html' | 'svg' | '' {
  const m = /\.(html?|svg)$/i.exec(path)
  return !m ? '' : m[1]?.toLowerCase() === 'svg' ? 'svg' : 'html'
}

// isMarkdown: shown rendered rather than as code.
export function isMarkdown(path: string): boolean {
  return /\.(md|markdown|mdx)$/i.test(path)
}

// langOf is the highlighter's language for a file ('text' for plain).
export function langOf(path: string): string {
  return artifactLang(path)
}

// watchPathOf is a changed file's path for the File tab (relative to the
// session folder, with slashes), or null when it's outside the folder.
export function watchPathOf(workdir: string, path: string): string | null {
  const rel = relPath(workdir, path)
  if (rel === path && /^([a-z]:)?[\\/]/i.test(path)) return null
  return rel.split('\\').join('/')
}
