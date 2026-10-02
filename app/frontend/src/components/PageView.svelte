<script lang="ts">
  import { fade } from 'svelte/transition'
  import type { Page } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { displayAnswer } from '../stores/app.svelte'
  import { attachmentIcon, formatSize } from '../lib/attachments'
  import AnswerBlocks from './AnswerBlocks.svelte'
  import CopyButton from './CopyButton.svelte'
  import Icon from './Icon.svelte'
  import PageChips from './PageChips.svelte'
  import TraceStrip from './TraceStrip.svelte'

  interface Props {
    page: Page
  }

  let { page }: Props = $props()
  const app = useApp()
  const live = $derived(app.live[page.sessionId])
  const answer = $derived(displayAnswer(page, live))
  const open = $derived(page.status === 'open')
  const thinking = $derived(open && !answer)
  const compact = $derived(app.questionCompact)
  // A question sent with files only reads as their names.
  const questionLine = $derived(page.question || (page.attachments ?? []).map(f => f.name).join(', '))
  // The outline's entries, marked beside their blocks in the left margin.
  const markers = $derived(new Map(app.outlineFor(page).entries.map(e => [e.block, e])))
</script>

{#snippet toggle()}
  <button
    class="btn ghost small icon q-toggle"
    onclick={() => app.toggleQuestionCompact()}
    aria-expanded={!compact}
    aria-label={t(compact ? 'page.expandQuestion' : 'page.collapseQuestion')}
    title={t(compact ? 'page.expandQuestion' : 'page.collapseQuestion')}
  >
    <Icon name={compact ? 'chevron-down' : 'chevron-up'} size={14} />
  </button>
{/snippet}

{#key page.id}
  <article class="page" in:fade={{ duration: 140 }}>
    <header class="question" class:compact>
      {#if compact}
        <!-- Compact: the question on one line, cut short before the page number. -->
        <div class="q-head">
          <p class="q-line" title={questionLine}>{questionLine}</p>
          <span class="seq">{t('page.number', { n: page.seq })}</span>
          <span class="q-copy"><CopyButton text={page.question} label={t('copy.question')} /></span>
          {@render toggle()}
        </div>
      {:else}
        <div class="q-head">
          <span class="seq">{t('page.number', { n: page.seq })}</span>
          {#if page.bookmarked}<span class="marked"><Icon name="bookmark-check" size={13} />{t('nav.bookmarked')}</span>{/if}
          <span class="q-copy"><CopyButton text={page.question} label={t('copy.question')} /></span>
          {@render toggle()}
        </div>
        {#if page.question}<p class="q-text">{page.question}</p>{/if}
        {#if page.attachments?.length}
          <ul class="q-files" aria-label={t('page.attachments')}>
            {#each page.attachments as f, i (i)}
              <li title={f.path || f.name}>
                <Icon name={attachmentIcon(f.mediaType)} size={13} />
                <span class="fname">{f.name}</span>
                <span class="fsize">{formatSize(f.size)}</span>
              </li>
            {/each}
          </ul>
        {/if}
        <PageChips {page} />
      {/if}
    </header>

    <div class="body">
      {#if thinking}
        <p class="thinking" role="status">
          <span class="dots" aria-hidden="true"><i></i><i></i><i></i></span>
          {#if live?.thinkingTokens}
            {t('page.thinkingTokens', { n: live.thinkingTokens })}
          {:else}
            {t('page.thinking')}
          {/if}
        </p>
      {:else if answer}
        <AnswerBlocks markdown={answer} streaming={open} {markers} />
      {/if}

      {#if page.status === 'interrupted'}
        <p class="banner warn"><Icon name="square" size={14} />{t('page.interrupted')}</p>
      {:else if page.status === 'error'}
        <p class="banner err" role="alert"><Icon name="triangle-alert" size={14} />{page.error || t('page.failed')}</p>
      {/if}

      <TraceStrip items={page.trace} />
    </div>
  </article>
{/key}

<style>
  .page {
    max-width: var(--reading-width);
    margin: 0 auto;
    padding: var(--space-6) var(--space-6) var(--space-7);
  }
  .question {
    position: sticky;
    top: 0;
    z-index: 2;
    margin: 0 calc(-1 * var(--space-6)) var(--space-5);
    padding: var(--space-4) var(--space-6) var(--space-4);
    background: color-mix(in srgb, var(--bg) 88%, transparent);
    backdrop-filter: blur(10px);
    border-bottom: 1px solid var(--border);
  }
  .q-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: 26px;
  }
  .compact {
    padding-top: var(--space-2);
    padding-bottom: var(--space-2);
  }
  /* One line: the question gives way (ellipsis) so the page number stays. */
  .q-line {
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: var(--text-md);
    font-weight: 560;
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .compact .seq,
  .compact .q-copy {
    flex: none;
  }
  .q-toggle {
    flex: none;
    color: var(--text-faint);
  }
  .q-copy + .q-toggle {
    margin-left: calc(-1 * var(--space-2));
  }
  .seq {
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .marked {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: var(--text-xs);
    color: var(--accent);
  }
  .q-copy {
    margin-left: auto;
    opacity: 0;
    transition: opacity var(--fast) var(--ease);
  }
  .question:hover .q-copy,
  .q-copy:focus-within,
  .q-copy:has(:global(.copied)) {
    opacity: 1;
  }
  .q-text {
    margin: var(--space-1) 0 var(--space-3);
    font-size: 17px;
    font-weight: 560;
    line-height: 1.45;
    letter-spacing: -0.01em;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    max-height: 30vh;
    overflow-y: auto;
  }
  .q-files {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: var(--space-1) 0 var(--space-3);
    padding: 0;
    list-style: none;
  }
  .q-files li {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 280px;
    height: 26px;
    padding: 0 8px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    color: var(--accent);
    font-size: var(--text-xs);
  }
  .fname {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
    font-weight: 550;
  }
  .fsize {
    flex: none;
    color: var(--text-faint);
  }
  .thinking {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--text-muted);
    font-size: var(--text-md);
  }
  .dots {
    display: inline-flex;
    gap: 4px;
  }
  .dots i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    animation: bounce 1.2s var(--ease) infinite;
  }
  .dots i:nth-child(2) { animation-delay: 0.15s; }
  .dots i:nth-child(3) { animation-delay: 0.3s; }
  @keyframes bounce {
    0%, 80%, 100% { opacity: 0.3; transform: translateY(0); }
    40% { opacity: 1; transform: translateY(-3px); }
  }
  .banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-5) 0 0;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--text-sm);
  }
  .warn {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .err {
    background: var(--danger-soft);
    color: var(--danger);
  }
</style>
