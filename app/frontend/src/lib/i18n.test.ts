import { describe, expect, it } from 'vitest'
import en from '../locales/en.json'
import { t } from './i18n.svelte'
import { SECTION_KINDS } from './sections'

// Every key the source uses must exist in en.json.
const sources = import.meta.glob('../**/*.{svelte,ts}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('i18n', () => {
  const dict = en as Record<string, string>

  it('has every static key used in the source', () => {
    const missing = new Set<string>()
    for (const [file, src] of Object.entries(sources)) {
      if (file.endsWith('.test.ts')) continue
      for (const m of src.matchAll(/\bt\('([a-zA-Z0-9_.]+)'/g)) {
        const key = m[1] as string
        if (!(key in dict)) missing.add(`${key} (${file})`)
      }
    }
    expect([...missing]).toEqual([])
  })

  it('has the keys built from data', () => {
    for (const s of ['idle', 'starting', 'thinking', 'writing', 'running_tools', 'needs_approval', 'unread', 'error', 'exited']) {
      expect(dict).toHaveProperty(`state.${s}`)
    }
    for (const p of ['chat', 'cowork', 'code']) {
      expect(dict).toHaveProperty(`profile.${p}.desc`)
      expect(dict).toHaveProperty(`profile.${p}.empty`)
    }
    for (const th of ['system', 'light', 'dark']) expect(dict).toHaveProperty(`theme.${th}`)
    for (const k of SECTION_KINDS) expect(dict).toHaveProperty(`kind.${k}`)
  })

  it('fills parameters', () => {
    expect(t('nav.position', { i: 2, n: 5 })).toBe('Page 2 of 5')
    expect(t('no.such.key')).toBe('no.such.key')
  })
})
