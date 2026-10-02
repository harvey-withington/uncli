<script lang="ts">
  import { slide } from 'svelte/transition'
  import type { TraceItem } from '../lib/api'
  import { t } from '../lib/i18n.svelte'
  import { failureGist } from '../lib/trace'
  import Icon from './Icon.svelte'

  interface Props {
    items: TraceItem[] | null
  }

  let { items: raw }: Props = $props()
  const items = $derived(raw ?? [])
  let open = $state(false)
  let shown = $state<Record<string, boolean>>({}) // rows showing their full output

  const denied = $derived(items.filter(i => i.denied).length)
  const failed = $derived(items.filter(i => i.done && !i.ok && !i.denied).length)
  const running = $derived(items.filter(i => !i.done).length)
</script>

{#if items.length > 0}
  <section class="trace">
    <button class="summary" onclick={() => (open = !open)} aria-expanded={open}>
      <Icon name="wrench" size={14} />
      <span>{t('trace.calls', { n: items.length })}</span>
      {#if running > 0}<span class="pill run"><Icon name="loader" size={12} spin />{t('trace.running', { n: running })}</span>{/if}
      {#if denied > 0}<span class="pill denied"><Icon name="shield-x" size={12} />{t('trace.denied', { n: denied })}</span>{/if}
      {#if failed > 0}<span class="pill failed">{t('trace.failed', { n: failed })}</span>{/if}
      <span class="chev" class:open><Icon name="chevron-right" size={14} /></span>
    </button>
    {#if open}
      <ol transition:slide={{ duration: 160 }}>
        {#each items as it (it.id)}
          <li class:denied={it.denied} class:failed={it.done && !it.ok && !it.denied}>
            <span class="status" aria-hidden="true">
              {#if !it.done}<Icon name="loader" size={12} spin />{:else if it.denied}<Icon name="shield-x" size={12} />{:else if it.ok}<Icon name="check" size={12} />{:else}<Icon name="x" size={12} />{/if}
            </span>
            <span class="line">
              <span class="name" title={it.summary || it.name}>{it.summary || it.name}</span>
              {#if it.approved}<span class="who">{t(it.approved === 'you' ? 'trace.allowedByYou' : it.approved === 'session' ? 'trace.allowedForSession' : 'trace.allowedByRule')}</span>{/if}
              {#if it.output && !it.denied}
                <button class="more" onclick={() => (shown[it.id] = !shown[it.id])} aria-expanded={!!shown[it.id]}>
                  {shown[it.id] ? t('trace.hideOutput') : t('trace.showOutput')}
                </button>
              {/if}
            </span>
            <span class="visually-hidden">{it.denied ? t('trace.wasDenied') : !it.done ? t('trace.isRunning') : it.ok ? t('trace.ok') : t('trace.wasFailed')}</span>
            {#if it.denied && it.output}<span class="why">{it.output}</span>{/if}
            {#if it.done && !it.ok && !it.denied && it.output && !shown[it.id]}<span class="why failed-why">{failureGist(it.output)}</span>{/if}
            {#if shown[it.id] && it.output}<pre class="output">{it.output}</pre>{/if}
          </li>
        {/each}
      </ol>
    {/if}
  </section>
{/if}

<style>
  .trace {
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
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    text-align: left;
    border-radius: var(--radius);
  }
  .summary:hover {
    color: var(--text);
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 1px 8px;
    border-radius: 999px;
    font-size: var(--text-xs);
  }
  .run {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .denied {
    color: var(--danger);
  }
  .pill.denied,
  .pill.failed {
    background: var(--danger-soft);
    color: var(--danger);
  }
  .chev {
    margin-left: auto;
    display: inline-flex;
    transition: transform var(--normal) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  ol {
    margin: 0;
    padding: var(--space-1) var(--space-3) var(--space-3);
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    display: grid;
    grid-template-columns: 18px 1fr;
    gap: 2px var(--space-2);
    padding: 5px 0;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text-muted);
  }
  .status {
    display: inline-flex;
    padding-top: 2px;
    color: var(--success);
  }
  li.denied .status,
  li.failed .status {
    color: var(--danger);
  }
  .line {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    min-width: 0;
  }
  .name {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text);
  }
  .who {
    flex: none;
    padding: 0 6px;
    border-radius: 999px;
    background: var(--accent-soft);
    color: var(--accent);
    font-family: var(--font);
    font-size: 11px;
  }
  .more {
    flex: none;
    margin-left: auto;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-faint);
    font-family: var(--font);
    font-size: 11px;
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .more:hover {
    color: var(--text);
  }
  .failed-why {
    white-space: pre-wrap;
    font-family: var(--mono);
    font-size: 11px;
  }
  .output {
    grid-column: 2;
    margin: 2px 0 0;
    max-height: 260px;
    overflow: auto;
    padding: 6px 8px;
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    color: var(--text);
    font-size: 11px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .why {
    grid-column: 2;
    font-family: var(--font);
    font-size: var(--text-xs);
    color: var(--danger);
  }
</style>
