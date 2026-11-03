import { describe, expect, it } from 'vitest'
import type { Page } from './api'
import { artifactKind, artifactPathOf, artifactsAt, decodeText, sandboxDoc } from './artifacts'

const page = (seq: number, artifacts: Page['artifacts']): Page => ({
  id: `p${seq}`, sessionId: 's', seq, question: '', model: '', modifiers: [], answerMd: '', trace: [], touchedFiles: [],
  status: 'done', bookmarked: false, pinned: false, inputTokens: 0, outputTokens: 0, cacheRead: 0, cacheWrite: 0,
  costUsd: 0, durationMs: 0, startedAt: 0, finishedAt: 0, artifacts,
})

describe('artifactsAt', () => {
  const pages = [
    page(1, [{ path: 'report.html', hash: 'h1', size: 10 }]),
    page(2, []),
    page(3, [{ path: 'report.html', hash: 'h3', size: 12 }, { path: 'chart.svg', hash: 'c3', size: 5 }]),
    page(4, [{ path: 'chart.svg', size: 0, deleted: true }]),
  ]
  it('shows each artifact as of the page, the ones it changed marked', () => {
    expect(artifactsAt(pages, 0, null)).toEqual([{ path: 'report.html', hash: 'h1', size: 10, live: false, changed: true, seq: 1 }])
    expect(artifactsAt(pages, 1, null)).toEqual([{ path: 'report.html', hash: 'h1', size: 10, live: false, changed: false, seq: 1 }])
    const at3 = artifactsAt(pages, 2, null)
    expect(at3.map(a => [a.path, a.hash, a.changed])).toEqual([['chart.svg', 'c3', true], ['report.html', 'h3', true]])
    expect(artifactsAt(pages, 3, null).map(a => a.path)).toEqual(['report.html']) // deleted on page 4
  })
  it('adds files no turn recorded, on the latest page only', () => {
    const live = [{ path: 'notes.md', size: 3 }, { path: 'report.html', size: 99 }]
    const latest = artifactsAt(pages, 3, live)
    expect(latest.map(a => [a.path, a.live])).toEqual([['notes.md', true], ['report.html', false]])
    expect(artifactsAt(pages, 1, live).map(a => a.path)).toEqual(['report.html'])
  })
  it('works with no pages', () => {
    expect(artifactsAt([], 0, [{ path: 'a.md', size: 1 }])).toEqual([{ path: 'a.md', size: 1, live: true, changed: false, seq: 0 }])
  })
})

describe('artifactKind', () => {
  it('picks the viewer by extension', () => {
    expect(artifactKind('a/Report.HTML')).toBe('html')
    expect(artifactKind('flow.mmd')).toBe('mermaid')
    expect(artifactKind('notes.md')).toBe('markdown')
    expect(artifactKind('x.png')).toBe('image')
    expect(artifactKind('data.json')).toBe('text')
    expect(artifactKind('deck.pptx')).toBe('other')
    expect(artifactKind('Makefile')).toBe('other')
  })
})

describe('sandboxDoc', () => {
  it('puts a no-network policy first and removes what would redirect the frame', () => {
    const doc = sandboxDoc('<html><head><meta http-equiv="refresh" content="0;url=https://evil.example"><base href="https://evil.example/"><title>x</title></head><body><script>1</script></body></html>')
    expect(doc.startsWith('<!DOCTYPE html>')).toBe(true)
    expect(doc).toContain("default-src 'none'")
    expect(doc).toContain("script-src 'unsafe-inline'")
    expect(doc).not.toContain('evil.example')
    expect(doc.indexOf('Content-Security-Policy')).toBeLessThan(doc.indexOf('<title>'))
    expect(sandboxDoc('<p>x</p>', false)).toContain("script-src 'none'")
  })
})

describe('artifactPathOf', () => {
  it('finds files in the artifacts folder', () => {
    expect(artifactPathOf('C:\\s\\chat1', 'C:\\s\\chat1\\artifacts\\charts\\a.svg')).toBe('charts/a.svg')
    expect(artifactPathOf('C:\\s\\chat1', 'c:/S/chat1/Artifacts/a.svg')).toBe('a.svg')
    expect(artifactPathOf('/home/me/w', '/home/me/w/artifacts/x.html')).toBe('x.html')
    expect(artifactPathOf('/home/me/w', '/home/me/w/notes.md')).toBeNull()
  })
})

describe('decodeText', () => {
  it('decodes UTF-8', () => {
    expect(decodeText(btoa(String.fromCharCode(...new TextEncoder().encode('café ✓'))))).toBe('café ✓')
  })
})
