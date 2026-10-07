<script lang="ts">
  import { fly } from 'svelte/transition'
  import { clickOutside } from '../lib/actions'
  import type { UsageLimit } from '../lib/api'
  import { resetsIn } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { useApp } from '../lib/context'
  import Icon from './Icon.svelte'

  interface Props {
    usage: UsageLimit | null
    onclose: () => void
  }

  let { usage, onclose }: Props = $props()
  const app = useApp()
  const windows = $derived(Object.entries(usage?.windows ?? {}).sort(([a], [b]) => a.localeCompare(b)))
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="pop" use:clickOutside={onclose} onkeydown={e => e.key === 'Escape' && onclose()} transition:fly={{ y: -4, duration: 140 }} role="dialog" aria-label={t('usage.title')} tabindex="-1">
  <h3>{t('usage.title')}</h3>
  {#if windows.length === 0}
    <p class="none">{t('usage.none')}</p>
  {:else}
    {#each windows as [name, w] (name)}
      {@const pct = Math.round(w.utilization * 100)}
      <div class="win">
        <div class="row">
          <span>{t(`usage.window.${name}`)}</span>
          <span class="pct">{t('usage.used', { pct })}</span>
        </div>
        <div class="bar" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label={t(`usage.window.${name}`)}>
          <span style:width="{Math.min(100, pct)}%" class:high={pct >= 80}></span>
        </div>
        <span class="reset">{t('usage.resets', { when: resetsIn(w.resetsAt) })}</span>
      </div>
    {/each}
  {/if}
  <button class="over-time" onclick={() => { onclose(); app.usageOpen = true }}><Icon name="bar-chart" size={13} />{t('dash.open')}</button>
</div>

<style>
  .pop {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 20;
    width: 280px;
    padding: var(--space-4);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    outline: none;
  }
  .over-time {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-top: var(--space-4);
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font-size: var(--text-sm);
  }
  .over-time:hover {
    text-decoration: underline;
  }
  h3 {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
  }
  .none {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .win + .win {
    margin-top: var(--space-4);
  }
  .row {
    display: flex;
    justify-content: space-between;
    font-size: var(--text-sm);
  }
  .pct {
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
  .bar {
    height: 6px;
    margin: 6px 0 4px;
    border-radius: 999px;
    background: var(--surface-3);
    overflow: hidden;
  }
  .bar span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--accent);
    transition: width var(--normal) var(--ease);
  }
  .bar span.high {
    background: var(--danger);
  }
  .reset {
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
