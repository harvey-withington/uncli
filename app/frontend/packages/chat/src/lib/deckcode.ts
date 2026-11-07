// A walkthrough slide's code (decision 0029): coloured by Shiki where it
// knows the language, plain otherwise, each line marked added, removed or
// unchanged and carrying its line number, for DeckViewer to style.
import type { DeckSlide } from './deck'
import { highlight } from './render/highlight'
import { langOf } from './watched'

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

export async function codeHTML(s: DeckSlide): Promise<string> {
  const code = s.lines.map(l => l.text).join('\n')
  const html = await highlight(code, langOf(s.path)).catch(() => null)
  const doc = new DOMParser().parseFromString(
    html ?? `<pre class="shiki"><code>${s.lines.map(l => `<span class="line">${esc(l.text)}</span>`).join('\n')}</code></pre>`,
    'text/html',
  )
  const pre = doc.querySelector('pre')
  if (!pre) return ''
  pre.removeAttribute('style') // the viewer's background, not the theme's
  pre.removeAttribute('tabindex')
  pre.querySelectorAll('.line').forEach((el, i) => {
    const l = s.lines[i]
    if (!l) return
    el.classList.add(l.kind)
    el.setAttribute('data-n', String(l.n))
  })
  return pre.outerHTML
}
