<script lang="ts">
  import type { Snippet } from 'svelte'
  import { fade, scale } from 'svelte/transition'
  import { focusTrap } from '../lib/actions'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  interface Props {
    title: string
    width?: number
    onclose: () => void
    children: Snippet
    footer?: Snippet
  }

  let { title, width = 520, onclose, children, footer }: Props = $props()

  // Escape closes the topmost layer only.
  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      onclose()
    }
  }
</script>

<div class="scrim" transition:fade={{ duration: 120 }} onpointerdown={e => e.target === e.currentTarget && onclose()} role="presentation">
  <div
    class="dialog"
    style:width="{width}px"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    use:focusTrap
    {onkeydown}
    transition:scale={{ duration: 160, start: 0.97 }}
  >
    <header>
      <h2>{title}</h2>
      <button class="btn ghost icon" onclick={onclose} aria-label={t('common.close')}><Icon name="x" /></button>
    </header>
    <div class="body">{@render children()}</div>
    {#if footer}
      <footer>{@render footer()}</footer>
    {/if}
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: grid;
    place-items: center;
    background: var(--scrim);
    padding: var(--space-5);
  }
  .dialog {
    max-width: 100%;
    max-height: calc(100vh - 64px);
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-lg);
    outline: none;
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-4) var(--space-2) var(--space-5);
  }
  h2 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
    letter-spacing: -0.01em;
  }
  .body {
    padding: var(--space-2) var(--space-5) var(--space-5);
    overflow: auto;
  }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-5) var(--space-4);
    border-top: 1px solid var(--border);
  }
</style>
