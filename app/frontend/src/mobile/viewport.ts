// The phone app fills what's visible of the page, above the on-screen
// keyboard. Chrome on Android shrinks the page for the keyboard itself
// (interactive-widget=resizes-content in m/index.html); Safari doesn't: it
// keeps the page's height and scrolls it up to show the focused box, which
// takes the session's header off the top. Following the visual viewport
// does for both: the app is as tall as what shows, and moved to where it
// shows. The transform also makes the app the box its dialogs and toasts
// are fixed to, so they stay above the keyboard too.

export interface VisualViewportLike extends EventTarget {
  height: number
  offsetTop: number
}

export function placeIn(el: HTMLElement, vv: VisualViewportLike) {
  el.style.height = `${vv.height}px`
  el.style.transform = `translateY(${vv.offsetTop}px)`
}

// followViewport keeps el fitted to the visual viewport; it returns the stop.
export function followViewport(el: HTMLElement, vv: VisualViewportLike | null = window.visualViewport) {
  if (!vv) return () => {}
  Object.assign(el.style, { position: 'fixed', top: '0', left: '0', right: '0' })
  const place = () => placeIn(el, vv)
  place()
  vv.addEventListener('resize', place)
  vv.addEventListener('scroll', place)
  return () => {
    vv.removeEventListener('resize', place)
    vv.removeEventListener('scroll', place)
  }
}
