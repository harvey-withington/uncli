<script lang="ts">
  import { onDestroy } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import { useApp } from '../lib/context'
  import { relativeTime } from '../lib/format'
  import { t } from '../lib/i18n.svelte'

  // When the server can't be reached: which, why, when it last connected,
  // and that it keeps trying (lib/api/http.ts reconnects by itself). A
  // server on another protocol says so too, and one that has been updated
  // since this page loaded offers Reload, for the page to match it.

  interface Props {
    server: string
    mismatch?: boolean // the server speaks another protocol version
    updated?: boolean // the server runs another build than when this page loaded
  }

  let { server, mismatch = false, updated = false }: Props = $props()
  const app = useApp()
  const c = $derived(app.connection)
  let now = $state(Date.now())
  const timer = setInterval(() => (now = Date.now()), 15_000)
  onDestroy(() => clearInterval(timer))
  const down = $derived(!!c && c.state !== 'online' && (c.state === 'offline' || c.lastOnline !== null))
</script>

{#if mismatch}
  <p class="banner warn" role="alert"><Icon name="triangle-alert" size={15} />{t('mobile.versionMismatch', { server })}</p>
{/if}
{#if updated}
  <p class="banner info" role="status">
    <Icon name="circle-arrow-up" size={15} />
    <span>{t('mobile.updated')}</span>
    <button class="btn primary small" onclick={() => location.reload()}>{t('mobile.reload')}</button>
  </p>
{/if}
{#if down && c}
  <p class="banner" role="status">
    <Icon name="wifi-off" size={15} />
    <span>
      {t('mobile.offline', { server })}
      {#if c.error}<span class="why">({c.error})</span>{/if}
      {c.lastOnline ? t('mobile.lastOnline', { when: relativeTime(c.lastOnline, now) }) : ''}
      {t('mobile.retrying')}
    </span>
  </p>
{/if}

<style>
  .banner {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-2) var(--space-4);
    font-size: var(--text-sm);
    background: var(--warning-soft);
    color: var(--text);
    border-bottom: 1px solid var(--border);
  }
  .banner :global(svg) {
    flex: none;
    margin-top: 2px;
    color: var(--warning);
  }
  .banner.info {
    align-items: center;
    background: var(--accent-soft);
  }
  .banner.info :global(svg) {
    margin-top: 0;
    color: var(--accent);
  }
  .banner.info span {
    flex: 1;
  }
  .why {
    color: var(--text-muted);
  }
</style>
