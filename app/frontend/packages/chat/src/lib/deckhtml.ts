// The walkthrough's page (decision 0029): a whole HTML document for a
// sandboxed iframe's srcdoc. Its scripts (reveal.js and the deck's own)
// are one file loaded by URL from the app's origin, not inline: the frame
// inherits the page's content security policy, which may refuse inline
// scripts (the phone app's does). Loaded only when a deck first opens.
import revealCss from 'reveal.js/reveal.css?raw'
import playerUrl from 'virtual:uncli-deck-player'
import type { Deck, DeckSlide } from './deck'
import { langOf } from './watched'
import { highlight } from './render/highlight'

export interface DeckText {
  title: string // the page's question
  summary: string // "5 changes in 2 files"
  file: (s: DeckSlide) => string // "parser.go · lines 142–151 · 2 of 2"
  others: string // the heading over files without a record
  keys: string // how to move about
}

// Messages the deck sends its dialog.
export const DECK_MESSAGE = 'uncli-deck'
export type DeckMessage = { [DECK_MESSAGE]: 'escape' | 'fullscreen' }

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

// The slide's code, coloured by Shiki where it knows the language, each
// line numbered and marked.
async function codeOf(s: DeckSlide): Promise<string> {
  const code = s.lines.map(l => l.text).join('\n')
  const html = await highlight(code, langOf(s.path)).catch(() => null)
  const doc = new DOMParser().parseFromString(html ?? `<pre class="shiki"><code>${s.lines.map(l => `<span class="line">${esc(l.text)}</span>`).join('\n')}</code></pre>`, 'text/html')
  const pre = doc.querySelector('pre')
  if (!pre) return ''
  pre.removeAttribute('style') // the deck's own background, not the theme's
  pre.querySelectorAll('.line').forEach((el, i) => {
    const l = s.lines[i]
    if (!l) return
    el.classList.add(l.kind)
    el.setAttribute('data-n', String(l.n))
  })
  return pre.outerHTML
}

const CSS = `
:root { color-scheme: var(--scheme); }
html, body, .reveal-viewport { margin: 0; height: 100%; background: light-dark(#ffffff, #16171b); }
:root { --r-background-color: light-dark(#ffffff, #16171b); }
.reveal { font-family: system-ui, -apple-system, 'Segoe UI', sans-serif; font-size: 26px; color: light-dark(#1d1f24, #e6e7ea); }
.reveal .slides { text-align: left; }
.reveal .slides section { height: 100%; box-sizing: border-box; }
.reveal .slide { height: 100%; display: flex; flex-direction: column; gap: 14px; }
.reveal h2 { margin: 0; font-size: 1.5em; font-weight: 650; line-height: 1.25; }
.reveal .summary { color: light-dark(#5b606b, #a0a4ad); margin: 0; }
.reveal ul { margin: 0; padding-left: 1.2em; font-family: ui-monospace, 'Cascadia Code', Consolas, monospace; font-size: .7em; line-height: 1.6; }
.reveal .keys { margin-top: auto; color: light-dark(#8a8f99, #7b808a); font-size: .6em; }
.reveal .file { font-family: ui-monospace, 'Cascadia Code', Consolas, monospace; font-size: .7em; color: light-dark(#5b606b, #a0a4ad); display: flex; flex-wrap: wrap; gap: .3em 1em; align-items: baseline; }
.reveal .file b { color: light-dark(#1d1f24, #e6e7ea); font-weight: 600; }
.reveal .file .add { color: light-dark(#1a7f37, #3fb950); }
.reveal .file .del { color: light-dark(#cf222e, #f85149); }
.reveal pre.shiki { margin: 0; padding: 12px 0; flex: 1 1 auto; min-height: 0; overflow: auto; border-radius: 10px;
  background: light-dark(#f6f7f9, #1e2026); border: 1px solid light-dark(#e3e5e9, #2c2f36);
  font-family: ui-monospace, 'Cascadia Code', Consolas, monospace; font-size: 18px; line-height: 1.55; tab-size: 4; }
.reveal pre.shiki code { display: block; min-width: max-content; }
.reveal pre.shiki .line { display: inline-block; min-width: 100%; min-height: 1.55em; box-sizing: border-box; padding: 0 16px 0 4.6em; position: relative; white-space: pre; vertical-align: top; }
.reveal pre.shiki .line::before { content: attr(data-n); position: absolute; left: 0; width: 2.6em; text-align: right; color: light-dark(#a0a4ad, #5b606b); }
.reveal pre.shiki .line.add { background: light-dark(rgba(26,127,55,.12), rgba(63,185,80,.14)); }
.reveal pre.shiki .line.del { background: light-dark(rgba(207,34,46,.10), rgba(248,81,73,.13)); }
.reveal pre.shiki .line.add::after, .reveal pre.shiki .line.del::after { position: absolute; left: 3.2em; font-weight: 700; }
.reveal pre.shiki .line.add::after { content: '+'; color: light-dark(#1a7f37, #3fb950); }
.reveal pre.shiki .line.del::after { content: '−'; color: light-dark(#cf222e, #f85149); }
.reveal pre.shiki .line.del span { text-decoration: line-through; text-decoration-color: light-dark(rgba(207,34,46,.45), rgba(248,81,73,.5)); }
.reveal pre.shiki span { color: light-dark(var(--shiki-light), var(--shiki-dark)); }
.reveal .controls { color: light-dark(#4f46e5, #8b85ff); }
.reveal .progress { color: light-dark(#4f46e5, #8b85ff); }
`

// deckHTML is the whole document for the iframe. scheme is the dialog's
// color-scheme ("light", "dark" or "light dark").
export async function deckHTML(deck: Deck, text: DeckText, scheme: string): Promise<string> {
  const slides = [`<section><div class="slide">
    <h2>${esc(text.title)}</h2>
    <p class="summary">${esc(text.summary)}</p>
    <ul>${[...new Set(deck.slides.map(s => s.rel))].map(r => `<li>${esc(r)}</li>`).join('')}</ul>
    <p class="keys">${esc(text.keys)}</p>
  </div></section>`]
  for (const s of deck.slides) {
    slides.push(`<section><div class="slide">
      <div class="file"><b>${esc(text.file(s))}</b>${s.added ? `<span class="add">+${s.added}</span>` : ''}${s.removed ? `<span class="del">−${s.removed}</span>` : ''}</div>
      ${await codeOf(s)}
    </div></section>`)
  }
  if (deck.others.length) {
    slides.push(`<section><div class="slide"><h2>${esc(text.others)}</h2><ul>${deck.others.map(r => `<li>${esc(r)}</li>`).join('')}</ul></div></section>`)
  }
  const safeScheme = /^(light|dark|light dark|dark light)$/.test(scheme.trim()) ? scheme.trim() : 'light dark'
  return `<!doctype html><html><head><meta charset="utf-8"><style>${revealCss}</style><style>:root{--scheme:${safeScheme}}${CSS}</style></head>
<body><div class="reveal"><div class="slides">${slides.join('\n')}</div></div>
<script src="${esc(new URL(playerUrl, location.href).href)}"></script></body></html>`
}
