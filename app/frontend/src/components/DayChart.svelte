<script lang="ts">
  import type { UsageRow } from '../lib/api'
  import { t } from '../lib/i18n.svelte'
  import { compact, measureOf, money, niceMax, type Measure } from '../lib/usage'

  // One column per day (a single series, so no legend: the dashboard's
  // heading says what is plotted). Columns are at most 24 px, square at the
  // baseline with a 4 px rounded top, 2 px of surface between them; hairline
  // gridlines on clean ticks; every day has a hover target the full height
  // of its band, with a tooltip of its value and turns.

  interface Props {
    rows: UsageRow[] // one per day, oldest first, zeros included
    measure: Measure
  }

  let { rows, measure }: Props = $props()
  let width = $state(640)
  let hover = $state<number | null>(null)
  const hovered = $derived(hover === null ? undefined : rows[hover])
  const height = 200
  const pad = { top: 12, right: 8, bottom: 26, left: 52 }
  const plotW = $derived(Math.max(10, width - pad.left - pad.right))
  const plotH = height - pad.top - pad.bottom
  const values = $derived(rows.map(r => measureOf(r, measure)))
  const axis = $derived(niceMax(Math.max(0, ...values)))
  const ticks = $derived(Array.from({ length: Math.round(axis.max / axis.step) + 1 }, (_, i) => i * axis.step))
  const band = $derived(plotW / Math.max(1, rows.length))
  const barW = $derived(Math.max(1, Math.min(24, band - 2)))
  const fmt = (v: number) => (measure === 'cost' ? money(v) : compact(v))
  const y = (v: number) => pad.top + plotH - (v / axis.max) * plotH
  const x = (i: number) => pad.left + i * band + (band - barW) / 2

  // A column: square at the baseline, its top corners rounded (at most 4 px,
  // less when the column is too short or thin for that).
  function column(i: number, v: number): string {
    const h = (v / axis.max) * plotH
    if (h <= 0) return ''
    const r = Math.min(4, barW / 2, h)
    const x0 = x(i)
    const x1 = x0 + barW
    const top = pad.top + plotH - h
    const base = pad.top + plotH
    return `M${x0},${base}V${top + r}Q${x0},${top} ${x0 + r},${top}H${x1 - r}Q${x1},${top} ${x1},${top + r}V${base}Z`
  }

  const label = (key: string) => new Date(`${key}T12:00:00`).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
  // Date labels: the first and last day, and Mondays between when there's room.
  const shown = $derived(rows.map((r, i) => i === 0 || i === rows.length - 1 || (band * 7 > 46 && new Date(`${r.key}T12:00:00`).getDay() === 1 && i > 2 && i < rows.length - 3)))
</script>

<div class="chart" bind:clientWidth={width}>
  <svg {width} {height} role="img" aria-label={t('dash.chartLabel', { what: t(`dash.measure.${measure}`), n: rows.length })}>
    {#each ticks as v (v)}
      <line class="grid" x1={pad.left} x2={pad.left + plotW} y1={y(v)} y2={y(v)} />
      <text class="tick" x={pad.left - 8} y={y(v)} dy="0.32em" text-anchor="end">{fmt(v)}</text>
    {/each}
    {#each rows as r, i (r.key)}
      {#if values[i]}<path class="bar" class:dim={hover !== null && hover !== i} d={column(i, values[i] ?? 0)} />{/if}
      {#if shown[i]}
        <text class="tick" x={x(i) + barW / 2} y={height - 8} text-anchor={i === 0 ? 'start' : i === rows.length - 1 ? 'end' : 'middle'}>{label(r.key)}</text>
      {/if}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <rect class="hit" x={pad.left + i * band} y={pad.top} width={band} height={plotH}
        onpointerenter={() => (hover = i)} onpointerleave={() => (hover = null)} />
    {/each}
  </svg>
  {#if hover !== null && hovered}
    {@const r = hovered}
    <div class="tip" role="status" style:left="{Math.min(Math.max(x(hover) + barW / 2, 80), width - 80)}px" style:top="{Math.max(0, y(values[hover] ?? 0) - 8)}px">
      <strong>{new Date(`${r.key}T12:00:00`).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })}</strong>
      <span>{fmt(values[hover] ?? 0)} · {t(r.turns === 1 ? 'dash.turnOne' : 'dash.turns', { n: r.turns })}</span>
    </div>
  {/if}
</div>

<style>
  .chart {
    position: relative;
    width: 100%;
  }
  svg {
    display: block;
    overflow: visible;
  }
  .grid {
    stroke: var(--border);
    stroke-width: 1;
    shape-rendering: crispEdges;
  }
  .tick {
    fill: var(--text-faint);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }
  .bar {
    fill: var(--chart-bar);
    transition: opacity var(--fast) var(--ease);
  }
  .bar.dim {
    opacity: 0.45;
  }
  .hit {
    fill: transparent;
  }
  .tip {
    position: absolute;
    transform: translate(-50%, -100%);
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    box-shadow: var(--shadow-md);
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
    pointer-events: none;
  }
  .tip strong {
    color: var(--text);
    font-weight: 600;
  }
</style>
