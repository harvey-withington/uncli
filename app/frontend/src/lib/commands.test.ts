import { describe, expect, it } from 'vitest'
import doc from '../../../../docs/UI-CONVENTIONS.md?raw'
import { keyLabel, LOCAL_KEYS, matches, rank, score, SHORTCUTS, type Command } from './commands'

const ev = (key: string, mods: Partial<Record<'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey', boolean>> = {}) =>
  ({ key, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...mods })

describe('matches', () => {
  it('matches letters either case, ctrl or ⌘, and shift only where it counts', () => {
    expect(matches({ key: 'b' }, ev('B', { shiftKey: true }))).toBe(true)
    expect(matches({ key: 'b' }, ev('b', { ctrlKey: true }))).toBe(false)
    expect(matches({ key: 'n', ctrl: true }, ev('n', { metaKey: true }))).toBe(true)
    expect(matches({ key: 'p', ctrl: true, shift: true }, ev('P', { ctrlKey: true, shiftKey: true }))).toBe(true)
    expect(matches({ key: 'p', ctrl: true, shift: true }, ev('p', { ctrlKey: true }))).toBe(false)
    expect(matches({ key: '?' }, ev('?', { shiftKey: true }))).toBe(true)
    expect(matches({ key: 'ArrowLeft' }, ev('ArrowLeft', { altKey: true }))).toBe(false)
  })
  it('labels keys for the platform', () => {
    expect(keyLabel({ key: 'p', ctrl: true, shift: true })).toEqual(['Ctrl', 'Shift', 'P'])
    expect(keyLabel({ key: 'n', ctrl: true }, true)).toEqual(['⌘', 'N'])
    expect(keyLabel({ key: 'ArrowLeft' })).toEqual(['←'])
  })
})

describe('score and rank', () => {
  const cmd = (id: string, label: string): Command => ({ id, group: 'app', label, run: () => {} })
  const list = [cmd('new', 'New session'), cmd('pin', 'Pin this page'), cmd('never', 'Prompt me: Never'), cmd('usage', 'Usage over time')]
  it('matches word prefixes in order, then letters in order', () => {
    expect(score('new', 'New session')).toBeGreaterThan(score('ses', 'New session'))
    expect(score('pr nev', 'Prompt me: Never')).toBeGreaterThan(0)
    expect(score('nev pr', 'Prompt me: Never')).toBeLessThan(score('pr nev', 'Prompt me: Never'))
    expect(score('usg', 'Usage over time')).toBe(1) // letters in order
    expect(score('xyz', 'Usage over time')).toBe(0)
  })
  it('puts recent commands first when the query does not decide', () => {
    expect(rank(list, '', ['usage', 'pin']).map(c => c.id)).toEqual(['usage', 'pin', 'new', 'never'])
    expect(rank(list, 'p', []).map(c => c.id)).toEqual(['pin', 'never'])
  })
})

// The contract's keyboard table must list every shortcut the app has.
describe('UI-CONVENTIONS keyboard table', () => {
  const table = doc.slice(doc.indexOf('## Keyboard'), doc.indexOf('##', doc.indexOf('## Keyboard') + 5))
  const keysCells = table.split('\n').filter(l => l.startsWith('| ') && !l.startsWith('| Keys') && !l.startsWith('|---')).map(l => l.split('|')[1]?.trim() ?? '')
  it('names each registry shortcut', () => {
    for (const s of SHORTCUTS) {
      for (const k of s.keys) {
        const words = keyLabel(k)
        const wanted = words.join('+')
        expect(keysCells.some(c => c.includes(wanted)), `${s.id}: ${wanted} missing from the keyboard table`).toBe(true)
      }
    }
  })
  it('names each place-bound shortcut', () => {
    for (const k of LOCAL_KEYS) expect(keysCells.some(c => c.includes(k.keys.split(' / ')[0] ?? '')), k.keys).toBe(true)
  })
})
