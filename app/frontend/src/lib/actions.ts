// Shared DOM behaviours as Svelte actions.

// clickOutside calls fn when a pointer goes down outside the node.
export function clickOutside(node: HTMLElement, fn: () => void) {
  const handler = (e: PointerEvent) => {
    if (!node.contains(e.target as Node)) fn()
  }
  document.addEventListener('pointerdown', handler, true)
  return {
    update(next: () => void) { fn = next },
    destroy() { document.removeEventListener('pointerdown', handler, true) },
  }
}

// focusTrap keeps Tab inside the node and focuses its first control.
export function focusTrap(node: HTMLElement) {
  const selector = 'button:not([disabled]), [href], input:not([disabled]), select, textarea, [tabindex]:not([tabindex="-1"])'
  const focusables = () => Array.from(node.querySelectorAll<HTMLElement>(selector))
  const autofocus = node.querySelector<HTMLElement>('[data-autofocus]') ?? focusables()[0]
  queueMicrotask(() => autofocus?.focus())
  const onKey = (e: KeyboardEvent) => {
    if (e.key !== 'Tab') return
    const f = focusables()
    if (f.length === 0) return
    const first = f[0] as HTMLElement
    const last = f[f.length - 1] as HTMLElement
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }
  node.addEventListener('keydown', onKey)
  return { destroy() { node.removeEventListener('keydown', onKey) } }
}

// autosize grows a textarea with its content up to max pixels.
export function autosize(node: HTMLTextAreaElement, max = 240) {
  const resize = () => {
    node.style.height = 'auto'
    node.style.height = `${Math.min(node.scrollHeight, max)}px`
  }
  node.addEventListener('input', resize)
  queueMicrotask(resize)
  return { update() { resize() }, destroy() { node.removeEventListener('input', resize) } }
}

export interface DragSortOptions {
  item: string // selector for the draggable items, each with data-id
  onmove: (id: string, to: number) => void // to = index in the list as shown before the move
}

// dragSort makes a vertical list reorderable by dragging its items. A drag
// starts after a few pixels of movement, so clicks still work. A lifted
// copy of the item (.drag-ghost) follows the pointer; the item itself stays
// as an empty slot (.dragging) that moves to where it will land while the
// others slide out of its way, so the gap is the drop indicator. On release
// the ghost settles into the slot, then the list is reordered. Escape
// cancels. The list's scroller follows the pointer near its edges. Inputs
// and controls marked data-no-drag inside an item don't start a drag.
export function dragSort(node: HTMLElement, opts: DragSortOptions) {
  let options = opts
  let cancel: (() => void) | null = null

  const onpointerdown = (e: PointerEvent) => {
    if (e.button !== 0 || cancel) return
    const target = e.target as HTMLElement
    const item = target.closest<HTMLElement>(options.item)
    if (!item || !node.contains(item) || target.closest('input, textarea, [data-no-drag]')) return
    const id = item.dataset.id
    if (!id) return
    const startY = e.clientY
    let started = false
    let items: HTMLElement[] = []
    let rects: DOMRect[] = []
    let from = 0
    let slot = 0 // where the item will land, as an index in the list without it moved
    let shift = 0 // how far the others move to make room: its height plus the gap
    let ghost: HTMLElement | null = null
    let grabOffset = 0 // pointer to ghost top
    let lastY = startY
    const scroller = scrollParent(node)
    const scroll0 = scroller?.scrollTop ?? 0
    const scrolled = () => (scroller?.scrollTop ?? 0) - scroll0

    const start = () => {
      started = true
      items = Array.from(node.querySelectorAll<HTMLElement>(options.item))
      rects = items.map(el => el.getBoundingClientRect())
      from = slot = items.indexOf(item)
      const r = rects[from] as DOMRect
      const next = rects[from + 1] ?? rects[from - 1]
      const gap = next ? Math.max(0, next.top > r.top ? next.top - r.bottom : r.top - next.bottom) : 0
      shift = r.height + gap
      grabOffset = startY - r.top
      ghost = item.cloneNode(true) as HTMLElement
      ghost.classList.add('drag-ghost')
      ghost.removeAttribute('data-id')
      ghost.setAttribute('aria-hidden', 'true')
      // Inline, so the item's own positioning can't override it.
      Object.assign(ghost.style, { position: 'fixed', zIndex: '1000', margin: '0', left: `${r.left}px`, top: `${r.top}px`, width: `${r.width}px`, height: `${r.height}px` })
      document.body.appendChild(ghost)
      item.classList.add('dragging')
      node.classList.add('sorting')
      document.body.classList.add('grabbing') // grab cursor, no text selection while dragging
      document.addEventListener('keydown', onkey, true)
    }

    // Lay the list out for the item landing at slot: the item's empty slot
    // moves there and the ones it passes slide the other way.
    const layout = () => {
      items.forEach((el, i) => {
        let y = 0
        if (i === from) y = slot > from ? (rects[slot] as DOMRect).bottom - (rects[from] as DOMRect).bottom : slot < from ? (rects[slot] as DOMRect).top - (rects[from] as DOMRect).top : 0
        else if (slot > from && i > from && i <= slot) y = -shift
        else if (slot < from && i >= slot && i < from) y = shift
        el.style.transform = y ? `translateY(${y}px)` : ''
      })
    }

    const follow = (y: number) => {
      lastY = y
      if (!ghost) return
      const r = rects[from] as DOMRect
      ghost.style.transform = `translateY(${y - grabOffset - r.top}px)`
      // The ghost's middle, in the list's own coordinates at the start.
      const mid = y - grabOffset + r.height / 2 + scrolled()
      let s = 0
      rects.forEach((o, i) => {
        if (i !== from && mid > o.top + o.height / 2) s++
      })
      if (s !== slot) {
        slot = s
        layout()
      }
    }

    const onmove = (ev: PointerEvent) => {
      if (!started) {
        if (Math.abs(ev.clientY - startY) < 5) return
        start()
      }
      ev.preventDefault()
      autoScroll(ev.clientY)
      follow(ev.clientY)
    }

    // Near the scroller's top or bottom edge, scroll towards it.
    const autoScroll = (y: number) => {
      if (!scroller) return
      const box = scroller.getBoundingClientRect()
      const edge = 36
      const step = y < box.top + edge ? -(box.top + edge - y) / 3 : y > box.bottom - edge ? (y - (box.bottom - edge)) / 3 : 0
      if (step) scroller.scrollTop += Math.round(step)
    }
    const onscroll = () => follow(lastY)

    const detach = () => {
      document.removeEventListener('pointermove', onmove)
      document.removeEventListener('pointerup', onup)
      document.removeEventListener('pointercancel', oncancel)
      document.removeEventListener('keydown', onkey, true)
      scroller?.removeEventListener('scroll', onscroll)
      document.body.classList.remove('grabbing')
    }

    // The ghost glides into the slot (or home), then the list is put back
    // in plain order: reordered by the caller, or as it was.
    const finish = (commit: boolean) => {
      detach()
      cancel = null
      if (!started || !ghost) return
      const g = ghost
      if (!commit) slot = from
      layout()
      const home = rects[from] as DOMRect
      const landing = slot > from ? (rects[slot] as DOMRect).bottom - home.bottom : slot < from ? (rects[slot] as DOMRect).top - home.top : 0
      g.classList.add('settling')
      g.style.transform = `translateY(${landing - scrolled()}px)`
      const done = () => {
        node.classList.add('settled') // no transitions while transforms are cleared
        if (commit && slot !== from) options.onmove(id, slot > from ? slot + 1 : slot)
        // Once the list has re-rendered in its new order, drop the transforms.
        frame(() => {
          items.forEach(el => (el.style.transform = ''))
          item.classList.remove('dragging')
          g.remove()
          frame(() => node.classList.remove('sorting', 'settled'))
        })
      }
      if (reducedMotion()) done()
      else setTimeout(done, SETTLE_MS)
    }

    const onup = () => {
      if (started) {
        // The pointer-up after a drag would otherwise click the item.
        const swallow = (ce: Event) => {
          ce.stopPropagation()
          ce.preventDefault()
        }
        document.addEventListener('click', swallow, { capture: true, once: true })
        setTimeout(() => document.removeEventListener('click', swallow, { capture: true }), 0)
      }
      finish(true)
    }
    const oncancel = () => finish(false)
    const onkey = (ke: KeyboardEvent) => {
      if (ke.key !== 'Escape') return
      ke.preventDefault()
      ke.stopPropagation()
      finish(false)
    }

    cancel = () => finish(false)
    document.addEventListener('pointermove', onmove)
    document.addEventListener('pointerup', onup)
    document.addEventListener('pointercancel', oncancel)
    scroller?.addEventListener('scroll', onscroll)
  }

  node.addEventListener('pointerdown', onpointerdown)
  return {
    update(next: DragSortOptions) {
      options = next
    },
    destroy() {
      node.removeEventListener('pointerdown', onpointerdown)
      cancel?.()
    },
  }
}

const SETTLE_MS = 170

function reducedMotion(): boolean {
  return !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

function frame(fn: () => void) {
  if (typeof window.requestAnimationFrame === 'function') window.requestAnimationFrame(() => fn())
  else setTimeout(fn, 16)
}

function scrollParent(el: HTMLElement): HTMLElement | null {
  for (let p = el.parentElement; p; p = p.parentElement) {
    const o = getComputedStyle(p).overflowY
    if (o === 'auto' || o === 'scroll') return p
  }
  return null
}
