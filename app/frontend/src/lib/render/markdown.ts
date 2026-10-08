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

// Links to files (file:///…, as the Antigravity CLI writes them) are links
// too: markdown-it refuses them by default, which left their markdown
// showing. Scripts and inline data never are.
const unsafeLink = /^\s*(javascript|vbscript|data):/i
md.validateLink = url => !unsafeLink.test(url)

// Links never navigate the app window; the click handler opens web links
// in the user's browser and files in their editor (lib/links.ts).
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
  // For the outline: a real heading (level 1–6), or a paragraph acting as
  // one ("**1. Point.** More text…"), marked pseudo; its level is set by
  // the outline relative to the real headings around it.
  heading?: { level: number; text: string; pseudo?: boolean }
}

// The answer, its outline and the margin markers all parse the same text,
// often on every streamed delta; the last few results are kept.
const recent = new Map<string, MdBlock[]>()

export function toBlocks(src: string): MdBlock[] {
  const hit = recent.get(src)
  if (hit) return hit
  const out = parseBlocks(src)
  recent.set(src, out)
  if (recent.size > 8) recent.delete(recent.keys().next().value as string)
  return out
}

function parseBlocks(src: string): MdBlock[] {
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
      const block: MdBlock = { key, kind: 'md', html: md.renderer.render(group, md.options, {}), source }
      if (first.type === 'heading_open') {
        const text = plainText(group[1] as Token | undefined)
        if (text) block.heading = { level: Number(first.tag.slice(1)) || 1, text }
      } else if (first.type === 'paragraph_open') {
        const text = boldLead(group[1] as Token | undefined)
        if (text) block.heading = { level: 0, text, pseudo: true }
      }
      out.push(block)
    }
  }
  return out
}

// plainText is an inline token's text without its markup.
function plainText(inline: Token | undefined): string {
  if (!inline) return ''
  const parts = (inline.children ?? []).filter(c => c.type === 'text' || c.type === 'code_inline').map(c => c.content)
  return (parts.length ? parts.join('') : inline.content).replace(/\s+/g, ' ').trim()
}

const MAX_LEAD = 120

// boldLead returns the title of a paragraph that works as a heading: one
// that is entirely bold and short, or that opens with a short bold lead
// which is numbered ("1.", "2)") or ends like a label (":" or "."). Answers
// often structure points this way instead of using # headings.
function boldLead(inline: Token | undefined): string {
  // markdown-it starts the children with an empty text token; skip it.
  const all = inline?.children ?? []
  const kids = all.slice(all.findIndex(k => !(k.type === 'text' && k.content === '')))
  if (kids[0]?.type !== 'strong_open') return ''
  const close = kids.findIndex(k => k.type === 'strong_close')
  if (close < 0) return ''
  const lead = kids.slice(1, close).filter(k => k.type === 'text' || k.type === 'code_inline').map(k => k.content).join('').replace(/\s+/g, ' ').trim()
  if (!lead || lead.length > MAX_LEAD) return ''
  const rest = kids.slice(close + 1).filter(k => k.type === 'text' || k.type === 'code_inline').map(k => k.content).join('').trim()
  const wholeParagraph = rest === '' || /^[:.—-]$/.test(rest)
  const labelled = /^\d+[.)]\s/.test(lead) || /[:.]$/.test(lead)
  if (!wholeParagraph && !labelled) return ''
  return lead.replace(/[\s:.]+$/, '')
}

// Escape text for HTML (plain code before the highlighter loads).
export function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
