<script lang="ts">
  import { svgDoc } from '../lib/artifacts'
  import { t } from '../lib/i18n.svelte'
  import { renderMermaid, svgSize } from '../lib/mermaid'
  import CodeBlock from './CodeBlock.svelte'
  import Icon from './Icon.svelte'

  // A Mermaid code block in a Markdown document, drawn as its diagram
  // (lib/mermaid.ts), in a frame with no scripts, as tall as its shape asks
  // up to a limit. Code shows its source instead; one Mermaid can't read
  // shows its source with Mermaid's complaint.
  interface Props {
    code: string
  }
  let { code }: Props = $props()
  let svg = $state('')
  let error = $state('')
  let source = $state(false)
  const size = $derived(svgSize(svg))

  $effect(() => {
    const src = code
    let live = true
    svg = error = ''
    renderMermaid(src)
      .then(s => { if (live) svg = s })
      .catch(err => { if (live) error = err instanceof Error ? err.message : String(err) })
    return () => { live = false }
  })
</script>

<div class="mermaid">
  {#if error}
    <p class="note" role="alert"><Icon name="triangle-alert" size={13} />{t('mermaid.failed', { error: error.split('\n')[0] ?? '' })}</p>
    <CodeBlock {code} lang="mermaid" />
  {:else if source}
    <CodeBlock {code} lang="mermaid" />
  {:else if svg}
    <iframe title={t('mermaid.title')} sandbox="" srcdoc={svgDoc(svg, '#ffffff')}
      style:aspect-ratio={size ? `${size.width} / ${size.height}` : undefined} style:max-width={size ? `${Math.max(size.width, 240)}px` : undefined}></iframe>
  {:else}
    <p class="note"><Icon name="loader" spin size={13} />{t('mermaid.drawing')}</p>
  {/if}
  {#if !error}
    <button class="btn ghost small toggle" onclick={() => (source = !source)} aria-pressed={source}>
      <Icon name={source ? 'layers' : 'code'} size={13} />{source ? t('mermaid.diagram') : t('mermaid.source')}
    </button>
  {/if}
</div>

<style>
  .mermaid {
    position: relative;
    margin: 0 0 var(--space-4);
  }
  iframe {
    display: block;
    width: 100%;
    min-height: 120px;
    max-height: 70vh;
    margin: 0 auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: #ffffff; /* the diagram's own page, as Mermaid draws it */
  }
  .note {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    margin: 0 0 var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .note[role='alert'] {
    color: var(--warning);
  }
  .toggle {
    margin-top: var(--space-1);
    color: var(--text-muted);
  }
</style>
