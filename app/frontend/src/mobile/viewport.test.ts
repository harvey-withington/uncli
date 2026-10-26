import { describe, expect, it } from 'vitest'
import { followViewport } from './viewport'

class FakeViewport extends EventTarget {
  height = 844
  offsetTop = 0
}

describe('followViewport', () => {
  it('keeps the app to what shows above the keyboard, and stops', () => {
    const el = document.createElement('div')
    const vv = new FakeViewport()
    const stop = followViewport(el, vv)
    expect(el.style.position).toBe('fixed')
    expect(el.style.height).toBe('844px')
    // The keyboard opens and Safari scrolls the page up to the message box.
    vv.height = 480
    vv.offsetTop = 300
    vv.dispatchEvent(new Event('resize'))
    expect(el.style.height).toBe('480px')
    expect(el.style.transform).toBe('translateY(300px)')
    stop()
    vv.height = 844
    vv.dispatchEvent(new Event('resize'))
    expect(el.style.height).toBe('480px')
  })

  it('leaves the page alone without a visual viewport', () => {
    const el = document.createElement('div')
    followViewport(el, null)()
    expect(el.style.position).toBe('')
  })
})
