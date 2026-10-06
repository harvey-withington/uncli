<script lang="ts">
  import { slide } from 'svelte/transition'
  import type { TouchedFile } from '../lib/api'
  import { useApp } from '../lib/context'
  import { artifactPathOf } from '../lib/artifacts'
  import { canOpenDefault, editorLabel, HOW_ICON, relPath, splitPath } from '../lib/files'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // The files a page's turn changed: one row per file, relative to the
  // session folder, with how it changed (created, edited at a line, changed
  // or deleted by a command). A session that links to an IDE opens a file
  // in the user's editor at its line; others open documents with the
  // default app. Every file that still exists can be shown in its folder.

  interface Props {
    files: TouchedFile[] | null | undefined
    sessionId: string
  }

  let { files: raw, sessionId }: Props = $props()
  const app = useApp()
  const files = $derived(raw ?? [])
  const session = $derived(app.sessions.find(s => s.id === sessionId))
  const ideLinks = $derived(!!app.boot?.profiles.find(p => p.id === session?.profileId)?.ideLinks)
  const artifacts = $derived(app.hasArtifacts(sessionId))
  const editor = $derived(editorLabel(app.boot?.preferences, app.boot?.editors ?? []))
  let open = $state<boolean | null>(null) // null: open while the list is short
  const shown = $derived(open ?? files.length <= 6)

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

{#if files.length > 0}
  <section class="files">
    <button class="summary" onclick={() => (open = !shown)} aria-expanded={shown}>
      <Icon name="file-pen" size={14} />
      <span>{files.length === 1 ? t('files.changedOne') : t('files.changed', { n: files.length })}</span>
      <span class="chev" class:open={shown}><Icon name="chevron-right" size={14} /></span>
    </button>
    {#if shown}
      <ul transition:slide={{ duration: 160 }}>
        {#each files as f (f.path)}
          {@const rel = relPath(session?.workdir ?? '', f.path)}
          {@const parts = splitPath(rel)}
          {@const label = f.how === 'deleted' ? '' : openLabel(f)}
          {@const artifact = artifacts && f.how !== 'deleted' ? artifactPathOf(session?.workdir ?? '', f.path) : null}
          <li class={f.how}>
            <span class="how" title={t(`files.how.${f.how}`)}><Icon name={HOW_ICON[f.how]} size={13} /></span>
            {#snippet path()}
              {#if parts.dir}<span class="dir">{parts.dir}</span>{/if}<span class="name">{parts.name}</span>{#if f.line}<span class="line">:{f.line}</span>{/if}
            {/snippet}
            <!-- The name opens the file the way its first action does: an
                 artifact in the Artifacts tab, else in the editor or app. -->
            {#if artifact}
              <button class="path open" title={`${t('files.view')}: ${f.path}`} onclick={() => app.openArtifact(sessionId, artifact)}>{@render path()}</button>
            {:else if label}
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
                  <button class="btn ghost small icon" onclick={() => app.openArtifact(sessionId, artifact)} aria-label={`${t('files.view')}: ${parts.name}`} title={t('files.view')}>
                    <Icon name="layers" size={13} />
                  </button>
                {/if}
                {#if label}
                  <button class="btn ghost small icon" onclick={() => run(app.backend.openFile(sessionId, f.path, f.line ?? 0))} aria-label={`${label}: ${parts.name}`} title={label}>
                    <Icon name="external-link" size={13} />
                  </button>
                {/if}
                <button class="btn ghost small icon" onclick={() => run(app.backend.revealFile(f.path))} aria-label={`${t('files.reveal')}: ${parts.name}`} title={t('files.reveal')}>
                  <Icon name="folder-search" size={13} />
                </button>
              </span>
            {/if}
          </li>
        {/each}
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
