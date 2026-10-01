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
