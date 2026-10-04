<script lang="ts">
  import type { Preferences, Scope, UnknownCommands } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import CommandCheck from './CommandCheck.svelte'
  import SafeList from './SafeList.svelte'
  import SessionTypeAllowlist from './SessionTypeAllowlist.svelte'

  // Settings → "Safe and unsafe": the safe list for the current session's
  // project, filtered by scope or showing the session type's built-in list,
  // "Check a command", and what "Prompt when unsafe" does with what UNCLI
  // can't place. The header's shield opens Settings here.
  interface Props {
    savePrefs: (next: Preferences) => void
  }

  let { savePrefs }: Props = $props()
  const app = useApp()
  // Plain values, so a session update (its state, say) doesn't reload the list.
  const sessionId = $derived(app.current?.id ?? '')
  const workdir = $derived(app.current?.workdir ?? '')
  const profileId = $derived(app.current?.profileId ?? '')
  const prefs = $derived(app.boot?.preferences)
  const typeLabel = $derived(app.boot?.profiles.find(p => p.id === profileId)?.label ?? '')
  const unknownModes: UnknownCommands[] = ['model', 'inside', 'ask']
  type Filter = 'any' | Scope | 'builtin'
  const FILTERS: Filter[] = ['any', 'project', 'all', 'builtin']
  let filter = $state<Filter>('any')
</script>

<section id="settings-safe">
  <h3>{t('safe.title')}</h3>
  {#if sessionId}
    <p class="muted">{t('safe.body', { folder: workdir })}</p>
    <div class="filters" role="radiogroup" aria-label={t('safe.filter')}>
      {#each FILTERS as f (f)}
        <button class="chip" class:on={filter === f} role="radio" aria-checked={filter === f} onclick={() => (filter = f)}>{t(`safe.filter.${f}`)}</button>
      {/each}
    </div>
    {#if filter !== 'builtin'}
      <SafeList {sessionId} {filter} />
    {/if}
    {#if filter === 'builtin' || filter === 'any'}
      <SessionTypeAllowlist {sessionId} {typeLabel} />
    {/if}
    <CommandCheck {sessionId} />
  {:else}
    <p class="muted">{t('safe.noSession')}</p>
  {/if}
  {#if prefs}
    <h4>{t('settings.unknown')}</h4>
    <p class="muted">{t('settings.unknown.hint')}</p>
    <select id="unknown-commands" class="select wide" aria-label={t('settings.unknown')} value={prefs.unknownCommands ?? 'model'} onchange={e => savePrefs({ ...prefs, unknownCommands: e.currentTarget.value as UnknownCommands })}>
      {#each unknownModes as m (m)}
        <option value={m}>{t(`settings.unknown.${m}`)}</option>
      {/each}
    </select>
  {/if}
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
  h4 {
    margin: var(--space-4) 0 var(--space-1);
    font-size: var(--text-sm);
  }
  .muted {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
    overflow-wrap: anywhere;
  }
  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-bottom: var(--space-2);
  }
  .chip {
    height: 24px;
    padding: 0 10px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--text-xs);
    cursor: pointer;
  }
  .chip:hover {
    color: var(--text);
  }
  .chip.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent);
  }
  .wide {
    width: 100%;
  }
</style>
