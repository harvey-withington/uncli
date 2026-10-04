<script lang="ts">
  import { untrack } from 'svelte'
  import type { DecisionModel, DecisionTest, Preferences } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // Settings → "Decision model": what answers "is this safe?" for what
  // UNCLI can't place. The quick-task model by default; or a System One
  // server (Kev on this computer or the network, or hosted Jev) with its
  // address, model, pinned version, how sure "safe" must be, local only
  // and an API key, which is saved apart and never shown again. "Test"
  // asks about a sample command and shows the answer, how sure, and how
  // long it took.
  interface Props {
    prefs: Preferences
    savePrefs: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  const current = $derived<DecisionModel>(prefs.decisionModel ?? { provider: 'quick-task' })

  // The System One fields, edited here and saved together.
  let draft = $state<DecisionModel>({ provider: 'systemone' })
  let provider = $state<DecisionModel['provider']>('quick-task')
  let key = $state('')
  let keySaved = $state(false)
  let result = $state<DecisionTest | null>(null)
  let testing = $state(false)

  $effect(() => {
    const c = current
    untrack(() => {
      provider = c.provider
      if (c.provider === 'systemone') draft = { ...c }
    })
  })
  $effect(() => {
    app.backend.hasDecisionKey().then(v => (keySaved = v), () => {})
  })

  async function choose(p: DecisionModel['provider']) {
    provider = p
    result = null
    if (p === 'quick-task') await savePrefs({ ...prefs, decisionModel: { provider: 'quick-task' } })
  }

  async function save() {
    result = null
    await savePrefs({ ...prefs, decisionModel: { ...draft, provider: 'systemone', threshold: Number(draft.threshold) || undefined } })
  }

  async function saveKey() {
    try {
      await app.backend.setDecisionKey(key)
      keySaved = key.trim() !== ''
      key = ''
      showToast(t(keySaved ? 'decider.keySaved' : 'decider.keyRemoved'), 'success')
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function test() {
    testing = true
    result = null
    try {
      result = await app.backend.testDecisionModel()
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      testing = false
    }
  }

  const unsaved = $derived(provider === 'systemone' && JSON.stringify({ ...draft, provider: 'systemone' }) !== JSON.stringify(current))
</script>

<section id="settings-decider">
  <h3>{t('decider.title')}</h3>
  <p class="muted">{t('decider.body')}</p>
  <select class="select wide" aria-label={t('decider.use')} value={provider} onchange={e => choose(e.currentTarget.value as DecisionModel['provider'])}>
    <option value="quick-task">{t('decider.quickTask')}</option>
    <option value="systemone">{t('decider.systemOne')}</option>
  </select>

  {#if provider === 'systemone'}
    <form class="grid" onsubmit={e => { e.preventDefault(); void save() }}>
      <label for="decider-endpoint">{t('decider.endpoint')}</label>
      <input id="decider-endpoint" class="input" bind:value={draft.endpoint} placeholder="http://localhost:8009" />
      <label for="decider-model">{t('decider.model')}</label>
      <input id="decider-model" class="input" bind:value={draft.model} placeholder="kev-latest" />
      <label for="decider-version">{t('decider.version')}</label>
      <input id="decider-version" class="input" bind:value={draft.version} placeholder={t('decider.versionHint')} />
      <label for="decider-threshold">{t('decider.threshold')}</label>
      <input id="decider-threshold" class="input" type="number" min="0.5" max="1" step="0.01" bind:value={draft.threshold} placeholder="0.9" />
      <span></span>
      <label class="check"><input type="checkbox" bind:checked={draft.localOnly} />{t('decider.localOnly')}</label>
      <span></span>
      <button class="btn small" type="submit" disabled={!unsaved}>{t('decider.save')}</button>
    </form>
    <form class="row" onsubmit={e => { e.preventDefault(); void saveKey() }}>
      <input class="input" type="password" autocomplete="off" bind:value={key} aria-label={t('decider.key')} placeholder={keySaved ? t('decider.keyIsSaved') : t('decider.keyNone')} />
      <button class="btn small" type="submit">{t(key.trim() || !keySaved ? 'decider.saveKey' : 'decider.removeKey')}</button>
    </form>
  {/if}

  <div class="row test">
    <button class="btn small" onclick={test} disabled={testing || unsaved}>
      {#if testing}<Icon name="loader" spin size={13} />{:else}<Icon name="play" size={13} />{/if}{t('decider.test')}
    </button>
    {#if result}
      <p class="result" role="status" aria-label={t('decider.result')}>
        <code>{result.command}</code>
        {t(`decider.level.${result.verdict}`)}
        {#if result.calibrated}({t('decider.sure', { n: Math.round(result.confidence * 100) })}){/if}
        · {t('decider.millis', { n: result.millis })} · <span class="who">{result.decider.replace(/@$/, '')}</span>
        {#if result.reason}<br />{result.reason}{/if}
      </p>
    {/if}
  </div>
</section>

<style>
  section {
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
  .wide {
    width: 100%;
  }
  .grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-3);
    align-items: center;
    margin-top: var(--space-3);
    font-size: var(--text-sm);
  }
  .grid .btn {
    justify-self: start;
  }
  .check {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }
  .row {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .row .input {
    flex: 1;
    min-width: 0;
  }
  .test {
    align-items: flex-start;
  }
  .test .btn {
    flex: none;
    gap: 5px;
  }
  .result {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .result code {
    font-family: var(--mono);
    color: var(--text);
  }
  .who {
    font-family: var(--mono);
  }
</style>
