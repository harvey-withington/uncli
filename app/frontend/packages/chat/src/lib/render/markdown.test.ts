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

  it('renders html for markdown blocks, and raw html only cleaned', () => {
    expect(blocks[0]?.html).toContain('<strong>bold</strong>')
    expect(blocks[3]?.html).toContain('<table>')
    expect(blocks[5]?.html).not.toContain('script')
  })

  it('keeps harmless html and drops what could run, restyle or submit', () => {
    const html = toBlocks([
      '<details><summary>More</summary><p>Press <kbd>Ctrl</kbd>+<kbd>K</kbd> <sub>1</sub></p></details>',
      '',
      '<img src="https://uncli.app/a.png" alt="A" onerror="alert(1)" style="position:fixed">',
      '',
      '<a href="javascript:alert(1)">bad</a> <a href="https://uncli.app" target="_top">good</a>',
      '',
      '<div id="side-panel" class="btn primary"><iframe src="https://x"></iframe><button>Go</button><input type="text"><style>*{display:none}</style><svg><script>x</script></svg>text</div>',
    ].join('\n')).map(b => b.html).join('')
    expect(html).toContain('<details><summary>More</summary>')
    expect(html).toContain('<kbd>Ctrl</kbd>')
    expect(html).toContain('<sub>1</sub>')
    expect(html).toContain('<img src="https://uncli.app/a.png" alt="A">')
    expect(html).toContain('<a rel="noopener noreferrer" data-external="">bad</a>')
    expect(html).toContain('<a href="https://uncli.app" rel="noopener noreferrer" data-external="">good</a>')
    for (const gone of ['onerror', 'style', 'javascript', 'target', 'id=', 'class=', 'btn', '<iframe', '<button', '<input', '<svg', '<script', 'display:none']) {
      expect(html).not.toContain(gone)
    }
    expect(html).toContain('text</div>')
  })

  it('keeps markdown wrapped in html in one block', () => {
    const src = '<details><summary>More</summary>\n\n- one\n- **two**\n\n</details>'
    const blocks = toBlocks(`${src}\n\nAfter`)
    expect(blocks).toHaveLength(2)
    expect(blocks[0]?.html).toMatch(/^<details><summary>More<\/summary>\s*<ul>[\s\S]*<strong>two<\/strong>[\s\S]*<\/ul>\s*<\/details>\s*$/)
    expect(blocks[0]?.source).toBe(src)
    expect(blocks[1]?.html).toContain('<p>After</p>')
    // Never closed (still streaming, say): the rest stays inside.
    expect(toBlocks('<div>\n\nText\n\nMore')).toHaveLength(1)
  })

  it('shows task lists as read-only checkboxes', () => {
    const [list] = toBlocks('- [x] Done\n- [ ] To do\n- [] not a task\n- plain')
    expect(list?.html).toContain('<li class="task"><input type="checkbox" class="task" disabled="" checked=""> Done</li>')
    expect(list?.html).toContain('<li class="task"><input type="checkbox" class="task" disabled=""> To do</li>')
    expect(list?.html).toContain('<li>[] not a task</li>')
    expect(list?.html).toContain('<li>plain</li>')
    const [code] = toBlocks('`- [ ] in code`')
    expect(code?.html).not.toContain('checkbox')
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
