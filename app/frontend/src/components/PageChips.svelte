<script lang="ts">
  import type { Page } from '../lib/api'
  import { useApp } from '../lib/context'
  import { cost, duration, tokens } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { modelLabel } from '../lib/models'
  import Icon from './Icon.svelte'

  interface Props {
    page: Page
  }

  let { page }: Props = $props()
  const app = useApp()
  const mods = $derived(page.modifiers.map(id => app.boot?.modifiers.find(m => m.id === id)).filter(m => !!m))
  const done = $derived(page.status !== 'open')
</script>

<div class="chips" aria-label={t('page.details')}>
  <span class="chip model" title={t('page.model')}><Icon name="sparkles" size={12} />{modelLabel(app.boot?.models ?? null, page.model)}</span>
  {#each mods as m (m.id)}
    <span class="chip mod" title={t('page.modifier')}><Icon name={m.icon} size={12} />{m.label}</span>
  {/each}
  {#if done && page.outputTokens > 0}
    <span class="chip num" title={t('page.tokensTitle', { input: page.inputTokens + page.cacheRead + page.cacheWrite, output: page.outputTokens })}>
      {t('page.tokens', { input: tokens(page.inputTokens + page.cacheRead + page.cacheWrite), output: tokens(page.outputTokens) })}
    </span>
    <span class="chip num" title={t('page.costTitle')}>{cost(page.costUsd)}</span>
    <span class="chip num">{duration(page.durationMs)}</span>
  {/if}
</div>

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 22px;
    padding: 0 9px;
    border: 1px solid var(--border);
    border-radius: 999px;
    font-size: var(--text-xs);
    color: var(--text-muted);
    background: var(--surface);
  }
  .model {
    color: var(--text);
  }
  .mod {
    border-color: transparent;
    background: var(--accent-soft);
    color: var(--accent);
  }
  .num {
    font-variant-numeric: tabular-nums;
    border-style: dashed;
  }
</style>
