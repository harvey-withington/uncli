<script lang="ts">
  import type { Page } from '../lib/api'
  import { artifactsAt } from '../lib/artifacts'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { OUTLINE_MAX, OUTLINE_MIN } from '../lib/outline'
  import type { PanelTab } from '../lib/panels'
  import ArtifactPane from './ArtifactPane.svelte'
  import FilePane from './FilePane.svelte'
  import OutlinePanel from './OutlinePanel.svelte'
  import Icon from './Icon.svelte'
  import ResizeHandle from './ResizeHandle.svelte'

  // The side panel beside the answer: "On this page", for session types
  // that keep artifacts "Artifacts", and "File" (a file from the session
  // folder, watched live), as tabs. The first two have a badge with how
  // many sections or artifacts the page has; File is named for the file it
  // shows. The left edge resizes the panel. A session with no page yet
  // shows the panel all the same, its page tabs saying so.

  interface Props {
    sessionId: string
    page: Page | null
    scroller: HTMLElement | undefined
  }

  let { sessionId, page, scroller }: Props = $props()
  const app = useApp()
  const hasArtifacts = $derived(app.hasArtifacts(sessionId))
  const tab = $derived(app.tabFor(sessionId))
  const tabs = $derived<PanelTab[]>(hasArtifacts ? ['outline', 'artifacts', 'file'] : ['outline', 'file'])
  const watching = $derived(app.watchedFor(sessionId).current)

  const sections = $derived(page ? app.outlineFor(page).entries.length : 0)
  const artifacts = $derived.by(() => {
    if (!hasArtifacts || !page) return 0
    const pages = app.pages[sessionId] ?? []
    const index = Math.max(0, pages.findIndex(p => p.id === page.id))
    return artifactsAt(pages, index, app.artifactLive[sessionId] ?? null).length
  })

  const TAB_ICON: Record<PanelTab, string> = { outline: 'table-of-contents', artifacts: 'layers', file: 'eye' }
  const label = (k: PanelTab) =>
    k === 'file' ? watching.slice(watching.lastIndexOf('/') + 1) || t('watch.title') : t(k === 'artifacts' ? 'artifacts.title' : 'outline.title')
  const count = (k: PanelTab) => (k === 'artifacts' ? artifacts : sections)
  const countLabel = (k: PanelTab) => {
    const key = k === 'artifacts' ? 'panel.artifactCount' : 'panel.sectionCount'
    return count(k) === 1 ? t(key + 'One') : t(key, { n: count(k) })
  }

  function choose(k: PanelTab) {
    app.setTab(k)
    if (k === 'artifacts') void app.refreshArtifacts(sessionId)
  }

  // Arrow keys move between the tabs (one tab stop, as in ModeSwitch).
  function onkeydown(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight' && e.key !== 'Home' && e.key !== 'End') return
    e.preventDefault()
    const i = tabs.indexOf(tab)
    const next = e.key === 'Home' ? 0 : e.key === 'End' ? tabs.length - 1 : (i + (e.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    const k = tabs[next]
    if (!k) return
    choose(k)
    ;(e.currentTarget as HTMLElement).querySelector<HTMLElement>(`[data-tab="${k}"]`)?.focus()
  }
</script>

<aside class="side" style:width="{app.outline.width}px" aria-label={label(tab)}>
  <ResizeHandle edge="left" width={app.outline.width} min={OUTLINE_MIN} max={OUTLINE_MAX} label={t('panel.resize')}
    onresize={w => (app.outline.width = w)} oncommit={w => app.setOutlineWidth(w)} />
  <div class="tabs" role="tablist" aria-label={t('panel.label')} tabindex="-1" {onkeydown}>
    {#each tabs as k (k)}
      <button
        role="tab"
        data-tab={k}
        id="side-tab-{k}"
        aria-selected={tab === k}
        aria-controls="side-panel"
        tabindex={tab === k ? 0 : -1}
        class:on={tab === k}
        onclick={() => choose(k)}
      >
        <Icon name={TAB_ICON[k]} size={13} />
        {#if k === 'file'}
          <span class="name file" title={watching || undefined}>{label(k)}</span>
        {:else}
          <span class="name">{label(k)}</span>
          {#if page}<span class="badge" aria-label={countLabel(k)} title={countLabel(k)}>{count(k)}</span>{/if}
        {/if}
      </button>
    {/each}
  </div>
  <div class="body" id="side-panel" role="tabpanel" aria-labelledby="side-tab-{tab}">
    {#if tab === 'file'}
      <FilePane {sessionId} />
    {:else if !page}
      <p class="none">{t('panel.noPage')}</p>
    {:else if tab === 'artifacts'}
      <ArtifactPane {page} />
    {:else}
      <OutlinePanel {page} {scroller} />
    {/if}
  </div>
</aside>

<style>
  .side {
    position: relative;
    flex: none;
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-left: 1px solid var(--border);
    background: var(--bg);
  }
  .tabs {
    display: flex;
    gap: var(--space-1);
    padding: var(--space-4) var(--space-3) 0 var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .tabs button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-bottom: -1px;
    padding: var(--space-1) var(--space-2) var(--space-2);
    border: 0;
    border-bottom: 2px solid transparent;
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    font-weight: 600;
    white-space: nowrap;
    transition: color var(--fast) var(--ease), border-color var(--fast) var(--ease);
  }
  .tabs button:hover {
    color: var(--text);
  }
  .tabs button.on {
    border-bottom-color: var(--accent);
    color: var(--text);
  }
  .name.file {
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .none {
    margin: var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .badge {
    min-width: 18px;
    padding: 0 5px;
    border-radius: 999px;
    background: var(--surface-3);
    color: var(--text-muted);
    font-size: 11px;
    line-height: 16px;
    text-align: center;
  }
  .on .badge {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .body {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
</style>
