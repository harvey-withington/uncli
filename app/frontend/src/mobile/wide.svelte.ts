// Whether the phone app has a tablet's width: 900 px or more (landscape on
// most tablets, portrait on large ones). Wide, the session list sits beside
// the open session and the side panel beside the answer; narrower, the
// phone's one-screen-at-a-time layout is unchanged. Off where the browser
// can't say (tests).
export const WIDE_QUERY = '(min-width: 900px)'

export const wide = $state({ on: false })

const mq = typeof globalThis.matchMedia === 'function' ? globalThis.matchMedia(WIDE_QUERY) : null
if (mq) {
  wide.on = mq.matches
  mq.addEventListener?.('change', e => (wide.on = e.matches))
}
