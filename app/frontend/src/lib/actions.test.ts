import { afterEach, describe, expect, it, vi } from 'vitest'
import { clickOutside, focusTrap, rootOf } from './actions'

// An embedded element's shadow root (decision 0026), with a node inside.
function embedded() {
  const host = document.createElement('div')
  document.body.append(host)
  const shadow = host.attachShadow({ mode: 'open' })
  const node = document.createElement('div')
  node.innerHTML = '<button id="a">A</button><button id="b">B</button>'
  shadow.append(node)
  return { host, shadow, node }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('actions inside a shadow root', () => {
  it('rootOf finds the shadow root and its host, or the document and body', () => {
    const { host, shadow, node } = embedded()
    expect(rootOf(node)).toEqual({ root: shadow, place: shadow, host })
    const plain = document.createElement('div')
    document.body.append(plain)
    expect(rootOf(plain)).toEqual({ root: document, place: document.body, host: document.body })
  })

  it('clickOutside tells a click inside the node from one outside, across the shadow boundary', () => {
    const { node } = embedded()
    const fn = vi.fn()
    const action = clickOutside(node, fn)
    node.querySelector('button')!.dispatchEvent(new Event('pointerdown', { bubbles: true, composed: true }))
    expect(fn).not.toHaveBeenCalled()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true, composed: true }))
    expect(fn).toHaveBeenCalledOnce()
    action.destroy()
  })

  it('focusTrap wraps Tab using the shadow root’s focused element', async () => {
    const { node, shadow } = embedded()
    const action = focusTrap(node)
    await Promise.resolve()
    const b = shadow.getElementById('b') as HTMLButtonElement
    b.focus()
    expect(shadow.activeElement).toBe(b)
    node.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(shadow.activeElement).toBe(shadow.getElementById('a'))
    action.destroy()
  })
})
