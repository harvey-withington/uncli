import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { shadowCss } from './shadow'

describe('shadowCss', () => {
  it('reads the theme’s :root as the chat’s own root inside the element, schemes from the element', () => {
    const css = shadowCss(`:root { --bg: var(--uncli-bg, #fff); }
@media (prefers-color-scheme: dark) { :root:not([data-uncli-scheme='light']) { --bg: #111; } }
:root[data-uncli-scheme='dark'] { --bg: #111; }`)
    expect(css).toContain('.uncli-root { --bg: var(--uncli-bg, #fff); }')
    expect(css).toContain(":host(:not([data-uncli-scheme='light'])) .uncli-root { --bg: #111; }")
    expect(css).toContain(":host([data-uncli-scheme='dark']) .uncli-root { --bg: #111; }")
    expect(css).not.toMatch(/:root/)
    // Nothing of UNCLI's working tokens on the element itself, where a
    // host's --uncli-accent: var(--accent) would find them.
    expect(css).not.toMatch(/:host(?:\((?:[^()]|\([^()]*\))*\))?\s*\{[^}]*--(?:bg|text|accent|border)\s*:/)
  })

  it('leaves no :root in UNCLI’s real theme, and keeps every public token', () => {
    const theme = readFileSync('packages/chat/src/styles/theme.css', 'utf8')
    const css = shadowCss(theme)
    expect(css).not.toMatch(/:root/)
    for (const token of theme.match(/--uncli-[a-z0-9-]+/g) ?? []) expect(css).toContain(token)
  })

  it('gives the element the host’s font unless the host sets one, and keeps overlays inside it', () => {
    const css = shadowCss('')
    expect(css).toContain('--font: var(--uncli-font, var(--uncli-host-font')
    expect(css).toMatch(/\.scrim, \.toasts \{ position: absolute !important; \}/)
  })
})
