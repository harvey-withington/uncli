<script lang="ts">
  import { untrack } from 'svelte'
  import { t } from '../lib/i18n.svelte'
  import { loadFollow, saveFollow } from '../lib/keep'
  import Icon from './Icon.svelte'

  // Where the page scroller sits, and the toggle under its scrollbar that
  // chooses how it moves while an answer is written: stay where the viewer
  // left it (the default), or follow the end. Updates to the page never
  // move it; another page opens where it was left (the top the first
  // time), or at the end when following.
  interface Props {
    scroller: HTMLElement | undefined
    pageId: string | undefined
  }

  let { scroller, pageId }: Props = $props()
  let follow = $state(loadFollow())
  let width = $state(16) // the scrollbar's, so the toggle sits right under it
  const left = new Map<string, number>() // where each page was left, this run
  let shown = '' // the page the scroller shows

  const frame = (f: () => void) => (window.requestAnimationFrame ?? ((g: FrameRequestCallback) => window.setTimeout(g, 16)))(f)

  // Where the page is left, as it's scrolled.
  $effect(() => {
    const el = scroller
    if (!el) return
    const keep = () => {
      if (shown) left.set(shown, el.scrollTop)
    }
    el.addEventListener('scroll', keep, { passive: true })
    return () => el.removeEventListener('scroll', keep)
  })

  // Another page: where it was left, or the end when following.
  $effect(() => {
    const id = pageId
    const el = scroller
    if (!el || !id || id === shown) return
    untrack(() => {
      shown = id
      const to = follow ? Infinity : (left.get(id) ?? 0)
      frame(() => {
        if (shown === id) el.scrollTop = to === Infinity ? el.scrollHeight : to
      })
    })
  })

  // Following: whenever the page grows, show its end.
  $effect(() => {
    const el = scroller
    const content = el?.firstElementChild
    if (!el || !content || !follow) return
    const toEnd = () => {
      el.scrollTop = el.scrollHeight
    }
    toEnd()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(toEnd)
    ro.observe(content)
    return () => ro.disconnect()
  })

  $effect(() => {
    const el = scroller
    if (!el || typeof ResizeObserver === 'undefined') return
    const measure = () => {
      width = el.offsetWidth - el.clientWidth || 16
    }
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  })

  function toggle() {
    follow = !follow
    saveFollow(follow)
  }
</script>

<div class="foot">
  <button
    class="follow"
    class:on={follow}
    style:width="{width}px"
    onclick={toggle}
    aria-pressed={follow}
    aria-label={t('scroll.follow')}
    title={t(follow ? 'scroll.followingHint' : 'scroll.stayingHint')}
  >
    <Icon name="arrow-down-to-line" size={12} />
  </button>
</div>

<style>
  /* A strip under the page scroller, so the toggle sits below the
     scrollbar's own arrows rather than over them. */
  .foot {
    flex: none;
    display: flex;
    justify-content: flex-end;
    height: 16px;
  }
  .follow {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 16px;
    padding: 0;
    border: 0;
    border-radius: 2px;
    background: none;
    color: var(--text-faint);
    cursor: pointer;
  }
  .follow:hover {
    color: var(--text);
    background: var(--surface-2);
  }
  .follow.on {
    color: var(--accent);
    background: var(--accent-soft);
  }
  .follow:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
</style>
