<script lang="ts">
  import type { SearchHit, SearchQuery } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { groupHits, RANGE_DAYS, snippetParts, type SearchRange } from '../lib/search'
  import { tintStyle } from '../lib/tint'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // Search across sessions: the box at the top of the sidebar (Ctrl+K), and
  // while it has text, results in place of the session list, grouped by
  // session, best first. Arrow keys move through them, Enter opens one.
  interface Props {
    results?: boolean // render the results list (the sidebar's body) instead of the box
  }

  let { results = false }: Props = $props()
  const app = useApp()
  let input: HTMLInputElement | undefined = $state()
  const DEBOUNCE_MS = 120
  let requests = 0 // not reactive: the search effect would re-run on its own count

  // Results live on the store's search state, shared by the box and the list.
  const f = $derived(app.searchFilters)
  const active = $derived(app.searchText.trim() !== '')

  $effect(() => {
    void app.searchFocus
    if (app.searchFocus && input) {
      input.focus()
      input.select()
    }
  })

  function onkeydown(e: KeyboardEvent) {
    const hits = app.search.result?.hits ?? []
    if (e.key === 'Escape') {
      e.preventDefault()
      if (app.searchText) app.searchText = ''
      else input?.blur()
    } else if (e.key === 'ArrowDown' && hits.length) {
      e.preventDefault()
      app.search.active = Math.min(hits.length - 1, app.search.active + 1)
      scrollActive()
    } else if (e.key === 'ArrowUp' && hits.length) {
      e.preventDefault()
      app.search.active = Math.max(0, app.search.active - 1)
      scrollActive()
    } else if (e.key === 'Enter' && hits[app.search.active]) {
      e.preventDefault()
      open(hits[app.search.active] as SearchHit)
    }
  }

  function scrollActive() {
    queueMicrotask(() => document.querySelector('.search-results .hit.active')?.scrollIntoView?.({ block: 'nearest' }))
  }

  function open(hit: SearchHit) {
    void app.openHit(hit, app.search.result?.terms ?? [])
  }

  function toggleProfile(id: string) {
    const on = f.profiles.includes(id)
    app.searchFilters.profiles = on ? f.profiles.filter(p => p !== id) : [...f.profiles, id]
  }

  // The query, debounced; a late answer to an older query is dropped.
  $effect(() => {
    if (results) return // the box runs the search; the list only shows it
    const text = app.searchText
    if (!text.trim()) {
      app.search.result = null
      return
    }
    const days = RANGE_DAYS[f.range]
    const q: SearchQuery = {
      text,
      profiles: [...f.profiles],
      sessionId: f.thisSession && app.currentId ? app.currentId : undefined,
      bookmarked: f.bookmarked || undefined,
      since: days ? Date.now() - days * 86_400_000 : undefined,
    }
    const seq = ++requests
    const timer = setTimeout(async () => {
      try {
        const r = await app.backend.search(q)
        if (seq === requests) {
          app.search.result = r
          app.search.active = 0
        }
      } catch (e) {
        if (seq === requests) showToast(String(e), 'error')
      }
    }, DEBOUNCE_MS)
    return () => clearTimeout(timer)
  })

  const groups = $derived(groupHits(app.search.result?.hits ?? []))
  const flat = $derived(app.search.result?.hits ?? [])
  const profileOf = (id: string) => app.boot?.profiles.find(p => p.id === id)
</script>


{#if !results}
  <div class="box" class:active>
    <Icon name="search" size={14} />
    <input
      bind:this={input}
      bind:value={app.searchText}
      type="search"
      placeholder={t('search.placeholder')}
      aria-label={t('search.label')}
      aria-controls="search-results"
      autocomplete="off"
      spellcheck="false"
      {onkeydown}
    />
    {#if app.searchText}
      <button class="clear" onclick={() => { app.searchText = ''; input?.focus() }} aria-label={t('search.clear')} title={t('search.clear')}>
        <Icon name="x" size={12} />
      </button>
    {:else}
      <kbd>Ctrl K</kbd>
    {/if}
  </div>
  {#if active}
    <div class="filters" role="group" aria-label={t('search.filters')}>
      {#each app.boot?.profiles ?? [] as p (p.id)}
        <button
          class="chip type"
          class:on={f.profiles.includes(p.id)}
          style={tintStyle(p.hue)}
          aria-pressed={f.profiles.includes(p.id)}
          title={t('search.type', { type: p.label })}
          aria-label={t('search.type', { type: p.label })}
          onclick={() => toggleProfile(p.id)}
        ><Icon name={p.icon} size={13} /></button>
      {/each}
      <button class="chip" class:on={f.bookmarked} aria-pressed={f.bookmarked} title={t('search.bookmarked')} aria-label={t('search.bookmarked')} onclick={() => (app.searchFilters.bookmarked = !f.bookmarked)}>
        <Icon name="bookmark" size={13} />
      </button>
      <button class="chip text" class:on={f.thisSession} aria-pressed={f.thisSession} disabled={!app.currentId} onclick={() => (app.searchFilters.thisSession = !f.thisSession)}>
        {t('search.thisSession')}
      </button>
      <select class="range" aria-label={t('search.range')} value={f.range} onchange={e => (app.searchFilters.range = e.currentTarget.value as SearchRange)}>
        {#each Object.keys(RANGE_DAYS) as r (r)}
          <option value={r}>{t(`search.range.${r}`)}</option>
        {/each}
      </select>
    </div>
  {/if}
{:else}
  <div class="search-results" id="search-results" aria-label={t('search.results')} role="region">
    {#if app.search.result}
      <p class="count" aria-live="polite">
        {flat.length === 0 ? t('search.none') : flat.length === 1 ? t('search.countOne') : t('search.count', { n: flat.length })}
      </p>
      {#each groups as g (g.sessionId)}
        {@const p = profileOf(g.profileId)}
        <section class="group">
          <h3 style={tintStyle(p?.hue)}>
            <span class="gicon"><Icon name={p?.icon ?? 'message-circle'} size={12} /></span>
            <span class="gtitle">{g.title || t('session.untitled')}</span>
          </h3>
          {#each g.hits as h (h.pageId)}
            <button
              class="hit"
              class:active={flat[app.search.active]?.pageId === h.pageId}
              class:current={app.currentPage?.id === h.pageId}
              onclick={() => {
                app.search.active = flat.findIndex(x => x.pageId === h.pageId)
                open(h)
              }}
            >
              <span class="hhead">
                <span class="pnum">{t('search.page', { n: h.seq })}</span>
                <span class="hq">{h.question}</span>
                {#if h.bookmarked}<Icon name="bookmark-check" size={12} />{/if}
              </span>
              <span class="snip">
                {#if h.field === 'extra'}<span class="where">{t('search.inFiles')}: </span>{:else if h.field === 'title'}<span class="where">{t('search.inTitle')}: </span>{/if}
                {#each snippetParts(h.snippet) as part, i (i)}{#if part.mark}<mark>{part.text}</mark>{:else}{part.text}{/if}{/each}
              </span>
            </button>
          {/each}
        </section>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .box {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: 32px;
    margin-bottom: var(--space-2);
    padding: 0 var(--space-2) 0 var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text-faint);
    transition: border-color var(--fast) var(--ease), box-shadow var(--fast) var(--ease);
  }
  .box:focus-within {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
    color: var(--accent);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 100%;
    border: 0;
    outline: none;
    background: transparent;
    color: var(--text);
    font-size: var(--text-sm);
  }
  input::-webkit-search-cancel-button {
    display: none;
  }
  input::placeholder {
    color: var(--text-faint);
  }
  kbd {
    font-size: 10.5px;
    font-family: var(--font);
    color: var(--text-faint);
  }
  .clear {
    display: inline-grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: 0;
    border-radius: 4px;
    background: none;
    color: var(--text-muted);
  }
  .clear:hover {
    background: var(--surface-2);
  }
  .filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    margin-bottom: var(--space-2);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    height: 24px;
    min-width: 26px;
    padding: 0 6px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface);
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .chip.type {
    color: var(--tint, var(--text-muted));
  }
  .chip.on {
    border-color: var(--tint, var(--accent));
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
  }
  .chip:disabled {
    opacity: 0.5;
  }
  .range {
    height: 24px;
    margin-left: auto;
    padding: 0 4px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface);
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .search-results {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .count {
    margin: 0 var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .group h3 {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 var(--space-1) 2px;
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-muted);
  }
  .gicon {
    display: inline-flex;
    color: var(--tint, var(--text-muted));
  }
  .gtitle {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hit {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 6px var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    text-align: left;
    color: var(--text);
  }
  .hit:hover,
  .hit.active {
    background: var(--surface-2);
  }
  .hit.current {
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .hhead {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .pnum {
    flex: none;
    font-weight: 600;
  }
  .hq {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .snip {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    font-size: var(--text-sm);
    line-height: 1.4;
  }
  .where {
    color: var(--text-faint);
  }
  mark {
    padding: 0 1px;
    border-radius: 3px;
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
</style>
