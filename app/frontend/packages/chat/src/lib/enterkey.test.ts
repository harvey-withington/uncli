import { describe, expect, it } from 'vitest'
import { resolveEnter } from './enterkey.svelte'

describe('what Enter does', () => {
  it('follows the host, then the viewer, then the device', () => {
    // A touch keyboard has no practical Shift+Enter: a new line by default.
    expect(resolveEnter(undefined, 'device', true)).toBe('newline')
    expect(resolveEnter(undefined, 'device', false)).toBe('send')
    // The viewer's setting beats the device.
    expect(resolveEnter(undefined, 'send', true)).toBe('send')
    expect(resolveEnter(undefined, 'newline', false)).toBe('newline')
    // A host beats both, to match its own chat.
    expect(resolveEnter('send', 'newline', true)).toBe('send')
    expect(resolveEnter('newline', 'send', false)).toBe('newline')
  })
})
