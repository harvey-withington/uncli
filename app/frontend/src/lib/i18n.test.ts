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
    for (const m of ['always', 'unsafe', 'never']) {
      for (const k of [`mode.${m}`, `mode.${m}.hint`]) expect(dict).toHaveProperty(k)
    }
    for (const v of ['safe', 'unsafe', 'blocked']) expect(dict).toHaveProperty(`verdict.${v}`)
    for (const f of ['inline', 'complex', 'context']) expect(dict).toHaveProperty(`fixed.${f}`)
    for (const f of ['any', 'project', 'all', 'builtin']) expect(dict).toHaveProperty(`safe.filter.${f}`)
    for (const b of ['looks', 'safe', 'inside', 'never', 'always', 'unknown', 'judging', 'user']) expect(dict).toHaveProperty(`why.${b}`)
    for (const a of ['you', 'listed', 'builtin', 'safe', 'looks', 'inside', 'never', 'mixed']) expect(dict).toHaveProperty(`trace.allowed.${a}`)
    for (const r of ['deletes', 'discards', 'outside', 'publishes', 'installs', 'system', 'stops', 'remote', 'secrets', 'runs-code', 'cloud']) expect(dict).toHaveProperty(`risk.${r}`)
  })

  it('fills parameters', () => {
    expect(t('nav.position', { i: 2, n: 5 })).toBe('Page 2 of 5')
    expect(t('no.such.key')).toBe('no.such.key')
  })
})
