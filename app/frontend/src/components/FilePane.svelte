<script lang="ts">
  import { untrack } from 'svelte'
  import type { WatchFile } from '../lib/api'
  import { useApp } from '../lib/context'
  import { canOpenDefault, editorLabel, splitPath } from '../lib/files'
  import { relativeTime } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { keepScroll } from '../lib/scrollmemory'
  import { showToast } from '../lib/toasts.svelte'
  import { forget, isMarkdown, langOf, pick, toggleStar } from '../lib/watched'
  import AnswerBlocks from './AnswerBlocks.svelte'
  import CodeBlock from './CodeBlock.svelte'
  import CopyButton from './CopyButton.svelte'
  import FileSwitcher from './FileSwitcher.svelte'
  import Icon from './Icon.svelte'

  // The File tab: one text file from the session folder, read again about
  // once a second while it shows, so it follows the agent's edits (a TODO
  // list, a plan, a log). Markdown is rendered; other text shows as code.
  // Where the viewer is in each file holds through updates and restarts,
  // and a view at the end stays at the end (a growing log; see
  // lib/scrollmemory.ts). A file that goes missing or empty for a moment
  // (an editor or agent rewriting it) keeps its last text until that has
  // lasted GRACE_MS. Which file, the recent ones and the starred ones are
  // kept per session (lib/watched.ts).

  interface Props {
    sessionId: string
  }

  let { sessionId: given }: Props = $props()
  // The prop is read through the parent's session object, which is replaced
  // on every state change; as a derived it only changes with the id, so the
  // file isn't reloaded (and the tab doesn't flicker) as a turn runs.
  const sessionId = $derived(given)
  const app = useApp()
  const POLL_MS = 1000
  const GRACE_MS = 2500
  const watched = $derived(app.watchedFor(sessionId))
  const current = $derived(watched.current)
  const parts = $derived(splitPath(current))
  const starred = $derived(watched.starred.includes(current))
  const session = $derived(app.sessions.find(s => s.id === sessionId))
  const ideLinks = $derived(!!app.boot?.profiles.find(p => p.id === session?.profileId)?.ideLinks)
  const editor = $derived(editorLabel(app.boot?.preferences, app.boot?.editors ?? []))
  const openLabel = $derived(ideLinks && editor ? t('files.openIn', { editor }) : canOpenDefault(current) ? t('files.open') : '')
  let file = $state<WatchFile | null>(null)
  let error = $state('')
  let switching = $state(false)
  let now = $state(Date.now())

  // The palette's "Watch a file…" asks for the switcher: over the file, or
  // with no file chosen (when it's the tab's content, and only takes the
  // focus when asked for) in its place.
  let picks = $state(0)
  $effect(() => {
    if (!app.fileSwitcher) return
    app.fileSwitcher = false
    if (untrack(() => current)) switching = true
    else picks++
  })

  // Read the file, then again each second; only a change comes back.
  $effect(() => {
    const sid = sessionId
    const path = current
    file = null
    error = ''
    if (!path) return
    let stopped = false
    let busy = false
    let last: WatchFile | null = null
    let gone = 0 // when the file went missing or empty, while its last text still shows
    const read = async () => {
      if (busy || (typeof document !== 'undefined' && document.hidden)) return
      busy = true
      try {
        const since = !last ? 0 : last.missing ? -1 : last.modified
        const next = await app.backend.watchRead(sid, path, since)
        if (stopped) return
        now = Date.now()
        error = ''
        if (!next) {
          gone = 0
          return
        }
        // Missing or emptied while it had text: likely mid-rewrite; give it a moment.
        if (last?.text && (next.missing || (!next.text && !next.binary && !next.tooBig))) {
          gone ||= Date.now()
          if (Date.now() - gone < GRACE_MS) return
        }
        gone = 0
        last = file = next
      } catch (e) {
        if (!stopped) error = String(e)
      } finally {
        busy = false
      }
    }
    void read()
    const timer = setInterval(read, POLL_MS)
    return () => {
      stopped = true
      clearInterval(timer)
    }
  })

  function choose(path: string) {
    switching = false
    app.setWatched(sessionId, pick(watched, path))
  }

  // abs is the file's full path, in the folder's own separators.
  function abs(): string {
    const dir = session?.workdir ?? ''
    const sep = dir.includes('\\') ? '\\' : '/'
    return dir.replace(/[\\/]+$/, '') + sep + current.split('/').join(sep)
  }

  async function run(p: Promise<void>) {
    try {
      await p
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

<div class="pane">
  {#if current}
    <div class="bar">
      <button class="pick" onclick={() => (switching = !switching)} aria-expanded={switching} aria-label={`${t('watch.switch')}: ${current}`} title={`${t('watch.switch')}: ${current}`}>
        <Icon name={isMarkdown(current) ? 'file-text' : 'file-code'} size={14} />
        <span class="name">{parts.name}</span>
        {#if parts.dir}<span class="dir">{parts.dir.slice(0, -1)}</span>{/if}
        <Icon name="chevron-down" size={13} />
      </button>
      <span class="gap"></span>
      <button class="btn ghost small icon" class:on={starred} aria-pressed={starred}
        onclick={() => app.setWatched(sessionId, toggleStar(watched, current))}
        aria-label={t(starred ? 'watch.unstar' : 'watch.star')} title={t(starred ? 'watch.unstar' : 'watch.star')}>
        <Icon name="star" size={14} fill={starred} />
      </button>
      {#if file?.text}<CopyButton text={file.text} label={t('watch.copy')} />{/if}
      {#if openLabel && !file?.missing}
        <button class="btn ghost small icon" onclick={() => run(app.backend.openFile(sessionId, abs(), 0))} aria-label={openLabel} title={openLabel}><Icon name="external-link" size={14} /></button>
      {/if}
      {#if !file?.missing}
        <button class="btn ghost small icon" onclick={() => run(app.backend.revealFile(abs()))} aria-label={t('files.reveal')} title={t('files.reveal')}><Icon name="folder-search" size={14} /></button>
      {/if}
      <span class="sep" aria-hidden="true"></span>
      <button class="btn ghost small icon" onclick={() => app.closeFile(sessionId)} aria-label={t('watch.close')} title={t('watch.close')}><Icon name="x" size={14} /></button>
    </div>
    {#if file && !file.missing}
      <p class="when">{t('watch.updated', { when: relativeTime(file.modified, now) })}</p>
    {/if}
    {#if switching}
      <div class="float">
        <FileSwitcher {sessionId} overlay onpick={choose} onclose={() => (switching = false)} />
      </div>
    {/if}
  {/if}
  <div class="body" use:keepScroll={{ key: current ? `file:${sessionId}:${current}` : '' }}>
    {#if !current}
      <p class="intro">{t('watch.intro')}</p>
      <FileSwitcher {sessionId} focus={picks} onpick={choose} />
    {:else if error}
      <p class="note err" role="alert"><Icon name="triangle-alert" size={14} />{error}</p>
    {:else if !file}
      <p class="note" role="status"><Icon name="loader" spin size={14} />{t('artifacts.loading')}</p>
    {:else if file.missing}
      <p class="note">
        <Icon name="file-x" size={14} /><span>{t('watch.missing', { path: current })}</span>
        <button class="btn ghost small" onclick={() => app.setWatched(sessionId, forget(watched, current))}>{t('watch.forget')}</button>
      </p>
    {:else if file.binary}
      <p class="note">{t('watch.binary', { path: current })}</p>
    {:else if file.tooBig}
      <p class="note">{t('watch.tooBig', { path: current })}</p>
    {:else if isMarkdown(current)}
      <div class="doc"><AnswerBlocks markdown={file.text} /></div>
    {:else}
      <div class="doc"><CodeBlock code={file.text} lang={langOf(current)} /></div>
    {/if}
  </div>
</div>

<style>
  .pane {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-2) 0 var(--space-3);
    min-height: 36px;
  }
  .pick {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    max-width: 100%;
    padding: 3px var(--space-2);
    margin-left: calc(-1 * var(--space-2));
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    white-space: nowrap;
  }
  .pick:hover,
  .pick[aria-expanded='true'] {
    border-color: var(--border);
    background: var(--surface-2);
    color: var(--text);
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text);
    font-weight: 600;
  }
  .dir {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .gap {
    flex: 1;
  }
  .sep {
    width: 1px;
    height: 16px;
    margin: 0 2px;
    background: var(--border);
  }
  .on {
    color: var(--warning);
  }
  /* The switcher over the file: anchored to the panel's right edge, the
     panel's width or 560 px if that's wider (it then reaches left, over
     the answer, never right, out of the window), and never wider than
     the window. */
  .float {
    position: absolute;
    top: 38px;
    right: var(--space-2);
    z-index: 5;
    display: flex;
    width: max(calc(100% - 2 * var(--space-2)), 560px);
    max-width: calc(100vw - 2 * var(--space-4));
    max-height: min(420px, 70%);
  }
  .when {
    margin: 0;
    padding: 0 var(--space-3) var(--space-2);
    border-bottom: 1px solid var(--border);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .body {
    position: relative; /* clips absolutely positioned descendants, and holds the switcher */
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: auto;
  }
  .doc {
    padding: var(--space-3) var(--space-4);
  }
  .doc :global(.code) {
    margin: 0;
  }
  .intro {
    margin: var(--space-3) var(--space-3) var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .note {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin: var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .err {
    color: var(--danger);
  }
</style>
