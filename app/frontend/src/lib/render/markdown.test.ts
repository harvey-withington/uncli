import { describe, expect, it } from 'vitest'
import { toBlocks } from './markdown'

const SRC = `Intro with **bold**.

- one
- two
  - nested

\`\`\`go
fmt.Println("hi")
\`\`\`

| a | b |
| --- | --- |
| 1 | 2 |

> quote

<script>alert(1)</script>`

describe('toBlocks', () => {
  const blocks = toBlocks(SRC)

  it('splits top-level blocks with their source markdown', () => {
    expect(blocks.map(b => b.kind)).toEqual(['md', 'md', 'code', 'md', 'md', 'md'])
    expect(blocks[0]?.source).toBe('Intro with **bold**.')
    expect(blocks[1]?.source).toBe('- one\n- two\n  - nested')
    expect(blocks[3]?.source).toBe('| a | b |\n| --- | --- |\n| 1 | 2 |')
    expect(blocks[4]?.source).toBe('> quote')
  })

  it('gives code blocks their code and language', () => {
    expect(blocks[2]?.code).toBe('fmt.Println("hi")')
    expect(blocks[2]?.lang).toBe('go')
    expect(blocks[2]?.source).toBe('```go\nfmt.Println("hi")\n```')
  })

  it('renders html for markdown blocks and never raw html', () => {
    expect(blocks[0]?.html).toContain('<strong>bold</strong>')
    expect(blocks[3]?.html).toContain('<table>')
    expect(blocks[5]?.html).not.toContain('<script>')
    expect(blocks[5]?.html).toContain('&lt;script&gt;')
  })

  it('marks links as external', () => {
    const [b] = toBlocks('See https://uncli.app now')
    expect(b?.html).toContain('data-external')
    expect(b?.html).toContain('href="https://uncli.app"')
  })

  // As the Antigravity CLI writes them: code as the text, a file URL.
  it('renders links to files, but never to scripts', () => {
    const [list] = toBlocks('* [`app/`](file:///S:/Local/Code/dummy/app) — the app, with [`README.md`](file:///S:/Local/Code/dummy/app/README.md).')
    expect(list?.html).toContain('<a href="file:///S:/Local/Code/dummy/app" rel="noopener noreferrer" data-external=""><code>app/</code></a>')
    expect(list?.html).toContain('href="file:///S:/Local/Code/dummy/app/README.md"')
    expect(list?.html).not.toContain('](file:')
    for (const bad of ['javascript:alert(1)', 'JAVASCRIPT:x', 'data:text/html,x', 'vbscript:x']) {
      expect(toBlocks(`[x](${bad})`)[0]?.html).not.toContain('<a')
    }
  })

  it('copes with a half-streamed fence', () => {
    const partial = toBlocks('Text\n\n```ts\nconst a = 1')
    expect(partial.at(-1)?.kind).toBe('code')
    expect(partial.at(-1)?.code).toBe('const a = 1')
  })

  it('gives empty input no blocks', () => {
    expect(toBlocks('')).toEqual([])
  })
})
