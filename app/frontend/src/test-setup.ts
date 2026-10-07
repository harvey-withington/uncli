import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/svelte'
import { afterEach } from 'vitest'

afterEach(() => cleanup())

// jsdom has no Web Animations API; Svelte transitions call element.animate.
if (typeof Element !== 'undefined' && !Element.prototype.animate) {
  Element.prototype.animate = function (): Animation {
    return {
      cancel() {}, finish() {}, play() {}, pause() {}, reverse() {},
      addEventListener() {}, removeEventListener() {},
      onfinish: null, oncancel: null, onremove: null, playState: 'finished',
      finished: Promise.resolve({} as Animation), ready: Promise.resolve({} as Animation),
    } as unknown as Animation
  }
}

// jsdom has no PointerEvent; a MouseEvent carries what the app reads
// (button, clientX/Y).
if (typeof window !== 'undefined' && !('PointerEvent' in window)) {
  class PointerEventPolyfill extends MouseEvent {
    pointerId: number
    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init)
      this.pointerId = init.pointerId ?? 1
    }
  }
  ;(window as unknown as { PointerEvent: typeof MouseEvent }).PointerEvent = PointerEventPolyfill
}

// jsdom has no ResizeObserver; Svelte's bind:clientWidth uses it. Nothing
// is ever resized in a test, so it never calls back.
if (typeof window !== 'undefined' && !('ResizeObserver' in window)) {
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  ;(window as unknown as { ResizeObserver: typeof ResizeObserverStub }).ResizeObserver = ResizeObserverStub
}
