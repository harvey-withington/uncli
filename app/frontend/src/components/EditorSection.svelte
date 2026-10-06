<script lang="ts">
  import { untrack } from 'svelte'
  import type { Editor, Preferences } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'

  // Settings → Editor: where Code sessions open the files a page lists, at
  // the line: the first editor installed (auto), a particular one, or the
  // user's own command with {file}, {line} and {folder}.

  interface Props {
    prefs: Preferences
    savePrefs: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  let editors = $state<Editor[]>(app.boot?.editors ?? [])
  // Custom picked but not saved yet: it needs a command first.
  let picked = $state<string | null>(null)
  const choice = $derived(picked ?? (prefs.editor || 'auto'))
  const firstFound = $derived(editors.find(e => e.found)?.label ?? '')
  let command = $state(untrack(() => prefs.editorCommand ?? '')) // edited here until saved
  const dirty = $derived(picked !== null || command.trim() !== (prefs.editorCommand ?? '').trim())

  // Look again: an editor may have been installed since UNCLI started.
  $effect(() => {
    app.backend.editors().then(e => {
      editors = e
      if (app.boot) app.boot.editors = e
    }).catch(() => {})
  })

  function choose(value: string) {
    if (value === 'custom' && !command.trim()) {
      picked = 'custom' // type the command, then Save
      return
    }
    picked = null
    savePrefs({ ...prefs, editor: value, editorCommand: value === 'custom' ? command.trim() : prefs.editorCommand })
  }

  async function saveCommand() {
    if (await savePrefs({ ...prefs, editor: 'custom', editorCommand: command.trim() })) picked = null
  }
</script>

<section id="settings-editor">
  <h3>{t('settings.editorTitle')}</h3>
  <p class="muted">{t('settings.editorBody')}</p>
  <select class="select" aria-label={t('settings.editorTitle')} value={choice} onchange={e => choose(e.currentTarget.value)}>
    <option value="auto">{firstFound ? t('settings.editorAuto', { editor: firstFound }) : t('settings.editorAutoNone')}</option>
    {#each editors as e (e.id)}
      <option value={e.id}>{e.found ? e.label : t('settings.editorMissing', { editor: e.label })}</option>
    {/each}
    <option value="custom">{t('settings.editorCustom')}</option>
  </select>
  {#if choice === 'custom'}
    <div class="row">
      <input class="input" bind:value={command} aria-label={t('settings.editorCommand')} placeholder={'subl {file}:{line}'} spellcheck="false" />
      <button class="btn" disabled={!dirty || !command.trim()} onclick={saveCommand}>{t('decider.save')}</button>
    </div>
    <p class="hint">{t('settings.editorCommandHint')}</p>
  {:else if choice !== 'auto' && !editors.find(e => e.id === choice)?.found}
    <p class="hint warn">{t('settings.editorNotFound')}</p>
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
  .muted {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .select {
    width: 100%;
  }
  .row {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
  .row .input {
    flex: 1;
    font-family: var(--mono);
  }
  .hint {
    margin: var(--space-1) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .warn {
    color: var(--warning);
  }
</style>
