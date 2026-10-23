// Answers render as a list of top-level blocks, each carrying the exact
// markdown source it came from (via markdown-it's line maps), so every block
// can copy its original markdown and every code block its code.
//
// Model output (and a watched file) is untrusted, and the app window holds
// the bridge to the Go side, so raw HTML renders only through DOMPurify:
// no scripts, event handlers, styles, forms, frames or ids, and links still
// go through the click handler (lib/links.ts). Task lists ("- [ ] thing")
// show as checkboxes, read only.
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import type Token from 'markdown-it/lib/token.mjs'

const md = new MarkdownIt({
  html: true, // cleaned by DOMPurify below
  linkify: true,
  typographer: false,
  breaks: false,
})

// Links to files (file:///…, as the Antigravity CLI writes them) are links
// too: markdown-it refuses them by default, which left their markdown
// showing. Scripts and inline data never are.
const unsafeLink = /^\s*(javascript|vbscript|data):/i
md.validateLink = url => !unsafeLink.test(url)

// A list item that starts "[ ] " or "[x] " is a task: its marker becomes a
// checkbox (GitHub's task lists).
const TASK = /^\[([ xX])\][ \t]/
md.core.ruler.after('inline', 'tasks', state => {
  const tokens = state.tokens
  for (let i = 2; i < tokens.length; i++) {
    const inline = tokens[i]
    if (!inline || inline.type !== 'inline' || tokens[i - 1]?.type !== 'paragraph_open' || tokens[i - 2]?.type !== 'list_item_open') continue
    const first = inline.children?.[0]
    const m = first?.type === 'text' ? TASK.exec(first.content) : null
    if (!first || !m) continue
    first.content = first.content.slice(m[0].length)
    const box = new state.Token('html_inline', '', 0)
    box.content = `<input type="checkbox" class="task" disabled${m[1] === ' ' ? '' : ' checked'}> `
    inline.children?.unshift(box)
    tokens[i - 2]?.attrSet('class', 'task')
  }
})

// What raw HTML may keep: the HTML profile (no SVG or MathML), less what
// could run, submit, restyle or cover the app, or clash with its ids and
// classes. URLs may also be file: links, as markdown ones may.
const PURIFY = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ['style', 'form', 'button', 'textarea', 'select', 'option', 'iframe', 'frame', 'object', 'embed', 'dialog', 'template', 'link', 'meta', 'base'],
  FORBID_ATTR: ['style', 'id', 'name', 'target', 'form', 'formaction', 'srcset', 'autofocus', 'tabindex', 'contenteditable', 'popover'],
  ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|tel|file):|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i,
}
const KEEP_CLASS = new Set(['task'])

let purify: ReturnType<typeof DOMPurify> | null = null

function clean(html: string): string {
  if (!purify) {
    if (typeof window === 'undefined') return md.utils.escapeHtml(html)
    purify = DOMPurify(window)
    purify.addHook('uponSanitizeAttribute', (_node, data) => {
      if (data.attrName !== 'class') return
      data.attrValue = data.attrValue.split(/\s+/).filter(c => KEEP_CLASS.has(c)).join(' ')
      if (!data.attrValue) data.keepAttr = false
    })
    purify.addHook('afterSanitizeAttributes', node => {
      // Only read-only checkboxes are inputs. Links never navigate the app
      // window: the click handler opens web links in the user's browser and
      // files in their editor (lib/links.ts).
      if (node.tagName === 'INPUT') {
        if (node.getAttribute('type') !== 'checkbox') node.remove()
        else node.setAttribute('disabled', '')
      }
      if (node.tagName === 'A') {
        node.setAttribute('rel', 'noopener noreferrer')
        node.setAttribute('data-external', '')
      }
    })
  }
  return purify.sanitize(html, PURIFY)
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
  // The end (exclusive) of the top-level block starting at i.
  const end = (i: number) => {
    if ((tokens[i] as Token).nesting === 1) {
      let depth = 0
      for (; i < tokens.length; i++) {
        depth += (tokens[i] as Token).nesting
        if (depth === 0) break
      }
    }
    return i + 1
  }
  let i = 0
  while (i < tokens.length) {
    const start = i
    i = end(i)
    // HTML that wraps markdown (<details> … blank line … </details>) spans
    // several blocks: they stay one, up to the tag that closes it.
    let open = tokens[start]?.type === 'html_block' ? unclosed(tokens[start]?.content ?? '') : 0
    while (open > 0 && i < tokens.length) {
      if (tokens[i]?.type === 'html_block') open += unclosed(tokens[i]?.content ?? '')
      i = end(i)
    }
    const group = tokens.slice(start, i)
    const first = group[0] as Token
    const last = [...group].reverse().find(g => g.map && g.level === 0)
    const map = first.map && last?.map ? [first.map[0], last.map[1]] : (first.map ?? group.find(g => g.map)?.map)
    const source = map ? lines.slice(map[0], map[1]).join('\n').replace(/\s+$/, '') : first.content
    const key = `${out.length}:${first.type}`
    if (first.type === 'fence' || first.type === 'code_block') {
      out.push({
        key, kind: 'code', html: '', source,
        code: first.content.replace(/\n$/, ''),
        lang: (first.info || '').trim().split(/\s+/)[0] || '',
      })
    } else {
      const block: MdBlock = { key, kind: 'md', html: clean(md.renderer.render(group, md.options, {})), source }
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

// unclosed is how many more container tags an HTML block opens than it
// closes (void and self-closed tags don't count).
const VOID = new Set(['area', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'source', 'track', 'wbr'])
function unclosed(html: string): number {
  let n = 0
  for (const m of html.matchAll(/<(\/?)([a-z][a-z0-9-]*)\b[^>]*?(\/?)>/gi)) {
    if (VOID.has((m[2] ?? '').toLowerCase()) || m[3]) continue
    n += m[1] ? -1 : 1
  }
  return n
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
