<script lang="ts">
  import { slide } from 'svelte/transition'
  import type { HeadMove, TouchedFile } from '../lib/api'
  import { useApp } from '../lib/context'
  import { artifactPathOf } from '../lib/artifacts'
  import { watchPathOf } from '../lib/watched'
  import { canOpenDefault, editorLabel, HOW_ICON, moveLabel, relPath, splitByMove, splitPath } from '../lib/files'
  import { t } from '../lib/i18n.svelte'
  import { atServer, serverTitle } from '../lib/remoteonly'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // The files a page's turn changed: one row per file, relative to the
  // session folder, with how it changed (created, edited at a line, changed
  // or deleted by a command). A session that links to an IDE opens a file
  // in the user's editor at its line; others open documents with the
  // default app. Every file that still exists can be shown in its folder,
  // and one in the session folder watched on the side panel's File tab
  // (outside it, Watch is off and says why). When the repo's HEAD moved
  // (a checkout, pull…), what commands changed is folded into one line
  // saying so, expandable; the tools' own writes and edits stay listed.

  interface Props {
    files: TouchedFile[] | null | undefined
    sessionId: string
    move?: HeadMove
  }

  let { files: raw, sessionId, move }: Props = $props()
  const app = useApp()
  // Opening a file or its folder happens on the server's screen.
  const here = $derived(atServer(app.backend))
  const files = $derived(raw ?? [])
  const session = $derived(app.sessions.find(s => s.id === sessionId))
  const ideLinks = $derived(!!app.boot?.profiles.find(p => p.id === session?.profileId)?.ideLinks)
  const artifacts = $derived(app.hasArtifacts(sessionId))
  const editor = $derived(editorLabel(app.boot?.preferences, app.boot?.editors ?? []))
  let open = $state<boolean | null>(null) // null: open while the list is short
  const parts = $derived(splitByMove(files, move))
  // Open while the rows are few (a HEAD move's files count as one).
  const shown = $derived(open ?? parts.own.length + (parts.moved.length ? 1 : 0) <= 6)
  let movedOpen = $state(false)

  function openLabel(f: TouchedFile): string {
    if (ideLinks && editor) return t('files.openIn', { editor })
    return canOpenDefault(f.path) ? t('files.open') : ''
  }

  async function run(p: Promise<void>) {
    try {
      await p
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

{#snippet row(f: TouchedFile)}
  {@const rel = relPath(session?.workdir ?? '', f.path)}
  {@const name = splitPath(rel)}
  {@const label = f.how === 'deleted' ? '' : openLabel(f)}
  {@const artifact = artifacts && f.how !== 'deleted' ? artifactPathOf(session?.workdir ?? '', f.path) : null}
  {@const watchable = f.how !== 'deleted' ? watchPathOf(session?.workdir ?? '', f.path) : null}
  <li class={f.how}>
    <span class="how" title={t(`files.how.${f.how}`)}><Icon name={HOW_ICON[f.how]} size={13} /></span>
    {#snippet path()}
      {#if name.dir}<span class="dir">{name.dir}</span>{/if}<span class="name">{name.name}</span>{#if f.line}<span class="line">:{f.line}</span>{/if}
    {/snippet}
    <!-- The name opens the file the way its first action does: an
         artifact in the Artifacts tab, else in the editor or app. -->
    {#if artifact}
      <button class="path open" title={`${t('files.view')}: ${f.path}`} onclick={() => app.openArtifact(sessionId, artifact)}>{@render path()}</button>
    {:else if label && here}
      <button class="path open" title={`${label}: ${f.path}`} onclick={() => run(app.backend.openFile(sessionId, f.path, f.line ?? 0))}>{@render path()}</button>
    {:else}
      <span class="path" title={f.path}>{@render path()}</span>
    {/if}
    {#if f.added || f.removed}
      <span class="counts" title={t('files.lines', { added: f.added ?? 0, removed: f.removed ?? 0 })}>
        {#if f.added}<span class="added">+{f.added}</span>{/if}{#if f.removed}<span class="removed">−{f.removed}</span>{/if}
      </span>
    {/if}
    <span class="visually-hidden">{t(`files.how.${f.how}`)}{#if f.added || f.removed}, {t('files.lines', { added: f.added ?? 0, removed: f.removed ?? 0 })}{/if}</span>
    {#if f.how !== 'deleted'}
      <span class="actions">
        {#if artifact}
          <button class="btn ghost small icon" onclick={() => app.openArtifact(sessionId, artifact)} aria-label={`${t('files.view')}: ${name.name}`} title={t('files.view')}>
            <Icon name="layers" size={13} />
          </button>
        {/if}
        <!-- Off, saying why, for a file outside the session folder:
             the File tab reads only that folder. -->
        <button class="btn ghost small icon" onclick={() => watchable && app.watchFile(sessionId, watchable)} disabled={!watchable}
          aria-label={`${t('files.watch')}: ${name.name}`} title={watchable ? t('files.watch') : t('files.watchOutside')}>
          <Icon name="eye" size={13} />
        </button>
        {#if label}
          <button class="btn ghost small icon" onclick={() => run(app.backend.openFile(sessionId, f.path, f.line ?? 0))} disabled={!here} aria-label={`${label}: ${name.name}`} title={serverTitle(app.backend, label)}>
            <Icon name="external-link" size={13} />
          </button>
        {/if}
        <button class="btn ghost small icon" onclick={() => run(app.backend.revealFile(f.path))} disabled={!here} aria-label={`${t('files.reveal')}: ${name.name}`} title={serverTitle(app.backend, t('files.reveal'))}>
          <Icon name="folder-search" size={13} />
        </button>
      </span>
    {/if}
  </li>
{/snippet}

{#if files.length > 0}
  <section class="files">
    <button class="summary" onclick={() => (open = !shown)} aria-expanded={shown}>
      <Icon name="file-pen" size={14} />
      <span>{files.length === 1 ? t('files.changedOne') : t('files.changed', { n: files.length })}</span>
      <span class="chev" class:open={shown}><Icon name="chevron-right" size={14} /></span>
    </button>
    {#if shown}
      <ul transition:slide={{ duration: 160 }}>
        {#each parts.own as f (f.path)}{@render row(f)}{/each}
        {#if move && parts.moved.length}
          <li class="move">
            <button class="fold" onclick={() => (movedOpen = !movedOpen)} aria-expanded={movedOpen} title={t('files.move.hint')}>
              <span class="how"><Icon name="git-branch" size={13} /></span>
              <span class="label">{moveLabel(move, parts.moved.length)}</span>
              <span class="chev" class:open={movedOpen}><Icon name="chevron-right" size={13} /></span>
            </button>
          </li>
          {#if movedOpen}
            {#each parts.moved as f (f.path)}{@render row(f)}{/each}
          {/if}
        {/if}
      </ul>
    {/if}
  </section>
{/if}

<style>
  .files {
    margin-top: var(--space-5);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
  }
  .summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    border: 0;
    border-radius: var(--radius);
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    text-align: left;
  }
  .summary:hover {
    color: var(--text);
  }
  .chev {
    margin-left: auto;
    display: inline-flex;
    transition: transform var(--normal) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  ul {
    margin: 0;
    padding: var(--space-1) var(--space-2) var(--space-2);
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 28px;
    padding: 0 var(--space-1);
    border-radius: var(--radius-sm);
    font-family: var(--mono);
    font-size: 12px;
  }
  li:hover {
    background: var(--surface-2);
  }
  .how {
    display: inline-flex;
    flex: none;
    color: var(--accent);
  }
  .write .how {
    color: var(--success);
  }
  .command .how {
    color: var(--text-muted);
  }
  .deleted .how {
    color: var(--danger);
  }
  .path {
    flex: 0 1 auto; /* the counts follow the name; the actions go right */
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl; /* when cut, keep the file name and cut the folders */
    text-align: left;
  }
  .path > span {
    direction: ltr;
    unicode-bidi: embed;
  }
  button.path {
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    cursor: pointer;
  }
  button.path:hover .name,
  button.path:focus-visible .name {
    color: var(--accent);
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .dir,
  .line {
    color: var(--text-faint);
  }
  .name {
    color: var(--text);
  }
  .deleted .name {
    color: var(--text-muted);
    text-decoration: line-through;
  }
  .counts {
    display: inline-flex;
    flex: none;
    gap: 6px;
    font-size: 11.5px;
  }
  .added {
    color: var(--success);
  }
  .removed {
    color: var(--danger);
  }
  /* The line a HEAD move's files fold into. */
  .fold {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-muted);
    font-family: var(--font); /* a sentence, not a path */
    font-size: var(--text-xs);
    text-align: left;
  }
  .fold:hover {
    color: var(--text);
  }
  .fold .label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .move .how {
    color: var(--text-muted);
  }
  .actions {
    display: inline-flex;
    flex: none;
    margin-left: auto;
    gap: 2px;
    opacity: 0;
    transition: opacity var(--fast) var(--ease);
  }
  li:hover .actions,
  .actions:focus-within {
    opacity: 1;
  }
  @media (hover: none) {
    .actions {
      opacity: 1;
    }
  }
</style>
