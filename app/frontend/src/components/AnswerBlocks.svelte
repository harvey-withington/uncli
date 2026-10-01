<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import type { OutlineEntry } from '../lib/outline'
  import { toBlocks } from '../lib/render/markdown'
  import { KIND_ICONS } from '../lib/sections'
  import CodeBlock from './CodeBlock.svelte'
  import CopyButton from './CopyButton.svelte'
  import Icon from './Icon.svelte'

  interface Props {
    markdown: string
    streaming?: boolean
    markers?: Map<number, OutlineEntry> // block index → the outline entry starting there
  }

  let { markdown, streaming = false, markers }: Props = $props()
  const app = useApp()
  const blocks = $derived(toBlocks(markdown))

  // Links open in the user's browser, never inside the app window.
  function onclick(e: MouseEvent) {
    const a = (e.target as HTMLElement).closest('a')
    if (!a) return
    e.preventDefault()
    const href = a.getAttribute('href')
    if (href) app.backend.openURL(href)
  }
</script>

<!-- The outline's section kind, in the page's left margin beside the block
     the section starts at. -->
{#snippet marker(m: OutlineEntry)}
  <span class="marker" style:color="var(--kind-{m.kind})" title={`${t(`kind.${m.kind}`)}: ${m.text}`} aria-hidden="true">
    <Icon name={KIND_ICONS[m.kind]} size={16} />
  </span>
{/snippet}

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="answer" {onclick}>
  {#each blocks as b, i (b.key)}
    {@const m = markers?.get(i)}
    {#if b.kind === 'code'}
      <div class="code" data-block={i}>
        {#if m}{@render marker(m)}{/if}
        <CodeBlock code={b.code ?? ''} lang={b.lang ?? ''} streaming={streaming && i === blocks.length - 1} />
      </div>
    {:else}
      <div class="block" data-block={i}>
        {#if m}{@render marker(m)}{/if}
        <div class="md">{@html b.html}</div>
        <span class="copy-md"><CopyButton text={b.source} label={t('copy.markdown')} /></span>
      </div>
    {/if}
  {/each}
  {#if streaming}<span class="caret" aria-hidden="true"></span>{/if}
</div>

<style>
  .answer {
    font-size: var(--text-lg);
    line-height: 1.68;
    color: var(--text);
  }
  .block {
    position: relative;
    margin: 0 calc(-1 * var(--space-3));
    padding: 0 var(--space-3);
    border-radius: var(--radius-sm);
    transition: background var(--fast) var(--ease);
  }
  .block:hover {
    background: var(--surface-2);
  }
  .code {
    position: relative;
  }
  /* In the page's left padding, centred on the block's first line (text
     blocks are widened by their hover padding, so they sit further out). */
  .marker {
    position: absolute;
    top: calc((1lh - 16px) / 2);
    right: calc(100% + 10px);
    display: inline-flex;
    opacity: 0.9;
  }
  .block > .marker {
    right: calc(100% + 10px - var(--space-3));
  }
  .code > .marker {
    top: 10px;
  }
  .copy-md {
    position: absolute;
    top: 4px;
    right: 4px;
    opacity: 0;
    transition: opacity var(--fast) var(--ease);
  }
  .block:hover .copy-md,
  .copy-md:focus-within,
  .copy-md:has(:global(.copied)) {
    opacity: 1;
  }
  .md :global(> :first-child) {
    margin-top: 0;
  }
  .md :global(p),
  .md :global(ul),
  .md :global(ol),
  .md :global(blockquote),
  .md :global(table) {
    margin: 0 0 var(--space-4);
  }
  .block:last-child .md :global(> :last-child) {
    margin-bottom: 0;
  }
  .md :global(h1),
  .md :global(h2),
  .md :global(h3),
  .md :global(h4) {
    margin: var(--space-5) 0 var(--space-2);
    line-height: 1.3;
    letter-spacing: -0.012em;
    font-weight: 650;
  }
  .md :global(h1) { font-size: 1.45em; }
  .md :global(h2) { font-size: 1.22em; }
  .md :global(h3) { font-size: 1.06em; }
  .md :global(ul),
  .md :global(ol) {
    padding-left: 1.4em;
  }
  .md :global(li + li) {
    margin-top: 0.25em;
  }
  .md :global(li::marker) {
    color: var(--text-faint);
  }
  .md :global(code) {
    padding: 0.12em 0.36em;
    border-radius: 5px;
    background: var(--surface-3);
    font-size: 0.86em;
  }
  .md :global(a) {
    color: var(--accent);
    text-decoration: underline;
    text-decoration-thickness: 1px;
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .md :global(blockquote) {
    padding: var(--space-1) var(--space-4);
    border-left: 3px solid var(--accent);
    color: var(--text-muted);
  }
  .md :global(blockquote p) {
    margin: 0;
  }
  .md :global(table) {
    border-collapse: collapse;
    font-size: 0.92em;
    display: block;
    overflow-x: auto;
  }
  .md :global(th),
  .md :global(td) {
    padding: 6px 14px 6px 0;
    border-bottom: 1px solid var(--border);
    text-align: left;
  }
  .md :global(th) {
    font-weight: 600;
    color: var(--text-muted);
    font-size: 0.9em;
  }
  .md :global(hr) {
    border: 0;
    border-top: 1px solid var(--border);
    margin: var(--space-5) 0;
  }
  .caret {
    display: inline-block;
    width: 8px;
    height: 1.1em;
    margin-left: 2px;
    vertical-align: text-bottom;
    border-radius: 2px;
    background: var(--accent);
    animation: blink 1s steps(2) infinite;
  }
  @keyframes blink {
    50% { opacity: 0; }
  }
</style>
