import { beforeEach, describe, expect, it } from 'vitest'
import { choices, forget, langOf, loadWatched, pick, RECENT_MAX, saveWatched, toggleStar, watchPathOf } from './watched'

const entries = [
  { path: 'plan/todo.txt', size: 1, modified: 300 },
  { path: 'docs/notes.md', size: 1, modified: 200 },
  { path: 'TODO.md', size: 1, modified: 100 },
]

describe('watched files', () => {
  beforeEach(() => localStorage.clear())

  it('keeps each session’s choices, and recent ones newest first', () => {
    let w = loadWatched('s1')
    expect(w).toEqual({ current: '', recent: [], starred: [] })
    w = pick(pick(pick(w, 'a.md'), 'b.md'), 'a.md')
    expect(w.current).toBe('a.md')
    expect(w.recent).toEqual(['a.md', 'b.md'])
    w = toggleStar(w, 'b.md')
    saveWatched('s1', w)
    expect(loadWatched('s1')).toEqual(w)
    expect(loadWatched('s2').current).toBe('')
    expect(toggleStar(w, 'b.md').starred).toEqual([])
    expect(forget(w, 'a.md')).toEqual({ current: '', recent: ['b.md'], starred: ['b.md'] })

    for (let i = 0; i < 20; i++) w = pick(w, `f${i}`)
    expect(w.recent).toHaveLength(RECENT_MAX)
  })

  it('ignores storage that isn’t its own shape', () => {
    localStorage.setItem('uncli-watch:s1', '{"current":3,"recent":["x",4,""],"starred":"y"}')
    expect(loadWatched('s1')).toEqual({ current: '', recent: ['x'], starred: [] })
    localStorage.setItem('uncli-watch:s1', 'not json')
    expect(loadWatched('s1').recent).toEqual([])
  })

  it('lists starred, then recent, then the folder; a search newest first', () => {
    const w = { current: 'TODO.md', recent: ['TODO.md', 'gone.md'], starred: ['docs/notes.md'] }
    expect(choices('', entries, w).map(c => [c.path, c.mark])).toEqual([
      ['docs/notes.md', 'starred'], ['TODO.md', 'recent'], ['gone.md', 'recent'], ['plan/todo.txt', ''],
    ])
    // A search keeps the folder's order (newest first), even over a starred or recent file.
    const todo = entries.filter(e => e.path.toLowerCase().includes('todo'))
    expect(choices('todo', todo, w).map(c => c.path)).toEqual(['plan/todo.txt', 'TODO.md'])
  })

  it('takes a changed file’s path inside the session folder only', () => {
    expect(watchPathOf('C:\\work\\app', 'C:\\Work\\app\\docs\\TODO.md')).toBe('docs/TODO.md')
    expect(watchPathOf('/home/me/app', '/home/me/app/TODO.md')).toBe('TODO.md')
    expect(watchPathOf('/home/me/app', '/etc/hosts')).toBeNull()
    expect(watchPathOf('/home/me/app', 'notes.md')).toBe('notes.md')
  })

  it('highlights by extension', () => {
    expect(langOf('src/a.ts')).toBe('typescript')
    expect(langOf('build.log')).toBe('text')
  })
})

describe('pages', () => {
  it('knows which files show as a page', async () => {
    const { pageKind } = await import('./watched')
    expect(pageKind('docs/report.HTML')).toBe('html')
    expect(pageKind('index.htm')).toBe('html')
    expect(pageKind('logo.svg')).toBe('svg')
    expect(pageKind('notes.md')).toBe('')
    expect(pageKind('page.html.bak')).toBe('')
  })
})
