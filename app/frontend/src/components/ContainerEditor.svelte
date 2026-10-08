<script lang="ts">
  import type { ContainerBase, ContainerInfo, ContainerProfile } from '../lib/api'
  import { confirm } from '../lib/confirm.svelte'
  import { untrack } from 'svelte'
  import { addPackages, idFor, MAX_STEP, problems, same, tidy, validPackage } from '../lib/containers'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // Edits one container in the user's containers.yaml (Settings →
  // Containers): its name, base, packages, setup steps and what it shares.
  // A new one gets its id from its name. A built-in one saved here is the
  // user's version until Reset; one of the user's own can be deleted, with
  // its container.
  interface Props {
    start: ContainerProfile
    container?: ContainerInfo // absent for a new one
    bases: ContainerBase[]
    taken: string[] // other containers' ids
    onclose: () => void
  }
  let { start, container, bases, taken, onclose }: Props = $props()
  const app = useApp()
  // The editor opens afresh for each container, so what it started from stays put.
  const isNew = untrack(() => !container)
  let draft = $state<ContainerProfile>(untrack(() => structuredClone($state.snapshot(start))))
  let typed = $state('')
  let saving = $state(false)
  const uid = untrack(() => `ce-${isNew ? 'new' : start.id}`)

  $effect(() => {
    if (isNew) draft.id = idFor(draft.label, taken)
  })

  const wrong = $derived(problems(draft))
  const unsaved = $derived(isNew || !same(draft, start))
  const typedBad = $derived(typed.trim() !== '' && addPackages([], typed).some(p => !validPackage(p)))

  function addTyped() {
    if (!typed.trim() || typedBad) return
    draft.packages = addPackages(draft.packages, typed)
    typed = ''
  }

  function onPackageKey(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
      e.preventDefault()
      addTyped()
    } else if (e.key === 'Backspace' && !typed && draft.packages.length) {
      draft.packages = draft.packages.slice(0, -1)
    }
  }

  const toggle = (k: 'brain' | 'mcp' | 'connectors', on: boolean) => {
    if (k === 'brain') draft.brain = on ? 'shared' : 'sandboxed'
    else draft[k] = on ? 'shared' : 'none'
  }

  async function save() {
    addTyped()
    if (wrong.length || saving) return
    saving = true
    try {
      await app.backend.saveContainer(tidy($state.snapshot(draft) as ContainerProfile))
      await app.loadContainers()
      showToast(t(container?.built ? 'containerEdit.savedBuilt' : 'containerEdit.saved', { label: draft.label.trim() }))
      onclose()
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      saving = false
    }
  }

  async function removeConfig() {
    if (!container) return
    const builtin = container.builtin
    const ok = await confirm({
      title: t(builtin ? 'containerEdit.resetTitle' : 'containerEdit.deleteTitle', { label: container.label }),
      message: t(builtin ? 'containerEdit.resetMessage' : 'containerEdit.deleteMessage'),
      confirmLabel: t(builtin ? 'containerEdit.reset' : 'containerEdit.delete'),
      danger: !builtin,
    })
    if (!ok) return
    try {
      await app.backend.removeContainerConfig(container.id)
      await app.loadContainers()
      onclose()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

<form class="editor" aria-label={isNew ? t('containerEdit.newTitle') : t('containerEdit.title', { label: start.label })} onsubmit={e => { e.preventDefault(); void save() }}>
  <div class="grid">
    <label for="{uid}-name">{t('containerEdit.name')}</label>
    <div>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="{uid}-name" class="input" bind:value={draft.label} autofocus={isNew} maxlength="60" />
      {#if isNew && draft.label.trim()}<span class="hint">{t('containerEdit.id', { id: draft.id })}</span>{/if}
    </div>

    <label for="{uid}-desc">{t('containerEdit.description')}</label>
    <input id="{uid}-desc" class="input" bind:value={draft.description} placeholder={t('containerEdit.descriptionHint')} />

    <label for="{uid}-base">{t('containerEdit.base')}</label>
    <select id="{uid}-base" class="select" bind:value={draft.base}>
      {#each bases as b (b.id)}<option value={b.id}>{b.label}</option>{/each}
    </select>

    <label for="{uid}-pkg">{t('containerEdit.packages')}</label>
    <div>
      <div class="chips" class:bad={typedBad}>
        {#each draft.packages as p (p)}
          <span class="chip" class:bad={!validPackage(p)}>{p}<button type="button" class="x" aria-label={t('containerEdit.removePackage', { name: p })} onclick={() => (draft.packages = draft.packages.filter(x => x !== p))}><Icon name="x" size={12} /></button></span>
        {/each}
        <input id="{uid}-pkg" bind:value={typed} onkeydown={onPackageKey} onblur={addTyped} placeholder={draft.packages.length ? '' : t('containerEdit.packagesHint')} aria-invalid={typedBad} aria-describedby="{uid}-pkg-hint" />
      </div>
      <span class="hint" id="{uid}-pkg-hint" class:err={typedBad}>{typedBad ? t('containerEdit.badPackage') : t('containerEdit.packagesNote')}</span>
    </div>

    <span class="label">{t('containerEdit.setup')}</span>
    <div class="steps">
      <span class="hint">{t('containerEdit.setupNote')}</span>
      {#each draft.setup as _, i (i)}
        <div class="step">
          <textarea class="input mono" rows="2" maxlength={MAX_STEP} bind:value={draft.setup[i]} aria-label={t('containerEdit.step', { n: i + 1 })} placeholder="pip install httpie" spellcheck="false"></textarea>
          <button type="button" class="btn small ghost icon" aria-label={t('containerEdit.removeStep', { n: i + 1 })} title={t('containerEdit.removeStep', { n: i + 1 })} onclick={() => (draft.setup = draft.setup.filter((_, j) => j !== i))}><Icon name="trash" size={14} /></button>
        </div>
      {/each}
      <button type="button" class="btn small" onclick={() => (draft.setup = [...draft.setup, ''])}><Icon name="plus" size={13} />{t('containerEdit.addStep')}</button>
    </div>

    <span class="label">{t('containerEdit.shares')}</span>
    <div class="shares">
      <label class="check"><input type="checkbox" checked={draft.brain === 'shared'} onchange={e => toggle('brain', e.currentTarget.checked)} />{t('containerEdit.shareBrain')}</label>
      <label class="check"><input type="checkbox" checked={draft.mcp === 'shared'} onchange={e => toggle('mcp', e.currentTarget.checked)} />{t('containerEdit.shareMCP')}</label>
      <label class="check"><input type="checkbox" checked={draft.connectors === 'shared'} onchange={e => toggle('connectors', e.currentTarget.checked)} />{t('containerEdit.shareConnectors')}</label>
      {#if draft.connectors === 'shared'}<span class="hint warn">{t('containerEdit.connectorsNote')}</span>{/if}
      <span class="hint">{t('containerEdit.whenApplied')}</span>
    </div>
  </div>

  {#if wrong.length && !isNew}<p class="err" role="alert">{wrong.map(k => t(k)).join(' ')}</p>{/if}
  <div class="actions">
    {#if container && (container.edited || !container.builtin)}
      <button type="button" class="btn small" class:danger={!container.builtin} onclick={removeConfig}>
        {t(container.builtin ? 'containerEdit.reset' : 'containerEdit.delete')}
      </button>
    {/if}
    <span class="grow"></span>
    <button type="button" class="btn small" onclick={onclose}>{t('containerEdit.cancel')}</button>
    <button type="submit" class="btn small primary" disabled={!!wrong.length || !unsaved || saving}>
      {#if saving}<Icon name="loader" spin size={13} />{/if}{t('containerEdit.save')}
    </button>
  </div>
</form>

<style>
  .editor {
    padding: var(--space-3);
    background: var(--surface-2);
    border-top: 1px solid var(--border);
  }
  .grid {
    display: grid;
    grid-template-columns: 110px 1fr;
    gap: var(--space-2) var(--space-3);
    align-items: start;
  }
  .grid > label,
  .label {
    padding-top: 6px;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .grid .input,
  .grid .select {
    width: 100%;
  }
  .hint {
    display: block;
    margin-top: 2px;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .hint.warn {
    color: var(--warning);
  }
  .hint.err,
  .err {
    color: var(--danger);
  }
  .err {
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    min-height: 32px;
    padding: 3px var(--space-1);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
  }
  .chips:focus-within {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
  }
  .chips.bad {
    border-color: var(--danger);
  }
  .chips input {
    flex: 1;
    min-width: 120px;
    border: 0;
    outline: none;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 0 2px 0 var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--surface-3);
    font-family: var(--mono);
    font-size: var(--text-xs);
    line-height: 22px;
    color: var(--text);
  }
  .chip.bad {
    color: var(--danger);
  }
  .x {
    display: inline-flex;
    padding: 2px;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-muted);
  }
  .x:hover {
    color: var(--text);
  }
  .steps,
  .shares {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
    padding-top: 6px;
  }
  .steps .hint {
    margin-top: 0;
  }
  .step {
    display: flex;
    gap: var(--space-1);
    align-self: stretch;
  }
  .step textarea {
    flex: 1;
    height: auto;
    min-height: 52px;
    padding: var(--space-1) var(--space-2);
    resize: vertical;
  }
  .mono {
    font-family: var(--mono);
    font-size: var(--text-xs);
  }
  .check {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .grow {
    flex: 1;
  }
</style>
