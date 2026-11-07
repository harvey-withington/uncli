import { describe, expect, it } from 'vitest'
import { startsSwipe, swipeOf } from './swipe'

const stroke = (dx: number, dy: number, ms = 200) => ({ x0: 200, y0: 300, x1: 200 + dx, y1: 300 + dy, ms })

describe('swiping between pages', () => {
  it('takes a clear, quick sideways swipe', () => {
    expect(swipeOf(stroke(-120, 10))).toBe(1) // left: the next page
    expect(swipeOf(stroke(120, -20))).toBe(-1) // right: the previous
  })

  it('leaves scrolling, short strokes and slow drags alone', () => {
    expect(swipeOf(stroke(-80, 70))).toBe(0) // mostly up and down
    expect(swipeOf(stroke(-40, 0))).toBe(0) // too short
    expect(swipeOf(stroke(-150, 0, 1500))).toBe(0) // too slow
  })

  it('doesn’t start by an edge, in a box being typed in, or in something that scrolls sideways', () => {
    const area = document.createElement('div')
    const p = document.createElement('p')
    const input = document.createElement('textarea')
    const code = document.createElement('pre')
    const line = document.createElement('span')
    code.append(line)
    area.append(p, input, code)
    document.body.append(area)
    code.style.overflowX = 'auto'
    Object.defineProperty(code, 'scrollWidth', { value: 900 })
    Object.defineProperty(code, 'clientWidth', { value: 300 })
    expect(startsSwipe(p, 200, 400, area)).toBe(true)
    expect(startsSwipe(p, 10, 400, area)).toBe(false) // the phone's back gesture
    expect(startsSwipe(p, 390, 400, area)).toBe(false)
    expect(startsSwipe(input, 200, 400, area)).toBe(false)
    expect(startsSwipe(line, 200, 400, area)).toBe(false)
    area.remove()
  })
})
