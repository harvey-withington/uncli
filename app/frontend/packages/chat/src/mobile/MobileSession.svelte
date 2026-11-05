<script lang="ts">
  import ApprovalCard from '../components/ApprovalCard.svelte'
  import HostReviewCard from '../components/HostReviewCard.svelte'
  import Composer from '../components/Composer.svelte'
  import Icon from '../components/Icon.svelte'
  import NavBar from '../components/NavBar.svelte'
  import PageView from '../components/PageView.svelte'
  import SessionHeader from '../components/SessionHeader.svelte'
  import SidePanel from '../components/SidePanel.svelte'
  import type { SessionView } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { wide } from './wide.svelte'
  import { useShell } from '../lib/shell.svelte'

  const shell = useShell()

  // A session on the phone, from the window's own components in a compact
  // layout: a header with Back, the page and its controls, the approval
  // cards over the end of it, and the composer. The toolbar (model,
  // modifiers, prompt level, Unattended) folds away under Options; the
  // side panel (outline, artifacts, a watched file) opens over the page,
  // or when wide (a tablet, wide.svelte.ts) beside it, as wide as it's
  // dragged. It's also an embedded chat's compact layout (Chat.svelte),
  // which says how wide it is and has no Back.

  interface Props {
    session: SessionView
    onback?: () => void
    wide?: boolean // the layout beside the answer; the phone's tablet width otherwise
  }

  let { session, onback, wide: wideProp }: Props = $props()
  const isWide = $derived(wideProp ?? wide.on)
  const app = useChat()
  const profile = $derived(app.boot?.profiles.find(p => p.id === session.profileId))
  const page = $derived(app.currentPage)
  const cards = $derived((session.approvals ?? []).filter(a => !a.judging))
  const checking = $derived((session.approvals ?? []).some(a => a.judging))
  // An embedding host's reviews of its own changes, after the CLI's cards.
  const reviews = $derived(app.reviewsOf(session.id))
  let scroller: HTMLElement | undefined = $state()

  async function restore() {
    try {
      await app.archive(session.id, false)
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

<section class="session" class:wide={isWide}>
  <SessionHeader {session} compact {onback} />

  <!-- Everything under the header. On a phone it makes no box of its own
       (display: contents); on a tablet it leaves room for the side panel. -->
  <div class="body" class:with-panel={isWide && app.outline.open} style:--panel-w="min({app.outline.width}px, 45%)">
  <div class="scroll" bind:this={scroller} part="page">
    <div class="content">
      {#if page}
        <PageView {page} />
      {:else}
        <p class="empty">{t(`profile.${session.profileId}.desc`)}</p>
      {/if}
    </div>
  </div>

  {#if cards.length || checking || reviews.length}
    <div class="approvals" part="approvals">
      {#each cards as a (a.requestId)}
        <ApprovalCard {session} approval={a} />
      {/each}
      {#each reviews as r (r.id)}
        <HostReviewCard review={r} />
      {/each}
      {#if checking}<p class="checking" role="status"><Icon name="loader" spin size={13} />{t('asking.checking')}</p>{/if}
    </div>
  {/if}
  <div class="dock"><NavBar /></div>
  <footer class="bottom" part="composer">
    {#if session.archived}
      <div class="restore" role="status">
        <span>{t('archive.readOnly')}</span>
        <button class="btn primary small" onclick={restore}><Icon name="archive-restore" size={14} />{t('archive.restore')}</button>
      </div>
    {:else}
      <Composer {session} />
    {/if}
    {#if app.embedded}<svelte:element this={'slot'} name="below-composer" />{/if}
  </footer>

  {#if app.outline.open}
    <div class="panel" role={isWide ? undefined : 'dialog'} aria-label={isWide ? undefined : t('outline.toggle')}>
      {#if !isWide}<button class="btn ghost icon touch close" onclick={() => app.togglePanel()} aria-label={t('common.close')}><Icon name="x" /></button>{/if}
      <SidePanel sessionId={session.id} page={page ?? null} {scroller} />
    </div>
  {/if}
  </div>
</section>

<style>
  .session {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg);
  }
  .touch {
    width: 44px;
    height: 44px;
    flex: none;
  }
  /* No padding: the question bar sticks at the very top, under the header,
     with nothing showing above or beside it. */
  .scroll {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
  }
  /* At least the scroller's height, so the fade sits at the bottom of the
     panel, never over the end of short content (as on the desktop). */
  .content {
    display: flow-root;
    min-height: 100%;
  }
  .scroll::after {
    content: "";
    position: sticky;
    bottom: 0;
    z-index: 1;
    display: block;
    height: 48px;
    margin-top: -48px;
    background: linear-gradient(to bottom, transparent, var(--bg) 85%);
    pointer-events: none;
  }
  /* The left padding is the desktop's: the outline's markers sit in it. */
  .scroll :global(.page) {
    padding: 0 var(--space-4) var(--space-7) var(--space-6);
  }
  /* The question bar carries the header's panel down, solid and edge to edge. */
  .scroll :global(.page .question) {
    margin-left: calc(-1 * var(--space-6));
    margin-right: calc(-1 * var(--space-4));
    padding-left: var(--space-6);
    padding-right: var(--space-4);
    background: var(--surface);
    backdrop-filter: none;
  }
  .empty {
    margin: 20vh var(--space-4) 0;
    text-align: center;
    color: var(--text-muted);
  }
  .approvals {
    max-height: 50%;
    overflow-y: auto;
    padding: var(--space-2) var(--space-3) calc(18px + var(--space-2));
    border-top: 1px solid var(--border);
    background: var(--bg);
  }
  .checking {
    display: flex;
    justify-content: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .dock {
    position: relative;
    z-index: 3;
    height: 0;
    display: flex;
    align-items: flex-start;
    justify-content: center;
  }
  .dock :global(.navbar) {
    transform: translateY(-50%);
  }
  .bottom {
    padding: calc(18px + var(--space-2)) var(--space-3) calc(var(--space-3) + env(safe-area-inset-bottom));
    border-top: 1px solid var(--border);
    background: var(--bg);
  }
  .restore {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .restore span {
    flex: 1;
  }
  .panel {
    position: absolute;
    inset: 0;
    z-index: 20;
    display: flex;
    flex-direction: column;
    background: var(--surface);
  }
  .panel .close {
    align-self: flex-end;
  }
  /* A phone: the body is no box, so its parts sit in the session's column
     as they always have, and the side panel covers the whole screen. */
  .body {
    display: contents;
  }
  /* A tablet: the body is the column under the header, the side panel sits
     at its right as wide as it's dragged, and the message box and approval
     cards line up with the answer column, as on the desktop. */
  .wide .body {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .wide .body.with-panel {
    padding-right: var(--panel-w);
  }
  .wide .panel {
    inset: 0 0 0 auto;
    width: var(--panel-w);
    border-left: 1px solid var(--border);
    background: var(--bg);
  }
  .wide .bottom > :global(*),
  .wide .approvals > :global(*) {
    max-width: calc(var(--reading-width) + 2 * var(--space-6));
    margin-left: auto;
    margin-right: auto;
  }
  .panel :global(> *:last-child) {
    flex: 1;
    min-height: 0;
    width: 100% !important;
  }
</style>
