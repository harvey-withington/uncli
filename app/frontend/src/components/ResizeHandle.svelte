<script lang="ts">
  // A drag handle on one edge of a panel. Drag it, or focus it and use the
  // arrow keys (Shift for bigger steps). edge is the panel side it sits on:
  // on a right edge, dragging right widens; on a left edge, dragging left does.
  interface Props {
    edge: 'left' | 'right'
    width: number
    min: number
    max: number
    label: string
    onresize: (width: number) => void // while dragging
    oncommit: (width: number) => void // when settled: persist it
  }

  let { edge, width, min, max, label, onresize, oncommit }: Props = $props()
  let dragging = $state(false)

  const clamp = (w: number) => Math.round(Math.min(max, Math.max(min, w)))
  const sign = $derived(edge === 'right' ? 1 : -1)

  function onpointerdown(e: PointerEvent) {
    const handle = e.currentTarget as HTMLElement
    handle.setPointerCapture(e.pointerId)
    const startX = e.clientX
    const startW = width
    let w = width
    dragging = true
    document.body.classList.add('resizing')
    const move = (ev: PointerEvent) => {
      w = clamp(startW + sign * (ev.clientX - startX))
      onresize(w)
    }
    const up = () => {
      dragging = false
      document.body.classList.remove('resizing')
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
      handle.removeEventListener('pointercancel', up)
      oncommit(w)
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
    handle.addEventListener('pointercancel', up)
  }

  function onkeydown(e: KeyboardEvent) {
    const step = e.shiftKey ? 48 : 16
    let w: number
    if (e.key === 'ArrowRight') w = clamp(width + sign * step)
    else if (e.key === 'ArrowLeft') w = clamp(width - sign * step)
    else return
    e.preventDefault()
    onresize(w)
    oncommit(w)
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="handle {edge}"
  class:dragging
  role="separator"
  aria-orientation="vertical"
  aria-label={label}
  aria-valuenow={width}
  aria-valuemin={min}
  aria-valuemax={max}
  tabindex="0"
  {onpointerdown}
  {onkeydown}
></div>

<style>
  .handle {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 6px;
    cursor: col-resize;
    z-index: 5;
    outline: none;
    transition: background var(--fast) var(--ease);
  }
  .left {
    left: -3px;
  }
  .right {
    right: -3px;
  }
  .handle:hover,
  .handle:focus-visible,
  .dragging {
    background: var(--accent);
  }
  /* No text selection or I-beam flicker while dragging. */
  :global(body.resizing) {
    cursor: col-resize;
    user-select: none;
  }
</style>
