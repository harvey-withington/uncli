<script lang="ts">
  import type { TranscriptEntry } from '../lib/api'
  import { useApp } from '../lib/context'
  import { relativeTime } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import { tintStyle } from '../lib/tint'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'

  // Import: conversations the CLI saved, from the terminal or a UNCLI
  // session that was deleted (its transcript outlives it), newest first.
  // Each can be imported as a session (the type is guessed and can be
  // changed) and then continued; one already in UNCLI opens instead. Those
  // whose folder is gone are hidden unless asked for (test runs and
  // throwaway folders); imported, they can be read but not continued.

  const app = useApp()
  let list = $state<TranscriptEntry[] | null>(null)
  let failed = $state('')
  let query = $state('')
  let showGone = $state(false)
  let busy = $state<string | null>(null)
  let types = $state<Record<string, string>>({})

  $effect(() => {
    app.backend.transcripts().then(l => (list = l)).catch(e => (failed = String(e)))
  })

  const folderName = (p: string) => p.split(/[\\/]/).filter(Boolean).pop() ?? p
  const words = $derived(query.toLowerCase().split(/\s+/).filter(Boolean))
  const gone = $derived((list ?? []).filter(e => e.folderGone).length)
  const shown = $derived(
    (list ?? []).filter(e => (showGone || !e.folderGone) && words.every(w => `${e.firstQuestion} ${e.workdir}`.toLowerCase().includes(w))),
  )

  async function importOne(e: TranscriptEntry) {
    busy = e.id
    try {
      await app.importTranscript(e.id, types[e.id] ?? e.profile)
      app.importOpen = false
    } catch (err) {
      showToast(String(err), 'error')
    } finally {
      busy = null
    }
  }
  function open(e: TranscriptEntry) {
    if (!e.sessionId) return
    app.importOpen = false
    void app.select(e.sessionId)
  }
</script>

<Modal title={t('import.title')} width={760} onclose={() => (app.importOpen = false)}>
  <p class="intro">{t('import.intro')}</p>
  <div class="tools">
    <span class="search">
      <Icon name="search" size={14} />
      <input class="input" bind:value={query} placeholder={t('import.search')} aria-label={t('import.search')} data-autofocus />
    </span>
    {#if gone > 0}
      <label class="gone-toggle"><input type="checkbox" bind:checked={showGone} />{t('import.showGone', { n: gone })}</label>
    {/if}
  </div>

  {#if failed}
    <p class="note err" role="alert"><Icon name="triangle-alert" size={14} />{failed}</p>
  {:else if !list}
    <p class="note" role="status"><Icon name="loader" spin size={14} />{t('import.loading')}</p>
  {:else if shown.length === 0}
    <p class="note">{list.length === 0 ? t('import.none') : t('import.noMatch')}</p>
  {:else}
    <ul class="list" aria-label={t('import.list')}>
      {#each shown as e (e.id)}
        {@const type = types[e.id] ?? e.profile}
        {@const profile = app.boot?.profiles.find(p => p.id === type)}
        <li class:gone={e.folderGone}>
          <span class="mode" style={tintStyle(profile?.hue)}><Icon name={profile?.icon ?? 'message-circle'} size={15} /></span>
          <span class="text">
            <span class="q" title={e.firstQuestion}>{e.firstQuestion || t('session.untitled')}</span>
            <span class="meta">
              <span title={e.workdir}>{e.fromChat ? t('import.chatFolder') : folderName(e.workdir)}</span>
              · {relativeTime(e.updated)}
              · {e.turns === 1 ? t('import.turnOne') : t('import.turns', { n: e.turns })}
              {#if e.fromChat && !e.sessionId}<span class="badge">{t('import.deletedChat')}</span>{/if}
              {#if e.folderGone}<span class="badge warn">{t('import.folderGone')}</span>{/if}
            </span>
          </span>
          {#if e.sessionId}
            <button class="btn small" onclick={() => open(e)}><Icon name="external-link" size={13} />{t('import.open')}</button>
          {:else}
            <select class="select small" aria-label={t('import.typeFor', { q: e.firstQuestion })} value={type} onchange={ev => (types[e.id] = ev.currentTarget.value)}>
              {#each app.boot?.profiles ?? [] as p (p.id)}<option value={p.id}>{p.label}</option>{/each}
            </select>
            <button class="btn small primary" onclick={() => importOne(e)} disabled={busy !== null}>
              {#if busy === e.id}<Icon name="loader" spin size={13} />{:else}<Icon name="download" size={13} />{/if}{t('import.import')}
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</Modal>

<style>
  .intro {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .tools {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
  }
  .search {
    flex: 1;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--text-faint);
  }
  .search .input {
    flex: 1;
  }
  .gone-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
  }
  .list {
    position: relative;
    max-height: 52vh;
    overflow-y: auto;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-2);
    border-bottom: 1px solid var(--border);
  }
  li.gone {
    opacity: 0.7;
  }
  .mode {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    flex: none;
    border-radius: 8px;
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .q {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .badge {
    padding: 0 6px;
    border-radius: 999px;
    background: var(--accent-soft);
    color: var(--accent);
  }
  .badge.warn {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .select.small {
    width: auto;
    padding-top: 2px;
    padding-bottom: 2px;
  }
  .note {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .err {
    color: var(--danger);
  }
</style>
