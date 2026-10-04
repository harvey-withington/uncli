<script lang="ts">
  import type { TraceReason } from '../lib/api'
  import { commandLines, shortCommand } from '../lib/approvals'
  import { t } from '../lib/i18n.svelte'

  // A command on an approval card. A shell command with reasons is coloured
  // part by part, one part a line: separators, each part's class words in
  // its tone (hover for the reason), and the rest quiet. Long quoted text
  // (a commit message) is shortened; past four lines the box scrolls.
  // Under the box, Claude's description of it on the left and "Show the
  // whole command" on the right.
  interface Props {
    command: string
    why?: TraceReason[]
    shell: boolean
    note?: string
  }

  let { command, why = [], shell, note }: Props = $props()
  let whole = $state(false)
  const shortened = $derived(shortCommand(command) !== command)
  const lines = $derived(shell && why.some(r => r.part) ? commandLines(command, why, shortened && !whole) : null)
</script>

<div class="box">
  <code class="target" aria-label={command}>
    {#if lines}
      {#each lines as line, i (i)}<span class="line">{#each line as seg, j (j)}<span class="seg {seg.kind} {seg.tone ? `tone-${seg.tone}` : ''}" title={seg.title}>{seg.text}</span>{/each}</span>{/each}
    {:else}{shortened && !whole ? shortCommand(command) : command}{/if}
  </code>
  {#if note || shortened}
    <div class="under">
      {#if note}<span class="note">{note}</span>{/if}
      {#if shortened}
        <button class="more" onclick={() => (whole = !whole)} aria-expanded={whole}>{t(whole ? 'approval.hideCommand' : 'approval.showCommand')}</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .box {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .target {
    display: block;
    max-height: calc(4 * 1.6em + 12px); /* four lines, then it scrolls */
    overflow-y: auto;
    padding: 6px 10px;
    border-radius: var(--radius-sm);
    background: var(--surface);
    border: 1px solid var(--border);
    font-family: var(--mono);
    font-size: var(--text-sm);
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .line {
    display: block;
  }
  .line + .line {
    padding-left: 1.5ch; /* the rest of a chain, under its first part */
  }
  .seg.rest {
    color: var(--cmd-rest);
  }
  .seg.sep {
    color: var(--cmd-sep);
    font-weight: 700;
  }
  .seg.class {
    font-weight: 650;
  }
  .seg.class.tone-read {
    color: var(--cmd-read);
  }
  .seg.class.tone-ran {
    color: var(--cmd-safe);
  }
  .seg.class.tone-judged {
    color: var(--cmd-judged);
  }
  .seg.class.tone-prompt {
    color: var(--cmd-unsafe);
  }
  .seg.class.tone-blocked {
    color: var(--cmd-blocked);
  }
  .under {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }
  .note {
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .more {
    flex: none;
    margin-left: auto;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--text-xs);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .more:hover {
    color: var(--text);
  }
</style>
