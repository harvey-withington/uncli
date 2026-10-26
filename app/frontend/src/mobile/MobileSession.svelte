<script lang="ts">
  import ActivityBadge from '../components/ActivityBadge.svelte'
  import ApprovalCard from '../components/ApprovalCard.svelte'
  import Composer from '../components/Composer.svelte'
  import Icon from '../components/Icon.svelte'
  import ModeSwitch from '../components/ModeSwitch.svelte'
  import NavBar from '../components/NavBar.svelte'
  import PageView from '../components/PageView.svelte'
  import SessionStatus from '../components/SessionStatus.svelte'
  import SidePanel from '../components/SidePanel.svelte'
  import Toolbar from '../components/Toolbar.svelte'
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { tintStyle } from '../lib/tint'
  import { showToast } from '../lib/toasts.svelte'

  // A session on the phone, from the window's own components in a compact
  // layout: a header with Back, the page and its controls, the approval
  // cards over the end of it, and the composer. The toolbar (model,
  // modifiers, prompt level, Unattended) folds away under Options; the
  // side panel (outline, artifacts, a watched file) opens over the page.

  interface Props {
    session: SessionView
    onback: () => void
  }

  let { session, onback }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const profile = $derived(app.boot?.profiles.find(p => p.id === session.profileId))
  const page = $derived(app.currentPage)
  const cards = $derived((session.approvals ?? []).filter(a => !a.judging))
  const checking = $derived((session.approvals ?? []).some(a => a.judging))
  const failed = $derived(!!session.error && session.state === 'error')
  let options = $state(false)
  let scroller: HTMLElement | undefined = $state()

  async function toggleUnattended() {
    try {
      app.upsertSession(await app.backend.setUnattended(session.id, !session.unattended))
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function restore() {
    try {
      await app.archive(session.id, false)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

<section class="session">
  <!-- One panel down to the page's question bar, which sits flush under it. -->
  <div class="head">
    <header class="bar">
      <button class="btn ghost icon touch" onclick={onback} aria-label={t('mobile.back')} title={t('mobile.back')}><Icon name="arrow-left" /></button>
      <span class="mode" style={tintStyle(profile?.hue)}><Icon name={profile?.icon ?? 'message-circle'} size={15} /></span>
      <div class="title">
        <h1>{session.title || t('session.untitled')}</h1>
        <ActivityBadge state={session.state} />
      </div>
      <button class="btn ghost icon touch" onclick={() => (options = !options)} aria-expanded={options} aria-label={t('mobile.options')} title={t('mobile.options')}><Icon name="settings" /></button>
      <button class="btn ghost icon touch" onclick={() => app.togglePanel()} aria-pressed={app.outline.open} aria-label={t('outline.toggle')} title={t('outline.toggle')}><Icon name="table-of-contents" /></button>
    </header>
    {#if options}
      <div class="options">
        <Toolbar {session}>
          {#snippet end()}
            <ModeSwitch {session} />
            <button class="btn ghost small away" class:on={session.unattended} onclick={toggleUnattended} aria-pressed={!!session.unattended} title={t('unattended.hint', names)}>
              <Icon name="coffee" size={15} />{t('unattended.label')}
            </button>
          {/snippet}
        </Toolbar>
        {#if !session.archived}
          <p class="restart">
            <button class="btn ghost small away" onclick={() => app.restart(session.id)}><Icon name="rotate-ccw" size={15} />{t('restart.label')}</button>
            <span>{t('restart.hintShort', names)}</span>
          </p>
        {/if}
      </div>
    {/if}
    <!-- With the question compact, only an error (as the desktop's compact header). -->
    {#if !app.questionCompact || failed}
      <div class="notes">
        {#if !app.questionCompact}<SessionStatus {session} />{/if}
        {#if failed}<p class="err" role="alert"><Icon name="triangle-alert" size={14} />{session.error}</p>{/if}
      </div>
    {/if}
  </div>

  <div class="scroll" bind:this={scroller}>
    <div class="content">
      {#if page}
        <PageView {page} />
      {:else}
        <p class="empty">{t(`profile.${session.profileId}.desc`)}</p>
      {/if}
    </div>
  </div>

  {#if cards.length || checking}
    <div class="approvals">
      {#each cards as a (a.requestId)}
        <ApprovalCard {session} approval={a} />
      {/each}
      {#if checking}<p class="checking" role="status"><Icon name="loader" spin size={13} />{t('asking.checking')}</p>{/if}
    </div>
  {/if}
  <div class="dock"><NavBar /></div>
  <footer class="bottom">
    {#if session.archived}
      <div class="restore" role="status">
        <span>{t('archive.readOnly')}</span>
        <button class="btn primary small" onclick={restore}><Icon name="archive-restore" size={14} />{t('archive.restore')}</button>
      </div>
    {:else}
      <Composer {session} />
    {/if}
  </footer>

  {#if app.outline.open && (page || app.tabFor(session.id) === 'file')}
    <div class="panel" role="dialog" aria-label={t('outline.toggle')}>
      <button class="btn ghost icon touch close" onclick={() => app.togglePanel()} aria-label={t('common.close')}><Icon name="x" /></button>
      <SidePanel sessionId={session.id} page={page ?? null} {scroller} />
    </div>
  {/if}
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
  .head {
    background: var(--surface);
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-1);
  }
  .touch {
    width: 44px;
    height: 44px;
    flex: none;
  }
  .mode {
    display: grid;
    place-items: center;
    flex: none;
    width: 26px;
    height: 26px;
    border-radius: 8px;
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
  }
  .title {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-left: var(--space-2);
  }
  h1 {
    margin: 0;
    font-size: var(--text-md);
    font-weight: 620;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .options {
    padding: var(--space-2) var(--space-3);
    border-top: 1px solid var(--border);
  }
  /* On a phone the prompt level and Unattended start their line, like
     everything else in Options, rather than sitting at the right end of it. */
  .options :global(.end) {
    margin-left: 0;
    padding-left: 0;
  }
  /* The toolbar wraps here, so a divider between groups would end a line. */
  .options :global(.sep) {
    display: none;
  }
  .away {
    gap: 6px;
    color: var(--text-muted);
  }
  .away.on {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .restart {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .restart .btn {
    flex: none;
  }
  .notes {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 var(--space-3) var(--space-2);
  }
  .notes:empty {
    display: none;
  }
  .err {
    display: flex;
    gap: var(--space-2);
    margin: var(--space-1) 0;
    font-size: var(--text-sm);
    color: var(--danger);
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
  .panel :global(> *:last-child) {
    flex: 1;
    min-height: 0;
    width: 100% !important;
  }
</style>
