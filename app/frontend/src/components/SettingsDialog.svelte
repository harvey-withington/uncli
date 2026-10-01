<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'

  // Phase 1: the CLI version setting. Running anything but the pinned
  // version is at the user's own risk.
  const app = useApp()
  let version = $state(app.cli?.version ?? '')
  let channels = $state<Record<string, string>>({})
  let busy = $state(false)

  $effect(() => {
    app.backend.cliChannels().then(c => (channels = c)).catch(() => {})
  })

  const pinned = $derived(app.cli?.pinned ?? '')
  const isPinned = $derived(!version || version === pinned)

  async function apply(v: string) {
    busy = true
    try {
      app.cli = await app.backend.setCLIVersion(v)
      version = app.cli.version
      showToast(t('settings.switched', { version: app.cli.version }), 'success')
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
</script>

<Modal title={t('settings.title')} width={480} onclose={() => (app.settingsOpen = false)}>
  <section>
    <h3>{t('settings.cliTitle')}</h3>
    <p class="muted">{t('settings.cliBody', { pinned })}</p>
    {#if app.cli?.custom}
      <p class="note">{t('settings.custom')}</p>
    {/if}
    <div class="row">
      <input class="input" bind:value={version} aria-label={t('settings.version')} placeholder={pinned} />
      <button class="btn" onclick={() => apply(version)} disabled={busy || !version.trim()}>
        {#if busy}<Icon name="loader" spin size={14} />{/if}{t('settings.use')}
      </button>
    </div>
    <div class="channels">
      {#each Object.entries(channels) as [name, v] (name)}
        <button class="btn small ghost" onclick={() => (version = v)}>{t('settings.channel', { name, version: v })}</button>
      {/each}
    </div>
    {#if !isPinned}
      <p class="warn" role="note"><Icon name="triangle-alert" size={14} />{t('settings.risk')}</p>
    {/if}
    <button class="btn" onclick={() => apply('')} disabled={busy || app.cli?.version === pinned}>{t('settings.backToPinned', { pinned })}</button>
  </section>
  {#if app.cli?.email}
    <section>
      <h3>{t('settings.account')}</h3>
      <p class="muted">{t('settings.signedIn', { email: app.cli.email, plan: app.cli.subscription ?? '' })}</p>
    </section>
  {/if}
</Modal>

<style>
  section + section {
    margin-top: var(--space-5);
    padding-top: var(--space-4);
    border-top: 1px solid var(--border);
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
  .note {
    font-size: var(--text-sm);
    color: var(--warning);
  }
  .row {
    display: flex;
    gap: var(--space-2);
  }
  .row .input {
    flex: 1;
    font-family: var(--mono);
  }
  .channels {
    display: flex;
    gap: var(--space-1);
    margin: var(--space-2) 0 var(--space-3);
  }
  .warn {
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--warning-soft);
    color: var(--warning);
    font-size: var(--text-sm);
  }
</style>
