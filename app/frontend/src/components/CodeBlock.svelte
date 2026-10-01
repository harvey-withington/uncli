<script lang="ts">
  import { escapeHtml } from '../lib/render/markdown'
  import { highlight } from '../lib/render/highlight'
  import { t } from '../lib/i18n.svelte'
  import CopyButton from './CopyButton.svelte'

  interface Props {
    code: string
    lang: string
    streaming?: boolean
  }

  let { code, lang, streaming = false }: Props = $props()
  let highlighted = $state<string | null>(null)

  // Highlight once the block is complete; while streaming, plain code
  // avoids re-highlighting on every delta.
  $effect(() => {
    const c = code
    const l = lang
    if (streaming) {
      highlighted = null
      return
    }
    let cancelled = false
    highlight(c, l)
      .then(html => { if (!cancelled) highlighted = html })
      .catch(() => { if (!cancelled) highlighted = null })
    return () => { cancelled = true }
  })
</script>

<div class="code">
  <div class="head">
    <span class="lang">{lang || t('page.code')}</span>
    <CopyButton text={code} label={t('copy.code')} />
  </div>
  {#if highlighted}
    <div class="shiki-wrap">{@html highlighted}</div>
  {:else}
    <pre class="plain"><code>{@html escapeHtml(code)}</code></pre>
  {/if}
</div>

<style>
  .code {
    position: relative;
    margin: var(--space-4) 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-2);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 32px;
    padding: 0 var(--space-1) 0 var(--space-3);
    border-bottom: 1px solid var(--border);
  }
  .lang {
    font-size: var(--text-xs);
    color: var(--text-faint);
    font-family: var(--mono);
  }
  .head :global(.copy) {
    opacity: 0;
    transition: opacity var(--fast) var(--ease);
  }
  .code:hover .head :global(.copy),
  .head :global(.copy:focus-visible),
  .head :global(.copy.copied) {
    opacity: 1;
  }
  .plain,
  .shiki-wrap :global(pre) {
    margin: 0;
    padding: var(--space-3) var(--space-4);
    overflow-x: auto;
    font-size: 12.5px;
    line-height: 1.6;
    background: transparent !important;
  }
  .shiki-wrap :global(span) {
    color: var(--shiki-light);
  }
  :global(:root[data-theme='dark']) .shiki-wrap :global(span) {
    color: var(--shiki-dark);
  }
  @media (prefers-color-scheme: dark) {
    :global(:root:not([data-theme='light'])) .shiki-wrap :global(span) {
      color: var(--shiki-dark);
    }
  }
</style>
