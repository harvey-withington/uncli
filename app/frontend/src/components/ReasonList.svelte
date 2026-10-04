<script lang="ts">
  import type { TraceReason } from '../lib/api'
  import { promptable, reasonLabel, reasonTone } from '../lib/approvals'
  import ShouldPrompt from './ShouldPrompt.svelte'

  // The reason for each part of a command (or a whole tool use), coloured
  // by what happened: only read (quiet), ran as safe, judged by the
  // decision model (highlighted), prompted, or blocked. With a session, a
  // part UNCLI judged safe offers "This should prompt" on its own line.
  interface Props {
    reasons: TraceReason[]
    label: string
    sessionId?: string
  }

  let { reasons, label, sessionId }: Props = $props()
</script>

<ul class="parts" aria-label={label}>
  {#each reasons as r, i (i)}
    {@const teach = sessionId ? promptable([r]) : []}
    <li class="tone-{reasonTone(r)}" data-by={r.by}>
      {#if r.part}<code>{r.part}</code>{/if}<span class="why">{reasonLabel(r)}</span>
      {#if sessionId && teach.length > 0}<ShouldPrompt {sessionId} classes={teach} />{/if}
    </li>
  {/each}
</ul>

<style>
  .parts {
    display: flex;
    flex-direction: column;
    gap: 3px;
    margin: 0;
    padding: 6px 8px;
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    list-style: none;
    font-family: var(--font);
    font-size: var(--text-xs);
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px var(--space-2);
    padding: 1px 0 1px 8px;
    border-left: 2px solid transparent;
  }
  code {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--text);
    overflow-wrap: anywhere;
  }
  .why {
    color: var(--text-muted);
  }
  .tone-read .why {
    color: var(--text-faint);
  }
  .tone-judged {
    border-left-color: var(--accent);
  }
  .tone-judged .why {
    color: var(--accent);
  }
  .tone-prompt {
    border-left-color: var(--warning);
  }
  .tone-prompt .why {
    color: var(--warning);
  }
  .tone-blocked {
    border-left-color: var(--danger);
  }
  .tone-blocked .why {
    color: var(--danger);
  }
</style>
