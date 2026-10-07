<script lang="ts">
  import type { UsageReport, UsageRow } from '../lib/api'
  import { useApp } from '../lib/context'
  import { duration } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { modelLabel } from '../lib/models'
  import { tintStyle } from '../lib/tint'
  import { allTokens, compact, fillDays, measureOf, money, periodSince, startOfDay, type Measure, type Period } from '../lib/usage'
  import DayChart from './DayChart.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'
  import UsageBreakdown from './UsageBreakdown.svelte'

  // The usage dashboard: what this computer's pages recorded, summed over a
  // period. Cost leads (the one hero number), then turns, tokens, time and
  // sessions; a chart (or table) by day; and breakdowns by model, session
  // type and session. Costs are the CLI's, at API prices.

  const app = useApp()
  let period = $state<Period>('30d')
  let measure = $state<Measure>('cost')
  let table = $state(false)
  let report = $state<UsageReport | null>(null)
  let failed = $state('')

  $effect(() => {
    const since = periodSince(period)
    let live = true
    app.backend.usageReport(since ? { since } : {}).then(r => { if (live) { report = r; failed = '' } }).catch(e => { if (live) failed = String(e) })
    return () => { live = false }
  })

  const days = $derived.by(() => {
    if (!report) return []
    const from = periodSince(period) ?? (report.first ? startOfDay(report.first) : Date.now())
    return fillDays(report.byDay, from, Date.now())
  })
  const fmt = (v: number) => (measure === 'cost' ? money(v) : compact(v))
  const profile = (id: string) => app.boot?.profiles.find(p => p.id === id)
  const pick = (r: UsageRow) => {
    if (!app.sessions.some(s => s.id === r.key)) return
    app.usageOpen = false
    void app.select(r.key)
  }
  const segments: [Period, string][] = [['7d', 'dash.period.7d'], ['30d', 'dash.period.30d'], ['all', 'dash.period.all']]
  const measures: [Measure, string][] = [['cost', 'dash.measure.cost'], ['tokens', 'dash.measure.tokens']]
</script>

<Modal title={t('dash.title')} width={880} onclose={() => (app.usageOpen = false)}>
  <div class="controls">
    <div class="seg" role="radiogroup" aria-label={t('dash.period')}>
      {#each segments as [p, key] (p)}
        <button role="radio" aria-checked={period === p} class:on={period === p} onclick={() => (period = p)}>{t(key)}</button>
      {/each}
    </div>
    <span class="gap"></span>
    <div class="seg" role="radiogroup" aria-label={t('dash.measure')}>
      {#each measures as [m, key] (m)}
        <button role="radio" aria-checked={measure === m} class:on={measure === m} onclick={() => (measure = m)}>{t(key)}</button>
      {/each}
    </div>
    <button class="btn ghost small" onclick={() => (table = !table)} aria-pressed={table}>
      <Icon name={table ? 'bar-chart' : 'table'} size={14} />{t(table ? 'dash.asChart' : 'dash.asTable')}
    </button>
  </div>

  {#if failed}
    <p class="err" role="alert"><Icon name="triangle-alert" size={14} />{failed}</p>
  {:else if !report}
    <p class="muted" role="status"><Icon name="loader" spin size={14} />{t('dash.loading')}</p>
  {:else}
    <div class="tiles">
      <div class="tile hero"><span class="label">{t('dash.cost')}</span><span class="value">{money(report.totals.costUsd)}</span></div>
      <div class="tile"><span class="label">{t('dash.turnsLabel')}</span><span class="value">{compact(report.totals.turns)}</span></div>
      <div class="tile"><span class="label">{t('dash.tokens')}</span><span class="value">{compact(allTokens(report.totals))}</span><span class="sub">{t('dash.tokensSplit', { out: compact(report.totals.output), cached: compact(report.totals.cacheRead) })}</span></div>
      <div class="tile"><span class="label">{t('dash.time')}</span><span class="value">{duration(report.totals.durationMs)}</span></div>
      <div class="tile"><span class="label">{t('dash.sessions')}</span><span class="value">{report.sessions}</span></div>
    </div>

    <h3>{t('dash.byDay', { what: t(`dash.measure.${measure}`) })}</h3>
    {#if table}
      <div class="tablewrap">
        <table>
          <thead><tr><th>{t('dash.day')}</th><th>{t('dash.cost')}</th><th>{t('dash.tokens')}</th><th>{t('dash.turnsLabel')}</th></tr></thead>
          <tbody>
            {#each days.filter(d => d.turns > 0).reverse() as d (d.key)}
              <tr><td>{d.key}</td><td>{money(d.costUsd)}</td><td>{compact(allTokens(d))}</td><td>{d.turns}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <DayChart rows={days} {measure} />
    {/if}

    <div class="breakdowns">
      <UsageBreakdown title={t('dash.byModel')} rows={report.byModel} {measure} empty={t('dash.none')}
        name={r => modelLabel(app.boot?.models ?? null, r.key) || r.key} />
      <UsageBreakdown title={t('dash.byType')} rows={report.byProfile} {measure} empty={t('dash.none')}
        name={r => profile(r.key)?.label ?? r.key} icon={r => ({ name: profile(r.key)?.icon ?? 'message-circle', style: tintStyle(profile(r.key)?.hue) })} />
      <UsageBreakdown title={t('dash.bySession')} rows={report.bySession} {measure} empty={t('dash.none')}
        name={r => app.sessions.find(s => s.id === r.key)?.title || r.label || t('session.untitled')}
        icon={r => { const s = app.sessions.find(x => x.id === r.key); const p = profile(s?.profileId ?? ''); return p ? { name: p.icon, style: tintStyle(p.hue) } : null }}
        onpick={pick} />
    </div>

    <p class="foot">
      {t('dash.quickTasks', { n: report.quickTasks, cost: money(report.quickTaskCostUsd) })}
      {t('dash.apiPrices')}
    </p>
  {/if}
  <span class="visually-hidden" aria-live="polite">{report ? t('dash.summary', { cost: money(report.totals.costUsd), what: fmt(measureOf(report.totals, measure)) }) : ''}</span>
</Modal>

<style>
  .controls {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-4);
  }
  .gap {
    flex: 1;
  }
  .seg {
    display: inline-flex;
    gap: 2px;
    padding: 2px;
    border-radius: var(--radius-sm);
    background: var(--surface-2);
  }
  .seg button {
    padding: 3px 12px;
    border: 0;
    border-radius: 4px;
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .seg button.on {
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-sm);
  }
  .tiles {
    display: grid;
    grid-template-columns: 1.5fr repeat(4, 1fr);
    gap: var(--space-3);
    margin-bottom: var(--space-5);
  }
  .tile {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .label {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .value {
    font-size: var(--text-xl);
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .hero .value {
    font-size: 48px;
    line-height: 1.05;
    letter-spacing: -0.02em;
  }
  .sub {
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  h3 {
    margin: 0 0 var(--space-2);
    font-size: var(--text-sm);
    font-weight: 600;
  }
  .tablewrap {
    max-height: 220px;
    overflow: auto;
    position: relative;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }
  th,
  td {
    padding: 4px 8px;
    border-bottom: 1px solid var(--border);
    text-align: right;
  }
  th:first-child,
  td:first-child {
    text-align: left;
  }
  th {
    position: sticky;
    top: 0;
    background: var(--surface);
    color: var(--text-muted);
    font-weight: 600;
  }
  .breakdowns {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-5);
    margin-top: var(--space-5);
  }
  .foot {
    margin: var(--space-5) 0 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .muted,
  .err {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .err {
    color: var(--danger);
  }
</style>
