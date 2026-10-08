<script lang="ts">
  import type { Page } from '../lib/api'
  import { artifactKind, artifactsAt, KIND_ICON } from '../lib/artifacts'
  import { formatSize } from '../lib/attachments'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import ArtifactViewer from './ArtifactViewer.svelte'
  import Icon from './Icon.svelte'

  // The "Artifacts" tab: what the session's artifacts folder held after
  // the page being shown (paging back pages the artifacts back too), the
  // ones that page changed marked, and the chosen one in the viewer.

  interface Props {
    page: Page
  }

  let { page }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(app.sessions.find(s => s.id === page.sessionId)?.adapter))
  const sessionId = $derived(page.sessionId)
  const pages = $derived(app.pages[sessionId] ?? [])
  const index = $derived(Math.max(0, pages.findIndex(p => p.id === page.id)))
  const entries = $derived(artifactsAt(pages, index, app.artifactLive[sessionId] ?? null))
  // The one chosen, if it exists as of this page; else what this page
  // changed, else the first.
  const selected = $derived(
    entries.find(e => e.path === app.artifactSel[sessionId]) ?? entries.find(e => e.changed) ?? entries[0] ?? null,
  )

  $effect(() => {
    void app.refreshArtifacts(sessionId)
  })
</script>

<div class="artifacts">
  <p class="asof">{t('artifacts.asOf', { n: page.seq })}</p>
  {#if entries.length === 0}
    <p class="empty">{t('artifacts.empty', names)}</p>
  {:else}
    <ul class="list" aria-label={t('artifacts.list')}>
      {#each entries as e (e.path)}
        <li>
          <button class="item" class:on={selected?.path === e.path} aria-current={selected?.path === e.path} onclick={() => (app.artifactSel[sessionId] = e.path)}>
            <Icon name={KIND_ICON[artifactKind(e.path)]} size={14} />
            <span class="path">{e.path}</span>
            {#if e.changed}<span class="changed" title={t('artifacts.changedHere')}>{t('artifacts.changed')}</span>{/if}
            <span class="size">{formatSize(e.size)}</span>
          </button>
        </li>
      {/each}
    </ul>
    {#if selected}
      {#key `${selected.path}|${selected.hash ?? 'live'}`}
        <ArtifactViewer {sessionId} entry={selected} />
      {/key}
    {/if}
  {/if}
</div>

<style>
  .artifacts {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .asof {
    margin: var(--space-2) var(--space-4);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .empty {
    margin: var(--space-3) var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .list {
    position: relative; /* clips absolutely positioned descendants (as .scroll in SessionPane) */
    flex: none;
    max-height: 32%;
    overflow-y: auto;
    margin: 0;
    padding: 0 var(--space-2) var(--space-2);
    list-style: none;
    border-bottom: 1px solid var(--border);
  }
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-height: 30px;
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    text-align: left;
  }
  .item:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  .item.on {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
    font-family: var(--mono);
    font-size: 12px;
  }
  .changed {
    flex: none;
    padding: 0 6px;
    border-radius: 999px;
    background: var(--accent-soft);
    color: var(--accent);
    font-size: 11px;
  }
  .size {
    flex: none;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
