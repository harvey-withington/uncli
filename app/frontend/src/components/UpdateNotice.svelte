<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // A newer UNCLI release (decision 0025), above the sidebar's footer on
  // the PC and atop the session list on the phone: Update now, and what's
  // new on its release page. Then how far it's got: downloading, handed to
  // the installer (UNCLI restarts in a moment), or why it failed. Nothing
  // while there's no newer release.
  const app = useApp()
  const u = $derived(app.update)
  const version = $derived(u?.latest?.version ?? '')
  const shown = $derived(!!u && (u.available || u.state === 'downloading' || u.state === 'installing'))
</script>

{#if u && shown}
  <div class="update" class:failed={u.state === 'error'} role="status">
    {#if u.state === 'downloading'}
      <p><Icon name="loader" spin size={14} />{t('update.downloading', { version, percent: u.percent ?? 0 })}</p>
    {:else if u.state === 'installing'}
      <p><Icon name="loader" spin size={14} />{t('update.installing', { version })}</p>
    {:else}
      <p><Icon name={u.state === 'error' ? 'triangle-alert' : 'circle-arrow-up'} size={14} />{u.state === 'error' ? t('update.failed', { error: u.error ?? '' }) : t('update.available', { version })}</p>
      <div class="acts">
        <button class="btn primary small" onclick={() => app.installUpdate()}>{t(u.state === 'error' ? 'update.retry' : 'update.now')}</button>
        {#if u.latest?.url}
          <button class="btn ghost small" onclick={() => app.backend.openURL(u.latest?.url ?? '')}>{t('update.whatsNew')}</button>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .update {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--accent);
    border-radius: var(--radius);
    background: var(--accent-soft);
    font-size: var(--text-sm);
    margin-bottom: var(--space-3);
  }
  .update.failed {
    border-color: var(--danger);
    background: var(--surface-2);
  }
  p {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin: 0;
  }
  p :global(svg) {
    flex: none;
    margin-top: 2px;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }
</style>
