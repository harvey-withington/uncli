<script lang="ts">
  import { untrack } from 'svelte'
  import { t } from '../lib/i18n.svelte'
  import { loadFollow, saveFollow } from '../lib/keep'
  import { keepScroll } from '../lib/scrollmemory'
  import Icon from './Icon.svelte'

  // Where the page scroller sits, and the toggle under its scrollbar that
  // chooses how it moves while an answer is written: stay where the viewer
  // left it (the default), or follow the end. Updates to the page never
  // move it; another page opens where it was left (the top the first
  // time, kept across restarts by lib/scrollmemory.ts), or at the end when
  // following.
  interface Props {
    scroller: HTMLElement | undefined
    pageId: string | undefined
  }

  let { scroller, pageId }: Props = $props()
  let follow = $state(loadFollow())
  let width = $state(16) // the scrollbar's, so the toggle sits right under it
  const options = () => ({ key: pageId ? `page:${pageId}` : '', follow })
  let memory: ReturnType<typeof keepScroll> | null = null

  $effect(() => {
    const el = scroller
    if (!el) return
    const m = keepScroll(el, untrack(options))
    memory = m
    return () => {
      m.destroy()
      memory = null
    }
  })
  $effect(() => {
    const o = options()
    untrack(() => memory?.update(o))
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
