<script lang="ts">
  import { untrack } from 'svelte'
  import type { ClassPreview, SafeClass, SafeEntry, Scope, Verdict } from '../lib/api'
  import { classLabel, scopeOf } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // The safe list: the user's corrections to what UNCLI counts as safe,
  // for the current session's project and all projects. Each entry's
  // verdict and scope change in place; moving an entry to All projects
  // promotes it (the backend merges duplicates). "Add" takes an example
  // command (or a tool's name) and shows the class it becomes first.
  interface Props {
    sessionId: string
    filter: 'any' | Scope
  }

  let { sessionId, filter }: Props = $props()
  const app = useApp()
  let entries = $state<SafeEntry[]>([])
  let tools = $state<string[]>([])
  let loading = $state(true)

  const VERDICTS: Verdict[] = ['safe', 'unsafe', 'blocked']
  const shown = $derived(entries.filter(e => filter === 'any' || scopeOf(e) === filter))

  $effect(() => {
    const id = sessionId
    untrack(() => {
      loading = true
      Promise.all([app.backend.safeList(id), app.backend.knownTools(id)]).then(
        ([list, known]) => { entries = list; tools = known; loading = false },
        e => { showToast(String(e), 'error'); loading = false },
      )
    })
  })

  async function run(f: () => Promise<SafeEntry[]>) {
    try {
      entries = await f()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
  const key = (e: SafeEntry) => `${e.folder ?? ''}|${e.kind}|${e.words}|${e.flags ?? ''}`

  // Labels: the user's own name for an entry, edited in place.
  let editing = $state('') // the key of the entry being labelled
  let draftLabel = $state('')
  function startLabel(e: SafeEntry) {
    editing = key(e)
    draftLabel = e.label ?? ''
  }
  async function saveLabel(e: SafeEntry) {
    if (editing !== key(e)) return
    editing = ''
    if ((e.label ?? '') !== draftLabel.trim()) await run(() => app.backend.setSafeLabel(sessionId, e, draftLabel))
  }
  function labelKeys(ev: KeyboardEvent, e: SafeEntry) {
    if (ev.key === 'Enter') {
      ev.preventDefault()
      void saveLabel(e)
    } else if (ev.key === 'Escape') {
      ev.stopPropagation() // not the dialog
      editing = ''
    }
  }
  const focus = (el: HTMLInputElement) => el.focus()

  // Adding: an example, the classes it becomes, a verdict and a scope.
  let example = $state('')
  let preview = $state<ClassPreview[]>([])
  let verdict = $state<Verdict>('safe')
  let scope = $state<Scope>('project')
  let timer: number | undefined
  const asTool = $derived(tools.includes(example.trim()))
  const classes = $derived<SafeClass[]>(asTool ? [{ kind: 'tool', words: example.trim() }] : preview.flatMap(p => (p.class ? [p.class] : [])))

  function onInput() {
    clearTimeout(timer)
    const text = example.trim()
    if (!text || tools.includes(text)) {
      preview = []
      return
    }
    timer = window.setTimeout(async () => {
      try {
        preview = await app.backend.previewClasses(sessionId, text)
      } catch (e) {
        showToast(String(e), 'error')
      }
    }, 250)
  }

  async function add() {
    if (classes.length === 0) return
    try {
      await app.backend.teach(sessionId, classes, verdict, scope)
      entries = await app.backend.safeList(sessionId)
      example = ''
      preview = []
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

{#if loading}
  <p class="empty"><Icon name="loader" spin size={14} /></p>
{:else if shown.length === 0}
  <p class="empty">{t(filter === 'any' ? 'safe.none' : 'safe.noneHere')}</p>
{:else}
  <ul class="entries" aria-label={t('safe.list')}>
    {#each shown as e (key(e))}
      {@const what = classLabel(e)}
      <li>
        {#if editing === key(e)}
          <input class="input small label-edit" bind:value={draftLabel} use:focus maxlength="80"
            aria-label={t('safe.labelFor', { what })} placeholder={t('safe.labelPlaceholder')}
            onkeydown={ev => labelKeys(ev, e)} onblur={() => saveLabel(e)} />
        {:else}
          <span class="what" title={e.kind === 'tool' ? e.words : what}>
            {#if e.kind === 'tool'}<Icon name="wrench" size={12} />{/if}
            {#if e.label}<span class="label">{e.label}</span><code class="aside">{what}</code>{:else}<code>{what}</code>{/if}
          </span>
          <button class="btn ghost small icon" onclick={() => startLabel(e)} aria-label={t(e.label ? 'safe.renameLabel' : 'safe.addLabel', { what })} title={t(e.label ? 'safe.renameLabel' : 'safe.addLabel', { what })}>
            <Icon name="pencil" size={13} />
          </button>
        {/if}
        <select class="select small {e.verdict}" value={e.verdict} aria-label={t('safe.verdictFor', { what })}
          onchange={ev => run(() => app.backend.setSafeEntry(sessionId, { ...e, verdict: ev.currentTarget.value as Verdict }, scopeOf(e)))}>
          {#each VERDICTS as v (v)}<option value={v}>{t(`verdict.${v}`)}</option>{/each}
        </select>
        <select class="select small" value={scopeOf(e)} aria-label={t('safe.scopeFor', { what })}
          onchange={ev => run(() => app.backend.moveSafeEntry(sessionId, e, ev.currentTarget.value as Scope))}>
          <option value="project">{t('scope.project')}</option>
          <option value="all">{t('scope.all')}</option>
        </select>
        <button class="btn ghost small icon" onclick={() => run(() => app.backend.deleteSafeEntry(sessionId, e))} aria-label={t('safe.remove', { what })} title={t('safe.remove', { what })}>
          <Icon name="trash" size={13} />
        </button>
      </li>
    {/each}
  </ul>
{/if}

<form class="add" onsubmit={e => { e.preventDefault(); void add() }}>
  <input class="input small" bind:value={example} oninput={onInput} list="safe-tools-{sessionId}" placeholder={t('safe.examplePlaceholder')} aria-label={t('safe.example')} />
  <datalist id="safe-tools-{sessionId}">
    {#each tools as tool (tool)}<option value={tool}></option>{/each}
  </datalist>
  <select class="select small" bind:value={verdict} aria-label={t('safe.addVerdict')}>
    {#each VERDICTS as v (v)}<option value={v}>{t(`verdict.${v}`)}</option>{/each}
  </select>
  <select class="select small" bind:value={scope} aria-label={t('scope.label')}>
    <option value="project">{t('scope.project')}</option>
    <option value="all">{t('scope.all')}</option>
  </select>
  <button class="btn small" type="submit" disabled={classes.length === 0}><Icon name="plus" size={13} />{t('safe.add')}</button>
</form>
{#if example.trim() && (classes.length > 0 || preview.length > 0)}
  <ul class="preview" aria-label={t('safe.preview')}>
    {#if asTool}
      <li>{t('safe.willAdd')} <code>{classLabel(classes[0] as SafeClass)}</code></li>
    {:else}
      {#each preview as p, i (i)}
        <li>
          {#if p.class}{t('safe.willAdd')} <code>{classLabel(p.class)}</code>
            {#if p.class.kind === 'command'}<span class="any">{t('approval.anyFiles')}</span>{/if}
          {:else}<code>{p.part}</code> {t(`fixed.${p.fixed ?? 'complex'}`)}{/if}
        </li>
      {/each}
    {/if}
  </ul>
{/if}

<style>
  .empty {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-faint);
  }
  .entries {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0 0 var(--space-3);
    padding: 0;
    list-style: none;
  }
  .entries li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 4px var(--space-2);
    border-radius: var(--radius-sm);
  }
  .entries li:hover {
    background: var(--surface-2);
  }
  .what {
    display: inline-flex;
    flex: 1;
    align-items: center;
    gap: 6px;
    min-width: 0;
    overflow: hidden;
    color: var(--text-muted);
  }
  .what code {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--mono);
    font-size: var(--text-xs);
    color: var(--text);
  }
  .what .label {
    flex: none;
    max-width: 60%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
    color: var(--text);
  }
  .what code.aside {
    color: var(--text-faint);
  }
  .label-edit {
    flex: 1;
    min-width: 0;
  }
  .small {
    height: 28px;
    font-size: var(--text-xs);
  }
  select.unsafe {
    color: var(--warning);
  }
  select.blocked {
    color: var(--danger);
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
  }
  .add .input {
    flex: 1 1 100%;
    min-width: 0;
    font-family: var(--mono);
  }
  .preview {
    margin: var(--space-2) 0 0;
    padding: 0 var(--space-2);
    list-style: none;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .preview code {
    font-family: var(--mono);
    color: var(--text);
  }
</style>
