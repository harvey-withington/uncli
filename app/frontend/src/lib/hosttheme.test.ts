import { afterEach, describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import doc from '../../../../docs/UI-CONVENTIONS.md?raw'
import { applyThemeFile, themeCSS, THEME_TOKENS } from './hosttheme'

// ?raw gives an empty stylesheet under vitest, so read it from disk.
const css = readFileSync('src/app.css', 'utf8')

describe('theme contract', () => {
  it('app.css reads exactly the public tokens', () => {
    const read = new Set([...css.matchAll(/var\(--uncli-([a-z0-9-]+)/g)].map(m => m[1]))
    expect([...read].sort()).toEqual([...THEME_TOKENS].sort())
  })

  it('UI-CONVENTIONS lists exactly the public tokens', () => {
    const section = doc.split('## Theme override')[1]?.split('\n## ')[0] ?? ''
    const listed = new Set([...section.matchAll(/`--uncli-([a-z0-9-]+)`/g)].map(m => m[1]))
    expect([...listed].sort()).toEqual([...THEME_TOKENS].sort())
  })
})

describe('themeCSS', () => {
  it('sets each scheme under its own attribute', () => {
    const { css, warnings } = themeCSS({ light: { bg: '#fff', font: "'Inter', sans-serif" }, dark: { accent: 'oklch(70% 0.1 200)' } })
    expect(warnings).toEqual([])
    expect(css).toContain(":root[data-uncli-scheme='light'] {\n  --uncli-bg: #fff;\n  --uncli-font: 'Inter', sans-serif;\n}")
    expect(css).toContain(":root[data-uncli-scheme='dark'] {\n  --uncli-accent: oklch(70% 0.1 200);\n}")
  })

  it('skips unknown names and refuses values that could add rules or fetch', () => {
    const { css, warnings } = themeCSS({
      light: {
        nope: 'red',
        bg: 'red; } body { display: none',
        surface: 'url(https://example.com/x.png)',
        text: '"unclosed',
        border: '@import x',
        accent: 'var(--host-accent)',
      },
    })
    expect(warnings).toHaveLength(5)
    expect(css).toBe(":root[data-uncli-scheme='light'] {\n  --uncli-accent: var(--host-accent);\n}")
  })
})

describe('applyThemeFile', () => {
  afterEach(() => applyThemeFile(null))

  it('replaces its one style element, and none removes it', () => {
    applyThemeFile({ light: { bg: '#fff' } })
    applyThemeFile({ dark: { bg: '#000' } })
    const els = document.querySelectorAll('#uncli-theme-file')
    expect(els).toHaveLength(1)
    expect(els[0]?.textContent).toContain('#000')
    applyThemeFile(null)
    expect(document.getElementById('uncli-theme-file')).toBeNull()
  })
})
