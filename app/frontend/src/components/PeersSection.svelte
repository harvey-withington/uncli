<script lang="ts">
  import { useShell } from '@uncli/chat/lib/shell.svelte'
  import { onMount } from 'svelte'
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { parsePairLink } from '@uncli/chat/lib/api'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import PeerList from './PeerList.svelte'

  // Settings → Remote servers (decision 0031): pair this PC with another
  // UNCLI, as a phone pairs, to see its sessions in the sidebar. The link
  // comes from that PC's Settings → Remote access → Pair a device; Allow
  // is clicked there.
  const app = useApp()
  const shell = useShell()
  let link = $state('')
  let name = $state('')
  let waiting = $state<string | null>(null) // the server waiting for Allow
  let error = $state('')
  const parsed = $derived(parsePairLink(link))

  onMount(async () => {
    try {
      name = await app.backend.thisPCName()
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  })

  async function pair() {
    if (!parsed) return
    error = ''
    waiting = new URL(parsed.base).hostname.split('.')[0] ?? parsed.base
    try {
      const p = await app.backend.addRemoteServer(link.trim(), name.trim())
      link = ''
      shell.toast(t('peers.paired', { name: p.name }), 'success')
      await app.loadServers()
    } catch (e) {
      if (waiting) error = e instanceof Error ? e.message : String(e)
    } finally {
      waiting = null
    }
  }

  async function cancel() {
    waiting = null
    try {
      await app.backend.cancelAddRemoteServer()
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

<section id="settings-peers">
  <h3>{t('peers.title')}</h3>
  <p class="muted">{t('peers.body')}</p>
  <div class="form">
    <label>
      {t('peers.link')}
      <input class="input" bind:value={link} placeholder="https://…ts.net:9871/m/#pair=…" aria-invalid={!!link.trim() && !parsed} disabled={!!waiting} />
    </label>
    <label>
      {t('peers.name')}
      <input class="input" bind:value={name} disabled={!!waiting} />
    </label>
  </div>
  {#if link.trim() && !parsed}<p class="note danger">{t('peers.badLink')}</p>{/if}
  <div class="actions">
    {#if waiting}
      <p class="note wait" role="status"><Icon name="loader" spin size={14} />{t('peers.waiting', { name: waiting })}</p>
      <button class="btn small" onclick={cancel}>{t('common.cancel')}</button>
    {:else}
      <button class="btn" onclick={pair} disabled={!parsed || !name.trim()}><Icon name="server" size={14} />{t('peers.pair')}</button>
    {/if}
  </div>
  {#if error}<p class="note danger" role="alert">{error}</p>{/if}
  <PeerList />
</section>

<style>
  section {
    margin-top: var(--space-6);
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
  .form {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: var(--space-3);
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--text-sm);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-top: var(--space-3);
  }
  .note {
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .note.wait {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-sm);
  }
  .note.danger {
    color: var(--danger);
  }
</style>
