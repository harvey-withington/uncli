<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'
  import type { Preferences } from '../lib/api'
  import { modelOptions } from '../lib/models'
  import { AUTO_SUMMARY_MIN_BLOCKS } from '../lib/outline'

  // Settings: the quick-task model, and the CLI version (anything but the
  // pinned version is at the user's own risk).
  const app = useApp()
  let version = $state(app.cli?.version ?? '')
  let channels = $state<Record<string, string>>({})
  let busy = $state(false)

  $effect(() => {
    app.backend.cliChannels().then(c => (channels = c)).catch(() => {})
  })

  const pinned = $derived(app.cli?.pinned ?? '')

  // Quick tasks: one provider and model for small jobs outside sessions.
  const prefs = $derived(app.boot?.preferences)
  const quickModels = $derived(modelOptions(app.boot?.models ?? null, prefs?.quickTaskModel.model))

  async function savePrefs(next: Preferences) {
    try {
      const saved = await app.backend.setPreferences(next)
      if (app.boot) app.boot.preferences = saved
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  function setQuick(field: 'provider' | 'model', value: string) {
    if (!prefs) return
    savePrefs({ ...prefs, quickTaskModel: { ...prefs.quickTaskModel, [field]: value } })
  }
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
  {#if prefs}
    <section>
      <h3>{t('settings.quickTitle')}</h3>
      <p class="muted">{t('settings.quickBody')}</p>
      <div class="grid">
        <label for="quick-provider">{t('settings.quickProvider')}</label>
        <select id="quick-provider" class="select" value={prefs.quickTaskModel.provider} onchange={e => setQuick('provider', e.currentTarget.value)}>
          {#each app.boot?.providers ?? [] as p (p.id)}
            <option value={p.id}>{p.label}</option>
          {/each}
        </select>
        <label for="quick-model">{t('settings.quickModel')}</label>
        <select id="quick-model" class="select" value={prefs.quickTaskModel.model} onchange={e => setQuick('model', e.currentTarget.value)}>
          {#each quickModels as m (m.value)}
            <option value={m.value}>{m.label}</option>
          {/each}
        </select>
      </div>
      <label class="check">
        <input type="checkbox" checked={prefs.autoSummarise} onchange={e => savePrefs({ ...prefs, autoSummarise: e.currentTarget.checked })} />
        <span>
          {t('settings.autoSummarise')}
          <span class="hint">{t('settings.autoSummariseHint', { n: AUTO_SUMMARY_MIN_BLOCKS })}</span>
        </span>
      </label>
    </section>
  {/if}
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
  .grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-3);
    align-items: center;
    margin-bottom: var(--space-3);
    font-size: var(--text-sm);
  }
  .check {
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
    font-size: var(--text-sm);
    cursor: pointer;
  }
  .check input {
    margin-top: 3px;
  }
  .hint {
    display: block;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
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
