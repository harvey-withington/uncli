// The walkthrough deck's own script (decision 0029), after reveal.js's in
// the same file (deck-player.js, made by build/deck-player.ts). It runs in
// the deck's sandboxed frame, loaded as a file from the app's origin: a
// page's content security policy, which the frame inherits, may refuse
// inline scripts (the phone app's does).
//
// Escape and F go to the dialog (it closes, or leaves full screen) instead
// of reveal.js's overview and full screen; O still opens the overview. The
// slides are landscape in a wide frame and portrait in a tall one (a
// phone), switching as the frame turns. Code too wide for its slide scrolls
// sideways under a finger instead of swiping to the next slide.
;(() => {
  const tell = what => parent.postMessage({ 'uncli-deck': what }, '*')
  const shape = () => (innerHeight > innerWidth * 1.1 ? { width: 460, height: 780 } : { width: 1200, height: 720 })
  let now = shape()
  const deck = new Reveal({
    hash: false, history: false, controls: true, progress: true, center: false, transition: 'slide',
    ...now, margin: 0.05, minScale: 0.2, maxScale: 1.4,
    keyboard: { 27: () => tell('escape'), 70: () => tell('fullscreen') },
  })
  const wide = () => document.querySelectorAll('pre.shiki').forEach(p => p.toggleAttribute('data-prevent-swipe', p.scrollWidth > p.clientWidth + 1))
  deck.initialize().then(() => {
    wide()
    deck.on('slidechanged', wide)
    addEventListener('resize', () => {
      const next = shape()
      if (next.width !== now.width) {
        now = next
        deck.configure(next)
      }
      wide()
    })
  })
})()
