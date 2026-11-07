import { describe, expect, it } from 'vitest'
import revealJs from 'reveal.js?raw'
import { revealScript } from './deckhtml'

describe('the deck’s reveal.js', () => {
  it('runs as a plain script, defining Reveal', () => {
    const script = revealScript(revealJs)
    expect(script).toContain('window.Reveal = ')
    expect(script).not.toMatch(/^\s*export\s/m)
    expect(() => new Function(script)).not.toThrow() // parses as a classic script
  })

  it('says so when the module changes shape', () => {
    expect(() => revealScript('const R = 1\nexport { R as Reveal }')).toThrow(/changed shape/)
  })
})
