import { beforeEach, describe, expect, it } from 'vitest'
import { forced, keepScroll, loadSpot, reveal, saveSpot, SPOTS_MAX, spotOf } from './scrollmemory'

// A scroller jsdom can measure: content height and viewport height are set
// by hand, and scrollTop is clamped as a browser does.
function scroller(height: number, view = 400) {
  const el = document.createElement('div')
  let top = 0
  let h = height
  Object.defineProperty(el, 'clientHeight', { get: () => view })
  Object.defineProperty(el, 'scrollHeight', { get: () => h })
  Object.defineProperty(el, 'scrollTop', {
    get: () => top,
    set: (v: number) => { top = Math.max(0, Math.min(v, Math.max(0, h - view))) },
  })
  document.body.append(el)
  return {
    el,
    // The viewer scrolls.
    scroll(to: number) { el.scrollTop = to; el.dispatchEvent(new Event('scroll')) },
    // The content changes height; a shorter one pulls the scroller up, as a browser does.
    resize(next: number) {
      h = next
      const before = top
      el.scrollTop = top
      if (top !== before) el.dispatchEvent(new Event('scroll'))
    },
  }
}

const frames = () => new Promise(r => setTimeout(r, 40))

describe('scroll memory', () => {
  beforeEach(() => localStorage.clear())

  it('keeps spots, and only the latest few hundred', () => {
    saveSpot('a', { top: 120.4, end: false, height: 900 }, 1)
    expect(loadSpot('a')).toEqual({ top: 120, end: false, height: 900 })
    expect(loadSpot('b')).toBeNull()
    for (let i = 0; i < SPOTS_MAX + 5; i++) saveSpot(`k${i}`, { top: i, end: false, height: 1 }, 10 + i)
    expect(loadSpot('a')).toBeNull()
    expect(loadSpot(`k${SPOTS_MAX + 4}`)?.top).toBe(SPOTS_MAX + 4)
    localStorage.setItem('uncli-scroll-spots', 'not json')
    expect(loadSpot('k1')).toBeNull()
  })

  it('tells the viewer’s scrolls from the browser’s', () => {
    const spot = { top: 600, end: false, height: 2000 }
    // Content shrank to 500: pinned at its end (100), above the spot.
    expect(forced({ scrollTop: 100, scrollHeight: 500, clientHeight: 400 }, spot)).toBe(true)
    // The viewer scrolled up with the content as it was.
    expect(forced({ scrollTop: 100, scrollHeight: 2000, clientHeight: 400 }, spot)).toBe(false)
    expect(forced({ scrollTop: 100, scrollHeight: 500, clientHeight: 400 }, null)).toBe(false)
    expect(spotOf({ scrollTop: 1600, scrollHeight: 2000, clientHeight: 400 }).end).toBe(true)
    expect(spotOf({ scrollTop: 0, scrollHeight: 300, clientHeight: 400 }).end).toBe(false)
  })

  it('holds the spot through content that shrinks for a moment', async () => {
    const s = scroller(2000)
    const keep = keepScroll(s.el, { key: 'file:a' })
    s.scroll(600)
    s.resize(300) // rewritten: nearly empty for a moment
    expect(s.el.scrollTop).toBe(0)
    s.resize(2000)
    s.el.append(document.createElement('p')) // the new text renders
    await frames()
    expect(s.el.scrollTop).toBe(600)
    keep.destroy()
    expect(loadSpot('file:a')?.top).toBe(600)
  })

  it('comes back after a restart, and each key has its own spot', async () => {
    saveSpot('file:a', { top: 500, end: false, height: 2000 })
    const s = scroller(2000)
    const keep = keepScroll(s.el, { key: 'file:a' })
    await frames()
    expect(s.el.scrollTop).toBe(500)
    keep.update({ key: 'file:b' })
    expect(s.el.scrollTop).toBe(0) // never left anywhere: the top
    s.scroll(200)
    keep.update({ key: 'file:a' })
    await frames()
    expect(s.el.scrollTop).toBe(500)
    expect(loadSpot('file:b')?.top).toBe(200)
    keep.destroy()
  })

  it('stays at the end as the content grows, when left there or following', async () => {
    const s = scroller(1000)
    const keep = keepScroll(s.el, { key: 'log' })
    s.scroll(600) // the end
    s.resize(1500)
    s.el.append(document.createElement('p'))
    await frames()
    expect(s.el.scrollTop).toBe(1100)
    keep.update({ key: 'other', follow: true })
    s.resize(3000)
    s.el.append(document.createElement('p'))
    await frames()
    expect(s.el.scrollTop).toBe(2600)
    keep.destroy()
  })

  it('reveals a row by scrolling its own list only, up or down', () => {
    const outer = document.createElement('div')
    outer.style.overflowY = 'auto'
    const list = document.createElement('ul')
    list.style.overflowY = 'auto'
    const row = document.createElement('li')
    list.append(row)
    outer.append(list)
    document.body.append(outer)
    Object.defineProperty(list, 'clientHeight', { get: () => 100 })
    Object.defineProperty(row, 'offsetHeight', { get: () => 20 })
    list.getBoundingClientRect = () => ({ top: 50 }) as DOMRect
    let rowTop = 200 // below the list's 100 px
    row.getBoundingClientRect = () => ({ top: rowTop - list.scrollTop + 50 }) as DOMRect
    reveal(row)
    expect(list.scrollTop).toBe(120) // its bottom (220) at the list's bottom
    expect(outer.scrollTop).toBe(0)
    expect(outer.scrollLeft).toBe(0)
    rowTop = 40 // above the view now
    reveal(row)
    expect(list.scrollTop).toBe(40)
    reveal(null)
  })
})
