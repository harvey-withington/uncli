<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { tintStyle } from '../lib/tint'
  import { blockMatching } from '../lib/search'
  import ApprovalCard from './ApprovalCard.svelte'
  import HostReviewCard from './HostReviewCard.svelte'
  import ScrollKeeper from './ScrollKeeper.svelte'
  import SessionHeader from './SessionHeader.svelte'
  import Composer from './Composer.svelte'
  import Icon from './Icon.svelte'
  import NavBar from './NavBar.svelte'
  import PageView from './PageView.svelte'
  import SidePanel from './SidePanel.svelte'
  import { useShell } from '../lib/shell.svelte'

  const shell = useShell()

  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const chat = useChat()
  const profile = $derived(chat.boot?.profiles.find(p => p.id === session.profileId))
  const page = $derived(chat.currentPage)
  let scroller: HTMLElement | undefined = $state()
  // The approval cards fade out under the page controls only while there is
  // more of them below; a card that fits doesn't fade.
  let approvalsEl: HTMLElement | undefined = $state()
  let moreBelow = $state(false)
  function measureApprovals() {
    const el = approvalsEl
    moreBelow = !!el && el.scrollHeight - el.scrollTop - el.clientHeight > 2
  }
  $effect(() => {
    const el = approvalsEl
    void cards.length // look again when cards come and go
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(measureApprovals)
    ro.observe(el)
    for (const child of el.children) ro.observe(child)
    measureApprovals()
    return () => ro.disconnect()
  })
  // Requests the quick-task model is still judging wait without a card.
  const cards = $derived((session.approvals ?? []).filter(a => !a.judging))
  const checking = $derived((session.approvals ?? []).some(a => a.judging))
  // An embedding host's reviews of its own changes, after the CLI's cards.
  const reviews = $derived(chat.reviewsOf(session.id))

  async function restore() {
    try {
      await chat.archive(session.id, false)
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  // Opened from search: once the page has rendered, scroll to the first
  // block holding a search word (below the sticky question) and flash it.
  $effect(() => {
    const r = chat.reveal
    const el = scroller
    if (!r || !el || page?.id !== r.pageId) return
    const frame = window.requestAnimationFrame ?? ((f: FrameRequestCallback) => window.setTimeout(f, 16))
    frame(() => frame(() => {
      if (chat.reveal !== r) return
      chat.reveal = null
      const blocks = Array.from(el.querySelectorAll<HTMLElement>('[data-block]'))
      const i = blockMatching(blocks.map(b => b.textContent ?? ''), r.terms)
      const target = blocks[i]
      if (!target) return
      const header = (el.querySelector('.question') as HTMLElement | null)?.offsetHeight ?? 0
      const top = target.getBoundingClientRect().top - el.getBoundingClientRect().top + el.scrollTop - header - 12
      const smooth = !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
      el.scrollTo?.({ top: Math.max(0, top), behavior: smooth ? 'smooth' : 'auto' })
      target.classList.add('search-flash')
      window.setTimeout(() => target.classList.remove('search-flash'), 1600)
    }))
  })
</script>

<section class="pane">
  <SessionHeader {session} />

  <div class="middle">
    <!-- The answer column: the page, the page controls floating on the line
         above the composer (centred over the answer, not the outline), and
         the composer. -->
    <div class="main">
      <div class="scroll" bind:this={scroller} part="page">
        <div class="content">
          {#if page}
            <PageView {page} />
          {:else}
            <div class="empty">
              <span class="eicon" style={tintStyle(profile?.hue)}><Icon name={profile?.icon ?? 'message-circle'} size={22} /></span>
              <h2>{t(`profile.${session.profileId}.empty`)}</h2>
              <p>{t(`profile.${session.profileId}.desc`)}</p>
            </div>
          {/if}
        </div>
      </div>
      <!-- Where the scroller sits through updates, and whether it follows
           the end (the toggle under its scrollbar). -->
      <ScrollKeeper {scroller} pageId={page?.id} />
      {#if cards.length || checking || reviews.length}
        <div class="approvals" class:more-below={moreBelow} bind:this={approvalsEl} onscroll={measureApprovals} part="approvals">
          {#each cards as a (a.requestId)}
            <ApprovalCard {session} approval={a} />
          {/each}
          {#each reviews as r (r.id)}
            <HostReviewCard review={r} />
          {/each}
          {#if checking}
            <p class="checking" role="status"><Icon name="loader" spin size={13} />{t('asking.checking')}</p>
          {/if}
        </div>
      {/if}
      <div class="dock"><NavBar /></div>
      <footer class="bottom" part="composer">
        <div class="inner">
          {#if session.archived}
            <!-- An archived session reads like any other, but can't take a turn. -->
            <div class="restore" role="status">
              <Icon name="archive" size={15} />
              <span>{t('archive.readOnly')}</span>
              <button class="btn primary small" onclick={restore}><Icon name="archive-restore" size={14} />{t('archive.restore')}</button>
            </div>
          {:else}
            <Composer {session} />
          {/if}
          {#if chat.embedded}<svelte:element this={'slot'} name="below-composer" />{/if}
        </div>
      </footer>
    </div>
    {#if chat.panelShown && chat.tabsFor(session.id).length > 0}
      <SidePanel sessionId={session.id} page={page ?? null} {scroller} />
    {/if}
  </div>
</section>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    height: 100%;
    background: var(--bg);
  }
  .middle {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .scroll {
    /* Positioned, so absolutely positioned descendants (visually-hidden
       labels) are clipped here instead of stretching the window. */
    position: relative;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
  /* At least the scroller's height, so the fade below always sits at the
     bottom of the panel, never over the end of short content. flow-root
     keeps children's margins inside it so they don't add scroll height. */
  .content {
    display: flow-root;
    min-height: 100%;
  }
  /* Text fades out above the page controls instead of running into them.
     Sticky inside the scroller, so it never covers the scrollbar; the
     negative margin keeps it from adding scroll height. */
  .scroll::after {
    content: "";
    position: sticky;
    bottom: 0;
    z-index: 1;
    display: block;
    height: 64px;
    margin-top: -64px;
    background: linear-gradient(to bottom, transparent, var(--bg) 85%);
    pointer-events: none;
  }
  /* A zero-height row on the line between page and composer; the pill is
     centred on it, half above and half below. */
  .dock {
    position: relative;
    z-index: 6; /* above the approval cards, which fade out beneath it */
    height: 0;
    display: flex;
    align-items: flex-start; /* don't squash the pill to the row's zero height */
    justify-content: center;
  }
  .dock :global(.navbar) {
    transform: translateY(-50%);
  }
  /* Approval cards sit just above the page controls, over the end of the
     answer, so they're in view whatever page is showing. Like the answer,
     they fade out beneath the page controls; the bottom padding leaves the
     last card's buttons clear of them when scrolled to the end. */
  .approvals {
    position: relative;
    z-index: 5;
    max-height: 55%;
    overflow-y: auto;
    padding: var(--space-2) var(--space-6) calc(18px + var(--space-4));
    margin-top: calc(-1 * var(--space-4));
  }
  .approvals.more-below::after {
    content: "";
    position: sticky;
    bottom: calc(-18px - var(--space-4)); /* to the panel's bottom edge, through the padding */
    z-index: 1;
    display: block;
    height: 40px;
    margin-top: -40px;
    background: linear-gradient(to bottom, transparent, var(--bg) 85%);
    pointer-events: none;
  }
  .checking {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .empty {
    max-width: 420px;
    margin: 18vh auto 0;
    padding: 0 var(--space-5);
    text-align: center;
    color: var(--text-muted);
  }
  .eicon {
    display: inline-grid;
    place-items: center;
    width: 48px;
    height: 48px;
    border-radius: 14px;
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
  }
  .empty h2 {
    margin: var(--space-4) 0 var(--space-2);
    font-size: var(--text-xl);
    font-weight: 620;
    letter-spacing: -0.015em;
    color: var(--text);
  }
  .empty p {
    margin: 0;
    font-size: var(--text-md);
  }
  /* Top padding is half the page-control buttons' height (36px) plus a
     small gap, so the composer starts just below them. */
  .bottom {
    padding: calc(18px + var(--space-2)) var(--space-6) var(--space-4);
    border-top: 1px solid var(--border);
    background: var(--bg);
  }
  .restore {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border: 1px dashed var(--border-strong);
    border-radius: var(--radius-lg);
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .restore span {
    flex: 1;
  }
  .inner {
    max-width: calc(var(--reading-width) + 2 * var(--space-6));
    margin: 0 auto;
  }
</style>
