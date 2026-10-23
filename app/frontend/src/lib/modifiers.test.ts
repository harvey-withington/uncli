import { describe, expect, it } from 'vitest'
import type { Modifier } from './api'
import { arrange, choicesOf, chosenIn } from './modifiers'

const m = (id: string, extra: Partial<Modifier> = {}): Modifier => ({ id, label: id, icon: 'x', scope: 'turn', ...extra })

describe('toolbar modifiers', () => {
  it('keeps singles as toggles and a group as one choice, where it first appears', () => {
    const items = arrange([m('agents'), m('cheap', { group: 'g', tone: 'success' }), m('solo', { group: 'alone' }), m('dear', { group: 'g', tone: 'warning' })])
    expect(items.map(x => (x.kind === 'group' ? `group:${x.members.map(y => y.id).join('+')}` : x.modifier.id))).toEqual(['agents', 'group:cheap+dear', 'solo'])
  })

  it('orders choices by cost, Normal between, and knows which is on', () => {
    const members = [m('dear', { tone: 'warning' }), m('cheap', { tone: 'success' }), m('plain')]
    expect(choicesOf(members, 'Normal').map(c => c.id || 'Normal')).toEqual(['cheap', 'Normal', 'dear', 'plain'])
    expect(chosenIn(members, ['x', 'dear'])).toBe('dear')
    expect(chosenIn(members, ['x'])).toBe('')
  })
})

describe('cycling', () => {
  it('moves on and back, wrapping round; an unknown value counts as the first', async () => {
    const { step } = await import('../components/CycleButton.svelte')
    const cs = [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }, { id: 'c', label: 'C' }]
    expect(step(cs, 'a', 1)?.id).toBe('b')
    expect(step(cs, 'c', 1)?.id).toBe('a')
    expect(step(cs, 'a', -1)?.id).toBe('c')
    expect(step(cs, 'gone', 1)?.id).toBe('b')
    expect(step([], 'a', 1)).toBeUndefined()
  })
})
