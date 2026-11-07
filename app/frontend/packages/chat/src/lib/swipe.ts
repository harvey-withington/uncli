// Swiping sideways on a touch screen to move between pages, as the
// walkthrough's slides do. Touch only: a mouse drags and selects as ever.
// The browser keeps vertical scrolling (touch-action: pan-y); what's left
// to read is a sideways swipe: clearly sideways, long enough and quick.
// It never takes a swipe that starts by a screen edge (the phone's own
// back gesture), in something that scrolls sideways (a code block, a
// table), or in a box being typed in, nor one that ends with text selected.

export const SWIPE = {
  edge: 24, // px from the screen's edges left to the phone
  min: 64, // px sideways at least
  ratio: 1.6, // sideways over up-and-down, at least
  ms: 700, // at most
}

export interface Stroke {
  x0: number
  y0: number
  x1: number
  y1: number
  ms: number
}

// swipeOf is where a finished stroke goes: 1 for the next page (swiped
// left), -1 for the previous (swiped right), 0 for neither.
export function swipeOf(s: Stroke): 1 | -1 | 0 {
  const dx = s.x1 - s.x0
  const dy = s.y1 - s.y0
  if (Math.abs(dx) < SWIPE.min || Math.abs(dx) < SWIPE.ratio * Math.abs(dy) || s.ms > SWIPE.ms) return 0
  return dx < 0 ? 1 : -1
}

// startsSwipe is whether a touch may start a swipe: not by an edge, not in
// a box being typed in, not in anything (up to the swiping area) that
// scrolls sideways.
export function startsSwipe(target: Element | null, x: number, width: number, area: Element): boolean {
  if (x < SWIPE.edge || x > width - SWIPE.edge) return false
  if (target?.closest('input, textarea, select, [contenteditable="true"], [contenteditable=""]')) return false
  for (let el = target; el && el !== area; el = el.parentElement) {
    if (el.scrollWidth > el.clientWidth + 1) {
      const ox = getComputedStyle(el).overflowX
      if (ox === 'auto' || ox === 'scroll') return false
    }
  }
  return true
}

// slideIn shows a page arriving from the side it was swiped towards
// (none with reduced motion).
export function slideIn(el: HTMLElement | undefined, dir: 1 | -1): void {
  if (!el || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  el.animate?.([{ transform: `translateX(${dir * 40}px)`, opacity: 0.3 }, { transform: 'none', opacity: 1 }], { duration: 180, easing: 'ease-out' })
}

// swipe is the action that reads swipes on an element.
export function swipe(node: HTMLElement, onswipe: (dir: 1 | -1) => void) {
  let handler = onswipe
  let start: { x: number; y: number; t: number; id: number } | null = null
  node.style.touchAction = 'pan-y pinch-zoom'
  // Or the browser takes a sideways swipe as Back, and leaves the app.
  node.style.overscrollBehaviorX = 'contain'
  const down = (e: PointerEvent) => {
    start = e.pointerType === 'touch' && e.isPrimary && startsSwipe(e.target as Element, e.clientX, window.innerWidth, node)
      ? { x: e.clientX, y: e.clientY, t: e.timeStamp, id: e.pointerId }
      : null
  }
  const up = (e: PointerEvent) => {
    if (!start || e.pointerId !== start.id) return
    const dir = swipeOf({ x0: start.x, y0: start.y, x1: e.clientX, y1: e.clientY, ms: e.timeStamp - start.t })
    start = null
    if (dir && !window.getSelection()?.toString()) handler(dir)
  }
  const cancel = () => (start = null)
  node.addEventListener('pointerdown', down)
  node.addEventListener('pointerup', up)
  node.addEventListener('pointercancel', cancel)
  return {
    update(next: (dir: 1 | -1) => void) {
      handler = next
    },
    destroy() {
      node.removeEventListener('pointerdown', down)
      node.removeEventListener('pointerup', up)
      node.removeEventListener('pointercancel', cancel)
      node.style.touchAction = ''
      node.style.overscrollBehaviorX = ''
    },
  }
}
