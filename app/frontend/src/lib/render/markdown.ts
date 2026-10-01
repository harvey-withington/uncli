// Answers render as a list of top-level blocks, each carrying the exact
// markdown source it came from (via markdown-it's line maps), so every block
// can copy its original markdown and every code block its code.
import MarkdownIt from 'markdown-it'
import type Token from 'markdown-it/lib/token.mjs'

const md = new MarkdownIt({
  html: false, // model output is untrusted: no raw HTML
  linkify: true,
  typographer: false,
  breaks: false,
})

// Links never navigate the app window; the click handler opens them in the
// user's browser.
const defaultLink = md.renderer.rules.link_open ?? ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))
md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  tokens[idx]?.attrSet('rel', 'noopener noreferrer')
  tokens[idx]?.attrSet('data-external', '')
  return defaultLink(tokens, idx, options, env, self)
}

export interface MdBlock {
  key: string
  kind: 'code' | 'md'
  html: string // for md blocks
  source: string // original markdown of the block
  code?: string // for code blocks: the code only
  lang?: string
}

export function toBlocks(src: string): MdBlock[] {
  const tokens = md.parse(src, {})
  const lines = src.split('\n')
  const out: MdBlock[] = []
  let i = 0
  while (i < tokens.length) {
    const start = i
    const t = tokens[i] as Token
    if (t.nesting === 1) {
      let depth = 0
      for (; i < tokens.length; i++) {
        depth += (tokens[i] as Token).nesting
        if (depth === 0) break
      }
    }
    i++
    const group = tokens.slice(start, i)
    const first = group[0] as Token
    const map = first.map ?? group.find(g => g.map)?.map
    const source = map ? lines.slice(map[0], map[1]).join('\n').replace(/\s+$/, '') : first.content
    const key = `${out.length}:${first.type}`
    if (first.type === 'fence' || first.type === 'code_block') {
      out.push({
        key, kind: 'code', html: '', source,
        code: first.content.replace(/\n$/, ''),
        lang: (first.info || '').trim().split(/\s+/)[0] || '',
      })
    } else {
      out.push({ key, kind: 'md', html: md.renderer.render(group, md.options, {}), source })
    }
  }
  return out
}

// Escape text for HTML (plain code before the highlighter loads).
export function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
