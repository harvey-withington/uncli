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
