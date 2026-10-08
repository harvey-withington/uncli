import { describe, expect, it } from 'vitest'
import { linkTarget } from './links'

const work = 'C:\\Users\\you\\code'

describe('answer links', () => {
  it('opens web links in the browser', () => {
    expect(linkTarget('https://example.com/a', work)).toEqual({ kind: 'web', url: 'https://example.com/a' })
  })

  it('opens file links as the Antigravity CLI writes them', () => {
    expect(linkTarget('file:///S:/Local/Code/Projects/dummy-project/dummy-project-1.0/app/README.md', work))
      .toEqual({ kind: 'file', path: 'S:/Local/Code/Projects/dummy-project/dummy-project-1.0/app/README.md', line: 0 })
    expect(linkTarget('file:///C:/My%20Project/main.go#L12-L20', work)).toEqual({ kind: 'file', path: 'C:/My Project/main.go', line: 12 })
    expect(linkTarget('file:///home/you/x.ts', work)).toEqual({ kind: 'file', path: '/home/you/x.ts', line: 0 })
  })

  it('opens paths relative to the session folder, with a line', () => {
    expect(linkTarget('app/main.go:42', work)).toEqual({ kind: 'file', path: 'C:\\Users\\you\\code\\app\\main.go', line: 42 })
    expect(linkTarget('./docs/../README.md', work)).toEqual({ kind: 'file', path: 'C:\\Users\\you\\code\\README.md', line: 0 })
    expect(linkTarget('C:\\x\\y.md', work)).toEqual({ kind: 'file', path: 'C:\\x\\y.md', line: 0 })
  })

  it('follows nothing else', () => {
    for (const h of ['javascript:alert(1)', 'mailto:a@b.c', 'vscode://file/x', '#top', '']) expect(linkTarget(h, work)).toBeNull()
    expect(linkTarget('app/main.go', '')).toBeNull()
  })
})
