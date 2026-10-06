<script lang="ts">
  import type { ArtifactEntry } from '../lib/artifacts'
  import { artifactKind, artifactLang, decodeText, imageType, sandboxDoc, svgDoc } from '../lib/artifacts'
  import { useApp } from '../lib/context'
  import { canOpenDefault } from '../lib/files'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import AnswerBlocks from './AnswerBlocks.svelte'
  import CodeBlock from './CodeBlock.svelte'
  import CopyButton from './CopyButton.svelte'
  import Icon from './Icon.svelte'

  // One artifact, shown by kind. HTML runs in an iframe with an opaque
  // origin (sandbox without allow-same-origin) and a policy that allows no
  // network, so it can't reach UNCLI, the bridge or the web; SVG and
  // Mermaid diagrams show the same way with no scripts at all. Markdown
  // goes through the app's renderer (raw HTML off), text through Shiki.

  interface Props {
    sessionId: string
    entry: ArtifactEntry
  }

  let { sessionId, entry }: Props = $props()
  const app = useApp()
  const kind = $derived(artifactKind(entry.path))
  const name = $derived(entry.path.slice(entry.path.lastIndexOf('/') + 1))
  let data = $state<string | null>(null) // base64
  let text = $state('')
  let diagram = $state('') // a rendered Mermaid diagram (SVG)
  let error = $state('')
  let loading = $state(false)
  let seq = 0

  $effect(() => {
    const e = entry
    const k = kind
    const mine = ++seq
    data = null
    text = diagram = error = ''
    if (k === 'pdf' || k === 'other') return
    loading = true
    app.backend
      .readArtifact(sessionId, e.hash ? { hash: e.hash } : { path: e.path })
      .then(async c => {
        if (mine !== seq) return
        data = c.data
        if (k !== 'image') text = decodeText(c.data)
        if (k === 'mermaid') diagram = await renderMermaid(text, mine)
      })
      .catch(err => {
        if (mine === seq) error = String(err)
      })
      .finally(() => {
        if (mine === seq) loading = false
      })
  })

  // Mermaid is big, so it loads the first time a diagram is shown. Its
  // strict level escapes labels and turns off clicks and scripts.
  async function renderMermaid(src: string, mine: number): Promise<string> {
    const { default: mermaid } = await import('mermaid')
    mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'default' })
    const { svg } = await mermaid.render(`uncli-mermaid-${mine}`, src)
    return svg
  }

  async function onDisk(): Promise<string | null> {
    try {
      return await app.backend.artifactPath(sessionId, entry.path)
    } catch (e) {
      showToast(String(e), 'error')
      return null
    }
  }
  async function open() {
    const p = await onDisk()
    if (p) app.backend.openFile(sessionId, p, 0).catch(e => showToast(String(e), 'error'))
  }
  async function reveal() {
    const p = await onDisk()
    if (p) app.backend.revealFile(p).catch(e => showToast(String(e), 'error'))
  }
</script>

<div class="viewer">
  <div class="bar">
    <span class="name" title={entry.path}>{name}</span>
    <span class="from">{entry.live ? t('artifacts.fromFolder') : t('artifacts.fromPage', { n: entry.seq })}</span>
    <span class="gap"></span>
    {#if text && kind !== 'image'}<CopyButton {text} label={t('artifacts.copy')} />{/if}
    {#if canOpenDefault(entry.path)}
      <button class="btn ghost small icon" onclick={open} aria-label={t('artifacts.open')} title={t('artifacts.open')}><Icon name="external-link" size={14} /></button>
    {/if}
    <button class="btn ghost small icon" onclick={reveal} aria-label={t('files.reveal')} title={t('files.reveal')}><Icon name="folder-search" size={14} /></button>
  </div>
  <div class="body" class:framed={kind === 'html' || kind === 'svg' || kind === 'mermaid'}>
    {#if error}
      <p class="note err" role="alert"><Icon name="triangle-alert" size={14} />{error}</p>
    {:else if loading}
      <p class="note" role="status"><Icon name="loader" spin size={14} />{t('artifacts.loading')}</p>
    {:else if kind === 'html' && text}
      <iframe title={name} sandbox="allow-scripts" srcdoc={sandboxDoc(text)}></iframe>
    {:else if kind === 'svg' && text}
      <iframe title={name} sandbox="" srcdoc={svgDoc(text, '#ffffff')}></iframe>
    {:else if kind === 'mermaid' && diagram}
      <iframe title={name} sandbox="" srcdoc={svgDoc(diagram, '#ffffff')}></iframe>
    {:else if kind === 'markdown'}
      <div class="doc"><AnswerBlocks markdown={text} /></div>
    {:else if kind === 'image' && data}
      <div class="picture"><img src={`data:${imageType(entry.path)};base64,${data}`} alt={name} /></div>
    {:else if kind === 'text'}
      <div class="doc"><CodeBlock code={text} lang={artifactLang(entry.path)} /></div>
    {:else}
      <p class="note">{t('artifacts.noPreview')}</p>
    {/if}
  </div>
</div>

<style>
  .viewer {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--border);
    min-height: 40px;
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
    font-size: var(--text-sm);
  }
  .from {
    flex: none;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .gap {
    flex: 1;
  }
  .body {
    position: relative; /* clips absolutely positioned descendants (as .scroll in SessionPane) */
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .body.framed {
    overflow: hidden;
    background: #ffffff; /* artifacts are made for a page, so they get one */
  }
  iframe {
    display: block;
    width: 100%;
    height: 100%;
    border: 0;
  }
  .doc {
    padding: var(--space-3) var(--space-4);
  }
  .picture {
    display: grid;
    place-items: center;
    min-height: 100%;
    padding: var(--space-4);
    background: var(--surface-2);
  }
  .picture img {
    max-width: 100%;
    height: auto;
  }
  .note {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .err {
    color: var(--danger);
  }
</style>
