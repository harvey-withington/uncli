// Where the viewer left each scroller (a page of answers, a watched file,
// an artifact), kept in localStorage so it holds across restarts, and held
// through updates: content that shrinks for a moment (a file rewritten,
// a block re-rendered) makes the browser pull the scroller up, which isn't
// the viewer's doing, so the spot they chose comes back once there is room
// for it. A spot at the end stays at the end as the content grows.
import { atEnd } from './keep'

const KEY = 'uncli-scroll-spots'
export const SPOTS_MAX = 300

export interface Spot {
  top: number
  end: boolean // left at the end: follow it as it grows
  height: number // the content's height then (a smaller one later means it shrank)
}

type Stored = Record<string, [top: number, end: 0 | 1, height: number, at: number]>

function read(): Stored {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    return v && typeof v === 'object' && !Array.isArray(v) ? (v as Stored) : {}
  } catch {
    return {}
  }
}

export function loadSpot(key: string): Spot | null {
  const v = read()[key]
  if (!Array.isArray(v) || typeof v[0] !== 'number') return null
  return { top: v[0], end: v[1] === 1, height: typeof v[2] === 'number' ? v[2] : 0 }
}

// saveSpot keeps a spot, and only the latest SPOTS_MAX of them.
export function saveSpot(key: string, spot: Spot, now = Date.now()) {
  const all = read()
  all[key] = [Math.round(spot.top), spot.end ? 1 : 0, Math.round(spot.height), now]
  const keys = Object.keys(all)
  if (keys.length > SPOTS_MAX) {
    keys.sort((a, b) => (all[a]?.[3] ?? 0) - (all[b]?.[3] ?? 0))
    for (const k of keys.slice(0, keys.length - SPOTS_MAX)) delete all[k]
  }
  try {
    localStorage.setItem(KEY, JSON.stringify(all))
  } catch {
    // not kept; still held for this run
  }
}

type Box = { scrollTop: number; scrollHeight: number; clientHeight: number }

// forced: a scroll the browser made because the content got shorter (the
// scroller pinned at its new end, above the viewer's spot), not the viewer.
export function forced(el: Box, spot: Spot | null): boolean {
  if (!spot) return false
  const pinned = el.scrollHeight - el.clientHeight - el.scrollTop <= 1
  return el.scrollTop < spot.top && pinned && el.scrollHeight < spot.height
}

// spotOf is where a scroller is now. The end counts only when there is
// something to scroll.
export function spotOf(el: Box): Spot {
  return { top: el.scrollTop, end: el.scrollTop > 0 && el.scrollHeight > el.clientHeight && atEnd(el), height: el.scrollHeight }
}

export interface ScrollOptions {
  key: string // what is showing ('' for nothing worth keeping)
  follow?: boolean // always show the end (the page's follow toggle)
}

// keepScroll holds a scroller's spot for its key: as a Svelte action
// (use:keepScroll={{ key }}) or called with an element.
export function keepScroll(el: HTMLElement, options: ScrollOptions) {
  let key = options.key
  let follow = !!options.follow
  let spot: Spot | null = key ? loadSpot(key) : null
  let mine = -1 // the scrollTop we last set, so its scroll event isn't taken for the viewer's
  let timer: ReturnType<typeof setTimeout> | undefined
  const frame = (f: () => void) => (window.requestAnimationFrame ?? ((g: FrameRequestCallback) => window.setTimeout(g, 16)))(f)

  const set = (top: number) => {
    el.scrollTop = top
    mine = el.scrollTop
  }
  // Put the scroller where it belongs: the end when following or left
  // there, else the viewer's spot (the browser stops it short while the
  // content is shorter; the next change tries again).
  const apply = () => {
    if (follow || spot?.end) {
      if (el.scrollHeight > el.clientHeight) set(el.scrollHeight)
    } else if (spot && Math.abs(el.scrollTop - spot.top) >= 1) set(spot.top)
  }
  let queued = false
  const later = () => {
    if (queued) return
    queued = true
    frame(() => {
      queued = false
      apply()
    })
  }
  const flush = () => {
    if (timer === undefined) return
    clearTimeout(timer)
    timer = undefined
    if (key && spot) saveSpot(key, spot)
  }
  const onScroll = () => {
    if (Math.abs(el.scrollTop - mine) < 1 || forced(el, spot)) return
    mine = -1
    spot = spotOf(el)
    clearTimeout(timer)
    timer = setTimeout(flush, 250)
  }

  el.addEventListener('scroll', onScroll, { passive: true })
  el.addEventListener('load', later, true) // an image arriving
  window.addEventListener('pagehide', flush)
  const mo = typeof MutationObserver === 'undefined' ? null : new MutationObserver(later)
  mo?.observe(el, { childList: true, subtree: true, characterData: true })
  const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(later)
  ro?.observe(el)
  const show = () => {
    if (spot || follow) later()
    else set(0) // never left here: the top
  }
  show()

  return {
    update(next: ScrollOptions) {
      const nextFollow = !!next.follow
      if (next.key !== key) {
        flush()
        key = next.key
        spot = key ? loadSpot(key) : null
        follow = nextFollow
        show()
      } else if (nextFollow !== follow) {
        follow = nextFollow
        if (!follow && el.scrollHeight > el.clientHeight) spot = spotOf(el)
        later()
      }
    },
    destroy() {
      flush()
      el.removeEventListener('scroll', onScroll)
      el.removeEventListener('load', later, true)
      window.removeEventListener('pagehide', flush)
      mo?.disconnect()
      ro?.disconnect()
    },
  }
}

// reveal scrolls the nearest scrolling box around an item (a list's
// highlighted row) just enough to show it, up or down only. Unlike
// scrollIntoView it moves nothing else: not the boxes further out, and not
// sideways, so a drop-down reaching past its panel can't shift the app.
export function reveal(item: Element | null | undefined) {
  if (!(item instanceof HTMLElement)) return
  let box = item.parentElement
  while (box && !/(auto|scroll)/.test(getComputedStyle(box).overflowY)) box = box.parentElement
  if (!box) return
  const top = item.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop
  const bottom = top + item.offsetHeight
  if (top < box.scrollTop) box.scrollTop = top
  else if (bottom > box.scrollTop + box.clientHeight) box.scrollTop = bottom - box.clientHeight
}
