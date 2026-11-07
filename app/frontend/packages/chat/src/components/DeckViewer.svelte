<script lang="ts">
  import { untrack } from 'svelte'
  import type { PageExplanation } from '../lib/api'
  import type { Deck } from '../lib/deck'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // A walkthrough's slides (decisions 0029, 0030): UNCLI's own viewer, so
  // they reflow like the rest of UNCLI at any size, turned or not. A title
  // slide, a slide per changed place (its file, lines and counts over its
  // code), then the files changed without a record. The slides sit in a row
  // that snaps to each one, so a phone swipes between them natively; arrow
  // keys, Page Up and Down, Home and End move too, and the counter says
  // where you are. Code wider than its slide scrolls on its own. Full
  // screen puts the viewer on the whole screen. An explanation, when there
  // is one, puts its summary on the title slide and its note for each place
  // above that place's code; Explain (with its options beside it) asks for
  // one, or for another. On a change's slide the file line and the note
  // stay put while the code scrolls on its own, and fold away (on every
  // slide at once) to give the code the room.
  interface Props {
    deck: Deck
    codes: string[] // each changed place's code (lib/deckcode.ts)
    title: string // the page's question
    summary: string // "3 changes in 2 files"
    explanation?: PageExplanation | null
    by?: string // who explained it, and what it cost
    explain?: { busy: boolean; run: () => void; options: () => void } // absent: no Explain
  }
  let { deck, codes, title, summary, explanation, by, explain }: Props = $props()
  const notes = $derived(new Map((explanation?.notes ?? []).map(n => [n.place, n.text])))

  let box = $state<HTMLElement>()
  let track = $state<HTMLElement>()
  let index = $state(0)
  let full = $state(false)
  // Folded: each change's file line and note give their room to its code.
  let folded = $state(false)
  const total = $derived(1 + deck.slides.length + (deck.others.length ? 1 : 0))
  const files = $derived([...new Set(deck.slides.map(s => s.rel))])

  function go(i: number, smooth = true) {
    if (!track) return
    const n = Math.max(0, Math.min(total - 1, i))
    index = n
    track.scrollTo?.({ left: n * track.clientWidth, behavior: smooth ? 'smooth' : 'instant' })
  }

  function onscroll() {
    if (track?.clientWidth) index = Math.round(track.scrollLeft / track.clientWidth)
  }

  // Turning the phone, or going full screen, keeps the slide in view.
  $effect(() => {
    if (!track) return
    const ro = new ResizeObserver(() => untrack(() => go(index, false)))
    ro.observe(track)
    return () => ro.disconnect()
  })

  function onkeydown(e: KeyboardEvent) {
    if (e.altKey || e.ctrlKey || e.metaKey || (e.target as HTMLElement).tagName === 'BUTTON') return
    const to = ({ ArrowRight: index + 1, PageDown: index + 1, ' ': index + 1, ArrowLeft: index - 1, PageUp: index - 1, Home: 0, End: total - 1 } as Record<string, number>)[e.key]
    if (to === undefined) return
    e.preventDefault()
    go(to)
  }

  function fullscreen() {
    if (full) void document.exitFullscreen()
    else void box?.requestFullscreen().catch(() => {})
  }
</script>

<svelte:document onfullscreenchange={() => (full = !!document.fullscreenElement)} />

<!-- A focusable region whose keys move between the slides. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div class="viewer" bind:this={box} tabindex="0" role="region" aria-roledescription={t('walk.slides')} aria-label={title} {onkeydown} data-autofocus>
  <div class="track" bind:this={track} {onscroll}>
    <section class="slide" aria-label={t('walk.slide', { n: 1, total })}>
      <h3>{title}</h3>
      <p class="summary">{summary}</p>
      {#if explanation?.summary}
        <p class="note">{explanation.summary}</p>
        {#if by}<p class="by">{by}</p>{/if}
      {/if}
      <ul class="files">{#each files as f (f)}<li>{f}</li>{/each}</ul>
      <p class="keys">{t('walk.keys')}</p>
    </section>
    {#each deck.slides as s, i (i)}
      {@const note = notes.get(s.place)}
      <section class="slide change" class:folded aria-label={t('walk.slide', { n: i + 2, total })}>
        <div class="head">
          <div class="file">
            <span class="path">{s.rel}</span>
            <span class="lines">{t('walk.lines', { from: s.first, to: s.last })}</span>
            {#if s.added}<span class="add">+{s.added}</span>{/if}{#if s.removed}<span class="del">−{s.removed}</span>{/if}
          </div>
          <button class="btn ghost small icon fold" onclick={() => (folded = !folded)} aria-expanded={!folded}
            aria-label={folded ? t('walk.unfold') : t('walk.fold')} title={folded ? t('walk.unfold') : t('walk.fold')}>
            <Icon name={folded ? 'chevron-down' : 'chevron-up'} size={14} />
          </button>
        </div>
        {#if note && !folded}<p class="note">{note}</p>{/if}
        <div class="code">{@html codes[i] ?? ''}</div>
      </section>
    {/each}
    {#if deck.others.length}
      <section class="slide" aria-label={t('walk.slide', { n: total, total })}>
        <h3>{t('walk.others')}</h3>
        <ul class="files">{#each deck.others as f (f)}<li>{f}</li>{/each}</ul>
      </section>
    {/if}
  </div>
  <div class="nav">
    <button class="btn ghost small icon" onclick={() => go(index - 1)} disabled={index === 0} aria-label={t('walk.prev')} title={t('walk.prev')}><Icon name="chevron-left" size={16} /></button>
    <span class="count" aria-live="polite">{index + 1} / {total}</span>
    <button class="btn ghost small icon" onclick={() => go(index + 1)} disabled={index >= total - 1} aria-label={t('walk.next')} title={t('walk.next')}><Icon name="chevron-right" size={16} /></button>
    <span class="grow"></span>
    {#if explain}
      <span class="split">
        <button class="btn small" onclick={explain.run} disabled={explain.busy}>
          <Icon name={explain.busy ? 'loader' : 'sparkles'} spin={explain.busy} size={13} />{explain.busy ? t('explain.busy') : explanation ? t('explain.again') : t('explain.button')}
        </button>
        <button class="btn small icon" onclick={explain.options} disabled={explain.busy} aria-label={t('explain.options')} title={t('explain.options')}><Icon name="chevron-down" size={13} /></button>
      </span>
    {/if}
    <button class="btn ghost small fs" onclick={fullscreen} aria-label={full ? t('walk.leaveFullscreen') : t('walk.fullscreen')} title={full ? t('walk.leaveFullscreen') : t('walk.fullscreen')}>
      <Icon name={full ? 'minimize' : 'maximize'} size={13} /><span class="txt">{full ? t('walk.leaveFullscreen') : t('walk.fullscreen')}</span>
    </button>
  </div>
</div>

<style>
  .viewer {
    display: flex;
    flex-direction: column;
    min-width: 0;
    /* As tall as the dialog has room for, a turned phone's included. */
    height: min(75dvh, 760px, calc(100dvh - 190px));
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    outline: none;
  }
  .viewer:focus-visible {
    box-shadow: 0 0 0 3px var(--accent-soft);
  }
  .viewer:fullscreen {
    height: 100%;
    border: 0;
    border-radius: 0;
  }
  .track {
    flex: 1;
    min-height: 0;
    display: flex;
    overflow-x: auto;
    overflow-y: hidden;
    scroll-snap-type: x mandatory;
    overscroll-behavior-x: contain;
    scrollbar-width: none;
  }
  .slide {
    flex: 0 0 100%;
    min-width: 0;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    overflow-y: auto;
    scroll-snap-align: start;
    scroll-snap-stop: always;
  }
  @media (max-width: 600px) {
    .slide {
      padding: var(--space-3);
    }
  }
  .slide.change {
    overflow: hidden;
  }
  .slide.change.folded {
    gap: var(--space-2);
  }
  .head {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .head .file {
    flex: 1;
    min-width: 0;
    padding-top: 4px;
  }
  .folded .file {
    flex-wrap: nowrap;
    overflow: hidden;
  }
  .folded .path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl; /* when cut, keep the file name */
    text-align: left;
  }
  .folded .lines {
    flex: none;
  }
  .fold {
    flex: none;
  }
  .change .note {
    flex: none;
    max-height: 40%;
    overflow-y: auto;
  }
  h3 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 600;
    line-height: 1.35;
  }
  .summary {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .files {
    margin: 0;
    padding-left: var(--space-4);
    font-family: var(--mono);
    font-size: var(--text-xs);
    line-height: 1.7;
    overflow-wrap: anywhere;
  }
  .keys {
    margin: auto 0 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  @media (hover: none) {
    .keys {
      display: none; /* a finger swipes, or uses the arrows below */
    }
  }
  .note {
    margin: 0;
    padding: var(--space-2) var(--space-3);
    border-left: 3px solid var(--accent);
    border-radius: var(--radius-sm);
    background: var(--accent-soft);
    font-size: var(--text-sm);
    line-height: 1.55;
    white-space: pre-line;
  }
  .by {
    margin: calc(-1 * var(--space-2)) 0 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .split {
    display: inline-flex;
  }
  .nav .btn {
    white-space: nowrap;
  }
  @media (max-width: 600px) {
    .fs .txt {
      display: none; /* the icon alone, its name kept for screen readers */
    }
  }
  .split .btn:first-child {
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
  }
  .split .btn + .btn {
    margin-left: -1px;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
  }
  .file {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px var(--space-3);
    font-family: var(--mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .path {
    color: var(--text);
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .add {
    color: var(--success);
  }
  .del {
    color: var(--danger);
  }
  .code {
    flex: 1;
    min-height: 0;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
  }
  .code :global(pre) {
    margin: 0;
    padding: var(--space-2) 0;
    overflow: visible;
    min-width: max-content;
    background: none;
    font-family: var(--mono);
    font-size: 12.5px;
    line-height: 1.6;
    tab-size: 4;
  }
  .code :global(code) {
    display: block;
    min-width: max-content;
  }
  .code :global(.line) {
    display: inline-block;
    box-sizing: border-box;
    min-width: 100%;
    min-height: 1.6em;
    padding: 0 var(--space-3) 0 4.4em;
    position: relative;
    white-space: pre;
    vertical-align: top;
  }
  .code :global(.line::before) {
    content: attr(data-n);
    position: absolute;
    left: 0;
    width: 2.6em;
    text-align: right;
    color: var(--text-faint);
  }
  .code :global(.line.add),
  .code :global(.line.del) {
    background: color-mix(in srgb, var(--success) 13%, transparent);
  }
  .code :global(.line.del) {
    background: color-mix(in srgb, var(--danger) 11%, transparent);
  }
  .code :global(.line.add::after),
  .code :global(.line.del::after) {
    position: absolute;
    left: 3.1em;
    font-weight: 700;
  }
  .code :global(.line.add::after) {
    content: '+';
    color: var(--success);
  }
  .code :global(.line.del::after) {
    content: '−';
    color: var(--danger);
  }
  .code :global(.line.del span) {
    text-decoration: line-through;
    text-decoration-color: color-mix(in srgb, var(--danger) 50%, transparent);
  }
  .code :global(pre span) {
    color: light-dark(var(--shiki-light), var(--shiki-dark));
  }
  .nav {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-2);
    border-top: 1px solid var(--border);
  }
  .count {
    min-width: 4.5em;
    text-align: center;
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--text-muted);
  }
  .grow {
    flex: 1;
  }
</style>
