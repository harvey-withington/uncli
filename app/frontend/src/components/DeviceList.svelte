<script lang="ts">
  import { onMount } from 'svelte'
  import type { PairedDevice } from '../lib/api'
  import { confirm } from '../lib/confirm.svelte'
  import { useApp } from '../lib/context'
  import { relativeTime } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // Settings → Remote access → Paired devices (decision 0022): each device
  // with when it paired and was last seen, and Revoke, which shuts it out
  // at once. Empty, it says how to pair one.
  const app = useApp()

  onMount(async () => {
    try {
      app.devices = await app.backend.pairedDevices()
    } catch (e) {
      showToast(String(e), 'error')
    }
  })

  async function revoke(d: PairedDevice) {
    const ok = await confirm({ title: t('devices.revokeTitle', { name: d.name }), message: t('devices.revokeBody'), confirmLabel: t('devices.revoke'), danger: true })
    if (!ok) return
    try {
      await app.backend.revokeDevice(d.id)
      app.devices = app.devices.filter(x => x.id !== d.id)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

<h4>{t('devices.title')}</h4>
{#if app.devices.length === 0}
  <p class="empty">{t('devices.none')}</p>
{:else}
  <ul aria-label={t('devices.title')}>
    {#each app.devices as d (d.id)}
      <li>
        <Icon name="smartphone" size={16} />
        <div class="who">
          <strong>{d.name}</strong>
          <span>{t('devices.seen', { paired: new Date(d.paired).toLocaleDateString(), seen: relativeTime(Date.parse(d.lastSeen)) })}</span>
        </div>
        <button class="btn small" onclick={() => revoke(d)}>{t('devices.revoke')}</button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  h4 {
    margin: var(--space-4) 0 var(--space-2);
    font-size: var(--text-sm);
  }
  .empty {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    color: var(--text-muted);
  }
  li + li {
    border-top: 1px solid var(--border);
  }
  .who {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    font-size: var(--text-sm);
  }
  .who strong {
    color: var(--text);
    font-weight: 500;
  }
  .who span {
    font-size: var(--text-xs);
  }
</style>
