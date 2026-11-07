import { describe, expect, it } from 'vitest'
import { deckPlayerSource } from '../../build/deck-player'

describe('the deck’s script file', () => {
  it('is reveal.js as a plain script, then the deck’s player', () => {
    const src = deckPlayerSource()
    expect(src).toMatch(/e\.Reveal\s*=/) // the UMD build defines the global
    expect(src).toContain("'uncli-deck'")
    expect(src.indexOf('new Reveal(')).toBeGreaterThan(src.indexOf('e.Reveal'))
    expect(src).not.toMatch(/^\s*export\s/m)
    expect(() => new Function(src)).not.toThrow() // parses as a classic script
  })
})
