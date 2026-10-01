<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import ActivityBadge from './ActivityBadge.svelte'
  import Composer from './Composer.svelte'
  import Icon from './Icon.svelte'
  import NavBar from './NavBar.svelte'
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

  // A different page starts at the top.
  $effect(() => {
    if (page?.id && scroller) scroller.scrollTop = 0
  })
</script>

<section class="pane">
  <header class="top">
    <div class="title-row">
      <span class="mode"><Icon name={profile?.icon ?? 'message-circle'} size={15} /></span>
      <h1 title={session.title}>{session.title || t('session.untitled')}</h1>
      <ActivityBadge state={session.state} />
    </div>
    <span class="workdir" title={session.workdir}>{profile?.label} · {session.workdir}</span>
    <Toolbar {session} />
    {#if session.error && session.state === 'error'}
      <p class="err" role="alert"><Icon name="triangle-alert" size={14} />{session.error}</p>
    {/if}
  </header>

  <div class="scroll" bind:this={scroller}>
    {#if page}
      <PageView {page} />
    {:else}
      <div class="empty">
        <span class="eicon"><Icon name={profile?.icon ?? 'message-circle'} size={22} /></span>
        <h2>{t(`profile.${session.profileId}.empty`)}</h2>
        <p>{t(`profile.${session.profileId}.desc`)}</p>
      </div>
    {/if}
  </div>

  <footer class="bottom">
    <div class="inner">
      <NavBar />
      <Composer {session} />
    </div>
  </footer>
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
    background: var(--accent-soft);
    color: var(--accent);
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
  .scroll {
    flex: 1;
    overflow-y: auto;
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
    background: var(--accent-soft);
    color: var(--accent);
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
  .bottom {
    padding: var(--space-3) var(--space-6) var(--space-4);
    border-top: 1px solid var(--border);
    background: var(--bg);
  }
  .inner {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-width: calc(var(--reading-width) + 2 * var(--space-6));
    margin: 0 auto;
  }
</style>
