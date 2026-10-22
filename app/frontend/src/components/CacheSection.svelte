<script lang="ts">
  import type { Preferences, PromptCache } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'

  // Settings → AI Providers → Conversation cache: how long a session's
  // conversation stays cached after its last message. Takes effect at each
  // session's next message.
  interface Props {
    prefs: Preferences
    savePrefs: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  // Only the first provider's CLI takes a cache lifetime today.
  const names = $derived(app.namesFor())
  const choices: PromptCache[] = ['auto', '5m', '1h']
  const current = $derived(prefs.promptCache ?? 'auto')
</script>

<section id="settings-cache">
  <h3>{t('settings.cacheTitle')}</h3>
  <p class="muted">{t('settings.cacheBody')}</p>
  <select
    class="select"
    aria-label={t('settings.cacheTitle')}
    value={current}
    onchange={e => savePrefs({ ...prefs, promptCache: e.currentTarget.value as PromptCache })}
  >
    {#each choices as c (c)}
      <option value={c}>{t(`settings.cache.${c}`)}</option>
    {/each}
  </select>
  <p class="hint">{t(`settings.cacheHint.${current}`, names)}</p>
  <p class="hint">{t('settings.cacheScope', names)}</p>
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
  .select {
    width: 100%;
  }
  .hint {
    margin: var(--space-2) 0 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
</style>
