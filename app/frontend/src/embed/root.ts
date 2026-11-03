// What every UNCLI element does to itself (decision 0026): a shadow root
// that adopts UNCLI's theme read as :host (one stylesheet shared by all of
// them), the host page's font for UNCLI's text, and UNCLI's scheme
// attribute from its own scheme attribute.
import themeCss from '../styles/theme.css?inline'
import { shadowCss } from './shadow'

let sheet: CSSStyleSheet | null = null
function stylesheet(): CSSStyleSheet | null {
  if (sheet || typeof CSSStyleSheet === 'undefined' || !('replaceSync' in CSSStyleSheet.prototype)) return sheet
  sheet = new CSSStyleSheet()
  sheet.replaceSync(shadowCss(themeCss))
  return sheet
}

// shadowFor gives an element its shadow root, styled, the first time.
export function shadowFor(el: HTMLElement): ShadowRoot {
  if (el.shadowRoot) return el.shadowRoot
  const root = el.attachShadow({ mode: 'open' })
  const s = stylesheet()
  if (s) root.adoptedStyleSheets = [s]
  else {
    const style = document.createElement('style')
    style.textContent = shadowCss(themeCss)
    root.append(style)
  }
  return root
}

// hostFont passes the host page's font to UNCLI's text (unless the host
// sets --uncli-font).
export function hostFont(el: HTMLElement) {
  const font = getComputedStyle(el.parentElement ?? document.body).fontFamily
  if (font) el.style.setProperty('--uncli-host-font', font)
}

// applyScheme sets UNCLI's scheme attribute, which the theme keys on, from
// the element's scheme attribute ("light" or "dark"; the system's otherwise).
export function applyScheme(el: HTMLElement) {
  const s = el.getAttribute('scheme')
  if (s === 'light' || s === 'dark') el.setAttribute('data-uncli-scheme', s)
  else el.removeAttribute('data-uncli-scheme')
}
