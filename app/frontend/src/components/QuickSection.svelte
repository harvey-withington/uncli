<script lang="ts">
  import type { AutoSummary, Preferences } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { modelOptions } from '../lib/models'
  import { AUTO_SUMMARY_MIN_WORDS } from '../lib/outline'

  // Settings → General → Quick tasks: one provider and model for small jobs
  // outside sessions (titles, page summaries), and when pages are
  // summarised on their own.
  interface Props {
    prefs: Preferences
    savePrefs: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  const quickModels = $derived(modelOptions(app.boot?.models ?? null, prefs.quickTaskModel.model))
  const summaryModes: AutoSummary[] = ['off', 'long', 'always']

  function setQuick(field: 'provider' | 'model', value: string) {
    savePrefs({ ...prefs, quickTaskModel: { ...prefs.quickTaskModel, [field]: value } })
  }
</script>

<section id="settings-quick">
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
    <label for="auto-summary">{t('settings.autoSummary')}</label>
    <select id="auto-summary" class="select" value={prefs.autoSummary} onchange={e => savePrefs({ ...prefs, autoSummary: e.currentTarget.value as AutoSummary })}>
      {#each summaryModes as m (m)}
        <option value={m}>{t(`settings.autoSummary.${m}`)}</option>
      {/each}
    </select>
    <span></span>
    <span class="hint">{t(`settings.autoSummaryHint.${prefs.autoSummary}`, { n: AUTO_SUMMARY_MIN_WORDS })}</span>
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
  .grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-3);
    align-items: center;
    font-size: var(--text-sm);
  }
  .hint {
    margin-top: -2px;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
</style>
