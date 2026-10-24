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
    </div>
  {/if}
  <div class="notes">
    <SessionStatus {session} />
    {#if failed}<p class="err" role="alert"><Icon name="triangle-alert" size={14} />{session.error}</p>{/if}
  </div>

  <div class="scroll" bind:this={scroller}>
    {#if page}
      <PageView {page} />
    {:else}
      <p class="empty">{t(`profile.${session.profileId}.desc`)}</p>
    {/if}
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
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-1);
    background: var(--surface);
    border-bottom: 1px solid var(--border);
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
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }
  .away {
    gap: 6px;
    color: var(--text-muted);
  }
  .away.on {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .notes {
    padding: 0 var(--space-3);
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
  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-3) var(--space-3) var(--space-5);
    overscroll-behavior: contain;
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
