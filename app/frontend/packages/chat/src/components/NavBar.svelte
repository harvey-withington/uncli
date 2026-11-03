<script lang="ts">
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { back, forward, nextBookmark, prevBookmark } from '../lib/nav'
  import Icon from './Icon.svelte'
  import { useShell } from '../lib/shell.svelte'

  const shell = useShell()

  // Page controls floating on the line between the answer and the composer:
  // separate rounded buttons, icons only; each has a label and tooltip.
  const chat = useChat()
  const pages = $derived(chat.currentPages)
  const i = $derived(chat.currentIndex)
  const n = $derived(pages.length)
  const page = $derived(chat.currentPage)

  async function toggle() {
    try {
      await chat.toggleBookmark()
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  async function pin() {
    try {
      await chat.togglePin()
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

<nav class="navbar" aria-label={t('nav.label')}>
  <button onclick={() => chat.goTo(prevBookmark(pages, i))} disabled={n === 0 || i === 0} title={t('nav.prevBookmark')} aria-label={t('nav.prevBookmark')}>
    <Icon name="rewind" fill size={15} />
  </button>
  <button onclick={() => chat.goTo(back(i))} disabled={i === 0} title={t('nav.back')} aria-label={t('nav.back')}>
    <Icon name="play" fill flip size={14} />
  </button>
  <span class="pos" aria-live="polite">
    {#if n > 0}
      <span aria-hidden="true">{i + 1}<span class="of">/</span>{n}</span>
      <span class="visually-hidden">{t('nav.position', { i: i + 1, n })}</span>
    {:else}
      <span aria-hidden="true">–</span>
      <span class="visually-hidden">{t('nav.noPages')}</span>
    {/if}
  </span>
  <button onclick={() => chat.goTo(forward(i, n))} disabled={i >= n - 1} title={t('nav.forward')} aria-label={t('nav.forward')}>
    <Icon name="play" fill size={14} />
  </button>
  <button onclick={() => chat.goTo(nextBookmark(pages, i))} disabled={n === 0 || i >= n - 1} title={t('nav.nextBookmark')} aria-label={t('nav.nextBookmark')}>
    <Icon name="fast-forward" fill size={15} />
  </button>
  <button
    class="bm"
    class:on={page?.bookmarked}
    onclick={toggle}
    disabled={!page}
    aria-pressed={page?.bookmarked ?? false}
    aria-label={t('nav.toggleBookmark')}
    title={t('nav.toggleBookmark')}
  >
    <Icon name="bookmark" fill={page?.bookmarked ?? false} size={15} />
  </button>
  <button
    class="bm"
    class:on={page?.pinned}
    onclick={pin}
    disabled={!page}
    aria-pressed={page?.pinned ?? false}
    aria-label={t('nav.togglePin')}
    title={t('nav.togglePin')}
  >
    <Icon name="pin" fill={page?.pinned ?? false} size={15} />
  </button>
</nav>

<style>
  .navbar {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  button,
  .pos {
    height: 36px;
    border: 1px solid var(--border-strong);
    border-radius: 999px;
    background: linear-gradient(to bottom, var(--surface), var(--surface-2));
    box-shadow: var(--shadow-md);
  }
  button {
    display: grid;
    place-items: center;
    width: 36px;
    padding: 0;
    color: var(--text-muted);
    transition: background var(--fast) var(--ease), color var(--fast) var(--ease), border-color var(--fast) var(--ease), transform var(--fast) var(--ease);
  }
  button:hover:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
    background: linear-gradient(to bottom, var(--surface), var(--accent-soft));
  }
  button:active:not(:disabled) {
    transform: scale(0.94);
  }
  button:disabled {
    color: var(--text-faint);
    cursor: default;
    box-shadow: var(--shadow-sm);
  }
  button:disabled :global(.icon) {
    opacity: 0.45;
  }
  .bm {
    margin-left: 6px;
  }
  .bm.on {
    color: var(--accent);
  }
  .pos {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 64px;
    padding: 0 12px;
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text);
    font-variant-numeric: tabular-nums;
  }
  .of {
    margin: 0 3px;
    color: var(--text-faint);
    font-weight: 400;
  }
</style>
