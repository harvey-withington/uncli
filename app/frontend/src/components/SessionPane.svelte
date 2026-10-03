<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { tintStyle } from '../lib/tint'
  import { showToast } from '../lib/toasts.svelte'
  import { blockMatching } from '../lib/search'
  import ApprovalCard from './ApprovalCard.svelte'
  import ActivityBadge from './ActivityBadge.svelte'
  import Composer from './Composer.svelte'
  import Icon from './Icon.svelte'
  import NavBar from './NavBar.svelte'
  import OutlinePanel from './OutlinePanel.svelte'
  import PageView from './PageView.svelte'
  import Toolbar from './Toolbar.svelte'

  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const profile = $derived(app.boot?.profiles.find(p => p.id === session.profileId))
  const page = $derived(app.currentPage)
  let scroller: HTMLElement | undefined = $state()

  // Unattended: rules still apply, but anything that would wait for the
  // user is declined, and Claude is told the user is away.
  async function toggleUnattended() {
    try {
      app.upsertSession(await app.backend.setUnattended(session.id, !session.unattended))
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  // A different page starts at the top.
  $effect(() => {
    if (page?.id && scroller) scroller.scrollTop = 0
  })

  // Opened from search: once the page has rendered, scroll to the first
  // block holding a search word (below the sticky question) and flash it.
  $effect(() => {
    const r = app.reveal
    const el = scroller
    if (!r || !el || page?.id !== r.pageId) return
    const frame = window.requestAnimationFrame ?? ((f: FrameRequestCallback) => window.setTimeout(f, 16))
    frame(() => frame(() => {
      if (app.reveal !== r) return
      app.reveal = null
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
  <header class="top">
    <div class="title-row">
      <span class="mode" style={tintStyle(profile?.hue)}><Icon name={profile?.icon ?? 'message-circle'} size={15} /></span>
      <h1 title={session.title}>{session.title || t('session.untitled')}</h1>
      <ActivityBadge state={session.state} />
      <button
        class="btn ghost small away-btn"
        class:on={session.unattended}
        onclick={toggleUnattended}
        aria-pressed={!!session.unattended}
        title={t('unattended.hint')}
      >
        <Icon name="coffee" size={15} />{t('unattended.label')}
      </button>
      <button
        class="btn ghost small icon"
        onclick={() => (app.permissionsOpen = true)}
        aria-label={t('rules.open')}
        title={t('rules.open')}
      >
        <Icon name="shield" />
      </button>
      <button
        class="btn ghost small icon"
        onclick={() => app.toggleOutline()}
        aria-pressed={app.outline.open}
        aria-label={t('outline.toggle')}
        title={t('outline.toggle')}
      >
        <Icon name={app.outline.open ? 'panel-right-close' : 'panel-right-open'} />
      </button>
    </div>
    <span class="workdir" title={session.workdir}>{profile?.label} · {session.workdir}</span>
    {#if session.unattended}
      <p class="away" role="status"><Icon name="coffee" size={14} />{t('unattended.on')}</p>
    {/if}
    <Toolbar {session} />
    {#if session.error && session.state === 'error'}
      <p class="err" role="alert"><Icon name="triangle-alert" size={14} />{session.error}</p>
    {/if}
  </header>

  <div class="middle">
    <!-- The answer column: the page, the page controls floating on the line
         above the composer (centred over the answer, not the outline), and
         the composer. -->
    <div class="main">
      <div class="scroll" bind:this={scroller}>
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
      {#if session.approvals?.length}
        <div class="approvals">
          {#each session.approvals as a (a.requestId)}
            <ApprovalCard {session} approval={a} />
          {/each}
        </div>
      {/if}
      <div class="dock"><NavBar /></div>
      <footer class="bottom">
        <div class="inner">
          <Composer {session} />
        </div>
      </footer>
    </div>
    {#if page && app.outline.open}
      <OutlinePanel {page} {scroller} />
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
  .top {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-6) var(--space-3);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .title-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }
  .mode {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 8px;
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
    flex: none;
  }
  h1 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 620;
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .workdir {
    display: block;
    margin: -4px 0 2px 38px;
    font-size: var(--text-xs);
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .err {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-sm);
    color: var(--danger);
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
    z-index: 4;
    height: 0;
    display: flex;
    align-items: flex-start; /* don't squash the pill to the row's zero height */
    justify-content: center;
  }
  .dock :global(.navbar) {
    transform: translateY(-50%);
  }
  .away-btn {
    margin-left: auto;
    gap: 6px;
    color: var(--text-muted);
  }
  .away-btn.on {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .away {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0 0 0 38px;
    font-size: var(--text-xs);
    color: var(--warning);
  }
  /* Approval cards sit just above the page controls, over the end of the
     answer, so they're in view whatever page is showing. */
  .approvals {
    position: relative;
    z-index: 5;
    max-height: 55%;
    overflow-y: auto;
    padding: var(--space-2) var(--space-6) var(--space-5);
    margin-top: calc(-1 * var(--space-4));
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
  .inner {
    max-width: calc(var(--reading-width) + 2 * var(--space-6));
    margin: 0 auto;
  }
</style>
