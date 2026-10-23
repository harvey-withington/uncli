<script lang="ts">
  import type { WatchEntry } from '../lib/api'
  import { useApp } from '../lib/context'
  import { splitPath } from '../lib/files'
  import { relativeTime } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { choices } from '../lib/watched'
  import Icon from './Icon.svelte'

  // The File tab's switcher: the session folder's text files, found by any
  // part of their path. With nothing typed it lists starred files, then the
  // ones shown lately, then the rest; with a search, the matches newest
  // first, so "todo" puts the latest TODO file at the top. ↑/↓ choose,
  // Enter shows, Escape closes; the input keeps focus (aria-activedescendant).
  // Shown over the file (overlay) or, while no file is chosen, in its place.

  interface Props {
    sessionId: string
    overlay?: boolean
    focus?: number // takes the focus when this changes to a non-zero value (0: not on showing)
    onpick: (path: string) => void
    onclose?: () => void
  }

  let { sessionId, overlay = false, focus = 1, onpick, onclose }: Props = $props()
  const app = useApp()
  let query = $state('')
  let entries = $state<WatchEntry[]>([])
  let loading = $state(true)
  let error = $state('')
  let active = $state(0)
  let listEl: HTMLElement | undefined = $state()
  let input: HTMLInputElement | undefined = $state()
  const watched = $derived(app.watchedFor(sessionId))
  const list = $derived(choices(query, entries, watched))
  const now = Date.now()

  // Each search asks the folder again (it changes as the agent works), a
  // moment after the typing stops.
  $effect(() => {
    const q = query
    const sid = sessionId
    let stale = false
    const timer = setTimeout(() => {
      app.backend
        .watchFiles(sid, q)
        .then(list => {
          if (stale) return
          entries = list
          error = ''
        })
        .catch(e => {
          if (!stale) error = String(e)
        })
        .finally(() => {
          if (!stale) loading = false
        })
    }, q ? 120 : 0)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  })
  $effect(() => {
    if (focus) input?.focus()
  })
  $effect(() => {
    void query
    active = 0
  })
  $effect(() => {
    listEl?.querySelector(`#watch-opt-${active}`)?.scrollIntoView?.({ block: 'nearest' })
  })

  function onkeydown(e: KeyboardEvent) {
    const n = list.length
    if (e.key === 'Escape' && onclose) {
      e.stopPropagation()
      onclose()
    } else if (e.key === 'ArrowDown' && n) {
      e.preventDefault()
      active = (active + 1) % n
    } else if (e.key === 'ArrowUp' && n) {
      e.preventDefault()
      active = (active - 1 + n) % n
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const c = list[active]
      if (c) onpick(c.path)
    }
  }
</script>

<div class="switcher" class:overlay>
  <div class="field">
    <Icon name="search" size={14} />
    <input
      bind:this={input}
      bind:value={query}
      {onkeydown}
      role="combobox"
      aria-expanded="true"
      aria-controls="watch-list"
      aria-activedescendant={list.length ? `watch-opt-${active}` : undefined}
      aria-label={t('watch.searchLabel')}
      placeholder={t('watch.search')}
      spellcheck="false"
      autocomplete="off"
    />
    {#if onclose}
      <button class="btn ghost small icon" onclick={onclose} aria-label={t('common.close')} title={t('common.close')}><Icon name="x" size={14} /></button>
    {/if}
  </div>
  {#if error}
    <p class="note err" role="alert">{error}</p>
  {:else if list.length === 0}
    <p class="note" role="status">{loading ? t('watch.loading') : t('watch.none')}</p>
  {:else}
    <ul id="watch-list" role="listbox" aria-label={t('watch.searchLabel')} bind:this={listEl}>
      {#each list as c, i (c.path)}
        {@const parts = splitPath(c.path)}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <li
          id="watch-opt-{i}"
          role="option"
          aria-selected={i === active}
          class:active={i === active}
          class:current={c.path === watched.current}
          title={c.path}
          onpointermove={() => (active = i)}
          onclick={() => onpick(c.path)}
        >
          <span class="mark" class:star={c.mark === 'starred'}>
            {#if c.mark === 'starred'}<Icon name="star" size={12} fill label={t('watch.starred')} />
            {:else if c.mark === 'recent'}<Icon name="history" size={12} label={t('watch.recent')} />{/if}
          </span>
          <span class="path"><span class="name">{parts.name}</span>{#if parts.dir}<span class="dir">{parts.dir.slice(0, -1)}</span>{/if}</span>
          {#if c.modified}<span class="when">{relativeTime(c.modified, now)}</span>{/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .switcher {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .overlay {
    flex: 1;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-lg);
  }
  .field {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-1) var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--border);
    color: var(--text-faint);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 24px;
    border: 0;
    background: none;
    outline: none;
    color: var(--text);
    font-size: var(--text-sm);
  }
  ul {
    position: relative;
    margin: 0;
    padding: var(--space-1);
    list-style: none;
    overflow-y: auto;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 5px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: var(--text-sm);
  }
  li.active {
    background: var(--accent-soft);
  }
  .mark {
    display: inline-flex;
    flex: none;
    width: 12px;
    color: var(--text-faint);
  }
  .mark.star {
    color: var(--warning);
  }
  .path {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    overflow: hidden;
    white-space: nowrap;
  }
  .name {
    flex: none;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text);
    font-weight: 600;
  }
  li.current .name,
  li.active .name {
    color: var(--accent);
  }
  .dir {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .when {
    flex: none;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .note {
    margin: var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .err {
    color: var(--danger);
  }
</style>
