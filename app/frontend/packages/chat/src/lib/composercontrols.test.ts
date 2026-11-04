import { describe, expect, it, vi } from 'vitest'
import { buttonOf, kindOf, type ComposerControl } from './composercontrols'

const modes = [
  { id: 'chat', label: 'Chat' },
  { id: 'edit', label: 'Edit' },
]

describe('a host’s composer controls', () => {
  it('draws one option as a button, several as a drop-down, and a toggle only when asked', () => {
    const onChange = vi.fn()
    expect(kindOf({ id: 'mode', options: modes, onChange })).toBe('menu')
    expect(kindOf({ id: 'mode', options: modes.slice(0, 1), onChange })).toBe('button')
    expect(kindOf({ id: 'mode', kind: 'toggle', value: 'chat', states: modes, onChange })).toBe('toggle')
    expect(kindOf({ id: 'review', label: 'Review', onClick: () => {} })).toBe('button')
  })

  it('clicks a one-option button through onChange, a plain one through onClick', () => {
    const onChange = vi.fn()
    const one: ComposerControl = { id: 'mode', options: modes.slice(0, 1), onChange }
    buttonOf(one).click()
    expect(onChange).toHaveBeenCalledWith('chat')
    expect(buttonOf(one).item.label).toBe('Chat')
    const onClick = vi.fn()
    buttonOf({ id: 'review', label: 'Review', icon: 'list-checks', onClick }).click()
    expect(onClick).toHaveBeenCalled()
  })
})
