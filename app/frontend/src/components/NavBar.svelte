<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { back, forward, nextBookmark, prevBookmark } from '../lib/nav'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  const app = useApp()
  const pages = $derived(app.currentPages)
  const i = $derived(app.currentIndex)
  const n = $derived(pages.length)
  const page = $derived(app.currentPage)

  async function toggle() {
    try {
      await app.toggleBookmark()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

<nav class="navbar" aria-label={t('nav.label')}>
  <button class="btn ghost icon" onclick={() => app.goTo(prevBookmark(pages, i))} disabled={n === 0 || i === 0} title={t('nav.prevBookmark')} aria-label={t('nav.prevBookmark')}>
    <Icon name="chevrons-left" />
  </button>
  <button class="btn ghost icon" onclick={() => app.goTo(back(i))} disabled={i === 0} title={t('nav.back')} aria-label={t('nav.back')}>
    <Icon name="chevron-left" />
  </button>
  <span class="pos" aria-live="polite">
    {#if n > 0}{t('nav.position', { i: i + 1, n })}{:else}{t('nav.noPages')}{/if}
  </span>
  <button class="btn ghost icon" onclick={() => app.goTo(forward(i, n))} disabled={i >= n - 1} title={t('nav.forward')} aria-label={t('nav.forward')}>
    <Icon name="chevron-right" />
  </button>
  <button class="btn ghost icon" onclick={() => app.goTo(nextBookmark(pages, i))} disabled={n === 0 || i >= n - 1} title={t('nav.nextBookmark')} aria-label={t('nav.nextBookmark')}>
    <Icon name="chevrons-right" />
  </button>
  <span class="sep" aria-hidden="true"></span>
  <button
    class="btn ghost bm"
    class:on={page?.bookmarked}
    onclick={toggle}
    disabled={!page}
    aria-pressed={page?.bookmarked ?? false}
    aria-label={t('nav.toggleBookmark')}
    title={t('nav.toggleBookmark')}
  >
    <Icon name={page?.bookmarked ? 'bookmark-check' : 'bookmark'} />
    {page?.bookmarked ? t('nav.bookmarked') : t('nav.bookmark')}
    <kbd>B</kbd>
  </button>
</nav>

<style>
  .navbar {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .pos {
    min-width: 92px;
    text-align: center;
    font-size: var(--text-sm);
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .sep {
    width: 1px;
    height: 18px;
    margin: 0 var(--space-2);
    background: var(--border);
  }
  .bm kbd {
    font-family: var(--font);
    font-size: 10.5px;
    padding: 0 5px;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-faint);
  }
  .bm.on {
    color: var(--accent);
  }
</style>
