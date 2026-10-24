<script lang="ts">
  import { onMount } from 'svelte'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { remoteLine, validPort } from '../lib/remote'
  import { showToast } from '../lib/toasts.svelte'
  import CopyButton from './CopyButton.svelte'
  import DeviceList from './DeviceList.svelte'
  import Icon from './Icon.svelte'
  import PairDialog from './PairDialog.svelte'

  // Settings → Remote access (decision 0021): serve this PC's sessions to
  // paired devices over Tailscale. The switch, the port, and the state: the
  // address to open, or why there isn't one.
  const app = useApp()
  const status = $derived(app.remote)
  const line = $derived(status ? remoteLine(status) : null)
  let port = $state('')
  let busy = $state(false)
  let pairing = $state(false)
  const canPair = $derived(status?.state === 'on')
  const portNumber = $derived(validPort(port))

  onMount(async () => {
    try {
      app.remote = await app.backend.remoteStatus()
      port = String(app.remote.port)
    } catch (e) {
      showToast(String(e), 'error')
    }
  })

  async function apply(on: boolean) {
    if (!portNumber) return
    busy = true
    try {
      app.remote = await app.backend.setRemoteAccess(on, portNumber)
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
</script>

<section id="settings-remote">
  <h3>{t('remote.title')}</h3>
  <p class="muted">{t('remote.body')}</p>
  <div class="row">
    <label class="check">
      <input type="checkbox" checked={!!status?.enabled} disabled={!status || busy || !portNumber} onchange={e => apply(e.currentTarget.checked)} />
      {t('remote.enable')}
    </label>
    <label class="port">
      {t('remote.port')}
      <input class="input" inputmode="numeric" bind:value={port} aria-invalid={!portNumber} disabled={busy} />
    </label>
    {#if status?.enabled && portNumber && portNumber !== status.port}
      <button class="btn small" onclick={() => apply(true)} disabled={busy}>{t('remote.applyPort')}</button>
    {/if}
  </div>
  {#if !portNumber}<p class="note danger">{t('remote.badPort')}</p>{/if}
  {#if status && line}
    <p class="state {line.tone}" role="status">
      {#if busy}<Icon name="loader" spin size={14} />{:else}<span class="dot" aria-hidden="true"></span>{/if}
      <span>{t(line.key, line.params)}</span>
    </p>
    {#if status.url}
      <div class="address">
        <code>{status.url}</code>
        <CopyButton text={status.url} label={t('remote.copyAddress')} />
      </div>
    {/if}
    {#if status.loopback}<p class="note">{t('remote.loopback', { loopback: status.loopback })}</p>{/if}
  {/if}
  <div class="pair">
    <button class="btn" onclick={() => (pairing = true)} disabled={!canPair}><Icon name="qr-code" size={14} />{t('pair.start')}</button>
    {#if !canPair}<span class="note">{t('pair.needsOn')}</span>{/if}
  </div>
  <DeviceList />
</section>
{#if pairing}<PairDialog onclose={() => (pairing = false)} />{/if}

<style>
  section {
    margin-top: var(--space-3);
  }
  h3 {
    margin: 0 0 var(--space-1);
    font-size: var(--text-md);
  }
  .muted {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
  }
  .check,
  .port {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }
  .port .input {
    width: 10ch; /* five digits and the field's padding */
  }
  .state {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin: var(--space-3) 0 0;
    font-size: var(--text-sm);
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    margin-top: 0.45em;
    border-radius: 50%;
    background: var(--text-faint);
  }
  .state.ok .dot {
    background: var(--success);
  }
  .state.warn .dot {
    background: var(--warning);
  }
  .state.danger .dot {
    background: var(--danger);
  }
  .address {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
  .address code {
    font-size: var(--text-sm);
    user-select: all;
  }
  .note {
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .pair {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    margin-top: var(--space-4);
  }
  .pair .note {
    margin: 0;
  }
  .note.danger {
    color: var(--danger);
  }
</style>
