import { describe, expect, it } from 'vitest'
import { autosize } from './actions'

const tick = () => new Promise(r => queueMicrotask(() => r(null)))

describe('autosize', () => {
  // jsdom has no layout: the text's height is 20px a line.
  function box() {
    const t = document.createElement('textarea')
    Object.defineProperty(t, 'scrollHeight', { get: () => 20 * Math.max(1, t.value.split('\n').length) })
    document.body.append(t)
    return t
  }

  it('grows with typing, up to its most', async () => {
    const t = box()
    autosize(t, { max: 100 })
    t.value = 'a\nb\nc'
    t.dispatchEvent(new Event('input'))
    expect(t.style.height).toBe('60px')
    t.value = 'a\n'.repeat(20)
    t.dispatchEvent(new Event('input'))
    expect(t.style.height).toBe('100px')
  })

  it('shrinks back when the text is cleared from code, as after Send', async () => {
    const t = box()
    const a = autosize(t, { max: 240, value: '' })
    t.value = 'a\nb\nc\nd\ne'
    t.dispatchEvent(new Event('input'))
    expect(t.style.height).toBe('100px')
    t.value = '' // no input event
    a.update({ max: 240, value: '' })
    await tick()
    expect(t.style.height).toBe('20px')
  })
})
