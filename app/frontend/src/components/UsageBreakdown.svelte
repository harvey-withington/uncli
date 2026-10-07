<script lang="ts">
  import type { UsageRow } from '../lib/api'
  import { compact, measureOf, money, type Measure } from '../lib/usage'
  import Icon from './Icon.svelte'

  // A ranked breakdown: each row's name, a magnitude bar (one hue, relative
  // to the largest row) and its value in text, so the bar is never the only
  // way to read it. Rows with onpick are buttons (a session opens).

  interface Props {
    title: string
    rows: UsageRow[]
    measure: Measure
    name: (r: UsageRow) => string
    icon?: (r: UsageRow) => { name: string; style?: string } | null
    onpick?: (r: UsageRow) => void
    empty: string
  }

  let { title, rows, measure, name, icon, onpick, empty }: Props = $props()
  const max = $derived(Math.max(0, ...rows.map(r => measureOf(r, measure))))
  const fmt = (v: number) => (measure === 'cost' ? money(v) : compact(v))
</script>

<section class="breakdown">
  <h4>{title}</h4>
  {#if rows.length === 0}
    <p class="empty">{empty}</p>
  {:else}
    <ul>
      {#each rows as r (r.key)}
        {@const v = measureOf(r, measure)}
        {@const ic = icon?.(r)}
        <li>
          <svelte:element this={onpick ? 'button' : 'div'} class="row" class:pick={!!onpick} onclick={onpick ? () => onpick(r) : undefined} role={onpick ? undefined : 'group'} aria-label={onpick ? undefined : `${name(r)}: ${fmt(v)}`}>
            <span class="name">
              {#if ic}<span class="ic" style={ic.style}><Icon name={ic.name} size={13} /></span>{/if}
              <span class="text" title={name(r)}>{name(r)}</span>
              <span class="value">{fmt(v)}</span>
            </span>
            <span class="track" aria-hidden="true"><span class="fill" style:width="{max > 0 ? Math.max(1, (v / max) * 100) : 0}%"></span></span>
          </svelte:element>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  h4 {
    margin: 0 0 var(--space-2);
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-muted);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .row {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 100%;
    padding: 6px var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    text-align: left;
    font: inherit;
  }
  .row.pick:hover,
  .row.pick:focus-visible {
    background: var(--surface-2);
  }
  .name {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: var(--text-sm);
    color: var(--text);
  }
  .ic {
    display: inline-flex;
    color: var(--tint, var(--text-muted));
  }
  .text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .value {
    flex: none;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .track {
    display: block;
    height: 6px;
  }
  .fill {
    display: block;
    height: 100%;
    border-radius: 0 3px 3px 0;
    background: var(--chart-bar);
  }
  .empty {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-faint);
  }
</style>
