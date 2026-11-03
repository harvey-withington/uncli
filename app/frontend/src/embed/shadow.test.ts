import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { shadowCss } from './shadow'

describe('shadowCss', () => {
  it('reads the theme’s :root as :host, schemes and all', () => {
    const css = shadowCss(`:root { --bg: var(--uncli-bg, #fff); }
@media (prefers-color-scheme: dark) { :root:not([data-uncli-scheme='light']) { --bg: #111; } }
:root[data-uncli-scheme='dark'] { --bg: #111; }`)
    expect(css).toContain(':host { --bg: var(--uncli-bg, #fff); }')
    expect(css).toContain(":host(:not([data-uncli-scheme='light'])) { --bg: #111; }")
    expect(css).toContain(":host([data-uncli-scheme='dark']) { --bg: #111; }")
    expect(css).not.toMatch(/:root/)
  })

  it('leaves no :root in UNCLI’s real theme, and keeps every public token', () => {
    const theme = readFileSync('src/styles/theme.css', 'utf8')
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
