<script lang="ts">
  import type { Page } from '../lib/api'
  import { useApp } from '../lib/context'
  import { cost } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { modelLabel } from '../lib/models'
  import { activeEntry, autoSummaryFor } from '../lib/outline'
  import { KIND_ICONS } from '../lib/sections'
  import Icon from './Icon.svelte'

  // The "On this page" tab: the answer's headings, or a summary written by
  // the quick-task model. Clicking an entry scrolls to it; the one being
  // read is highlighted.
  interface Props {
    page: Page
    scroller: HTMLElement | undefined
  }

  let { page, scroller }: Props = $props()
  const app = useApp()
  const outline = $derived(app.outlineFor(page))
  const blocks = $derived(outline.blocks)
  const showingSummary = $derived(outline.showingSummary)
  const entries = $derived(outline.entries)
  const quick = $derived(app.boot?.preferences.quickTaskModel)
  const quickLabel = $derived(quick ? modelLabel(app.boot?.models ?? null, quick.model) : '')
  const summarising = $derived(!!app.summarising[page.id])
  let active = $state(-1)
  // A finished answer the automatic summary passed over for being short.
  const short = $derived(
    page.status === 'done' && !page.outline && autoSummaryFor(page.answerMd, blocks.length, app.boot?.preferences.autoSummary ?? 'off') === 'short',
  )

  const summarise = () => app.summarise(page)

  // Answers summarise themselves as they finish (the store does that); an
  // older page opened here gets the same chance.
  $effect(() => {
    app.autoSummarise(page)
  })

  // The sticky question covers the top of the scroller; content is "being
  // read" once it passes just below it.
  function headerHeight(): number {
    return (scroller?.querySelector('.question') as HTMLElement | null)?.offsetHeight ?? 0
  }

  function blockEl(i: number): HTMLElement | null {
    return scroller?.querySelector<HTMLElement>(`[data-block="${i}"]`) ?? null
  }

  function track() {
    if (!scroller) return
    const top = scroller.getBoundingClientRect().top
    const tops = entries.map(e => (blockEl(e.block)?.getBoundingClientRect().top ?? Infinity) - top)
    active = activeEntry(tops, headerHeight() + 24)
  }

  $effect(() => {
    const el = scroller
    void entries // re-track when the entries change (streaming, new page, summary)
    if (!el) return
    queueMicrotask(track)
    el.addEventListener('scroll', track, { passive: true })
    return () => el.removeEventListener('scroll', track)
  })

  const smooth = () => (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth')

  function jump(i: number) {
    const el = blockEl(entries[i]?.block ?? -1)
    if (!scroller || !el) return
    const y = el.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop - headerHeight() - 12
    scroller.scrollTo({ top: Math.max(0, y), behavior: smooth() })
  }

  function toTop() {
    scroller?.scrollTo({ top: 0, behavior: smooth() })
  }
</script>

<div class="outline">
  <div class="head">
    <span class="what">{#if !(outline.summary.length > 0 && outline.headings.length > 0) && entries.length > 0}{t(showingSummary ? 'outline.summary' : 'outline.headings')}{/if}</span>
    <button
      class="btn ghost small icon"
      onclick={summarise}
      disabled={summarising || page.status === 'open'}
      aria-label={t(page.outline ? 'outline.resummarise' : 'outline.summarise', { model: quickLabel })}
      title={t(page.outline ? 'outline.resummarise' : 'outline.summarise', { model: quickLabel })}
    >
      <Icon name={summarising ? 'loader' : 'sparkles'} spin={summarising} size={14} />
    </button>
  </div>

  {#if outline.summary.length > 0 && outline.headings.length > 0}
    <div class="switch" role="group" aria-label={t('outline.show')}>
      <button class:on={showingSummary} aria-pressed={showingSummary} onclick={() => delete app.preferHeadings[page.id]}>{t('outline.summary')}</button>
      <button class:on={!showingSummary} aria-pressed={!showingSummary} onclick={() => (app.preferHeadings[page.id] = true)}>{t('outline.headings')}</button>
    </div>
  {/if}

  <nav>
    <button class="entry question" class:active={active === -1} onclick={toTop}>
      <span class="kind"><Icon name="message-circle" size={13} /></span>
      <span class="label">{t('outline.question')}</span>
    </button>
    {#each entries as e, i (e.block)}
      <button
        class="entry kind-{e.kind}"
        class:active={active === i}
        style:--indent={e.level - 1}
        onclick={() => jump(i)}
        aria-current={active === i ? 'location' : undefined}
        title={`${t(`kind.${e.kind}`)}: ${e.text}`}
      >
        <span class="kind" style:color="var(--kind-{e.kind})"><Icon name={KIND_ICONS[e.kind]} size={13} /></span>
        <span class="visually-hidden">{t(`kind.${e.kind}`)}: </span>
        <span class="label">{e.text}</span>
      </button>
    {/each}
  </nav>

  {#if summarising}
    <p class="note">{t('outline.summarising', { model: quickLabel })}</p>
  {:else if short}
    <p class="note">
      {t('outline.short')}
      <button class="link" onclick={summarise}>{t('outline.summariseShort')}</button>
    </p>
  {:else if entries.length === 0}
    <p class="note">
      {t('outline.empty')}
      {#if page.status !== 'open'}
        <button class="link" onclick={summarise}>{t('outline.summariseShort')}</button>
      {/if}
    </p>
  {/if}
  {#if showingSummary && page.outline}
    <p class="note by">
      <Icon name="sparkles" size={12} />
      {t('outline.by', { model: modelLabel(app.boot?.models ?? null, page.outline.model), cost: cost(page.outline.costUsd) })}
    </p>
  {/if}
</div>

<style>
  .outline {
    position: relative; /* clips absolutely positioned descendants (as .scroll in SessionPane) */
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: var(--space-2) var(--space-3) var(--space-4) var(--space-4);
    overflow-y: auto;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin: 0 0 var(--space-2) var(--space-2);
  }
  .what {
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .switch {
    display: flex;
    gap: 2px;
    margin: 0 0 var(--space-3) var(--space-2);
    padding: 2px;
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    align-self: flex-start;
  }
  .switch button {
    padding: 2px 10px;
    border: 0;
    border-radius: 4px;
    background: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .switch button.on {
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-sm);
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .entry {
    display: flex;
    align-items: center;
    gap: 7px;
    width: 100%;
    padding: 5px var(--space-2) 5px calc(var(--space-2) + var(--indent, 0) * 14px);
    border: 0;
    border-left: 2px solid transparent;
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    line-height: 1.4;
    text-align: left;
    transition: color var(--fast) var(--ease), background var(--fast) var(--ease), border-color var(--fast) var(--ease);
  }
  .label {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .kind {
    display: inline-flex;
    flex: none;
    color: var(--text-faint);
    transition: color var(--fast) var(--ease);
  }
  .entry:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  .entry.active {
    border-left-color: var(--accent);
    color: var(--accent);
    font-weight: 550;
  }
  .question .label {
    font-style: italic;
  }
  .note {
    margin: var(--space-3) var(--space-2) 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .by {
    display: flex;
    align-items: center;
    gap: 5px;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font-size: inherit;
    text-decoration: underline;
    text-underline-offset: 2px;
  }
</style>
