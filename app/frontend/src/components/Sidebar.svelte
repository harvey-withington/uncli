<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { theme, cycleTheme } from '../lib/theme.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import PinnedList from './PinnedList.svelte'
  import ResizeHandle from '@uncli/chat/components/ResizeHandle.svelte'
  import SearchPanel from './SearchPanel.svelte'
  import ServerGroup from './ServerGroup.svelte'
  import SidebarHeading from './SidebarHeading.svelte'
  import SidebarList from './SidebarList.svelte'
  import UpdateNotice from './UpdateNotice.svelte'
  import { SIDEBAR_MAX, SIDEBAR_MIN } from '@uncli/chat/lib/panels'
  import { groupSessions } from '@uncli/chat/lib/servers'

  const app = useApp()
  // The tab that needs the user, if any: the dot opens Settings there.
  const attentionAt = $derived((['containers', 'providers'] as const).find(k => (app.attention[k]?.length ?? 0) > 0) ?? '')
  const attentionTitle = $derived(
    attentionAt ? [t('attention.label'), ...Object.values(app.attention).flat().map(k => t(k as string))].join('\n') : t('settings.title'),
  )
  // This PC's sessions, then each paired server's (decision 0031).
  const groups = $derived(groupSessions(app.activeSessions, app.servers))
  const themeIcon = $derived(theme.value === 'light' ? 'sun' : theme.value === 'dark' ? 'moon' : 'monitor')
</script>

<aside class="sidebar" style:width="{app.sidebarWidth}px">
  <ResizeHandle
    edge="right"
    width={app.sidebarWidth}
    min={SIDEBAR_MIN}
    max={SIDEBAR_MAX}
    label={t('sidebar.resize')}
    onresize={w => (app.sidebarWidth = w)}
    oncommit={w => app.setSidebarWidth(w)}
  />
  <div class="brand">
    <img class="mark" src="/uncli-mark.svg" alt="" width="24" height="24" />
    <span class="wordmark">UNCLI</span>
    <span class="tag">{t('app.tagline')}</span>
  </div>

  <button class="btn primary new" onclick={() => app.openNewSession()}>
    <Icon name="plus" />
    {t('session.new')}
    <kbd>Ctrl N</kbd>
  </button>

  <SearchPanel />

  <nav aria-label={t('session.list')}>
    {#if app.searchText.trim()}
      <SearchPanel results />
    {:else}
      <PinnedList />
      {#if groups.servers.length > 0}
        <SidebarHeading icon="monitor" label={t('peers.thisPC')} />
      {/if}
      {#if groups.local.length === 0}
        <p class="empty">{t('session.none')}</p>
      {:else}
        <SidebarList sessions={groups.local} />
      {/if}
      {#each groups.servers as g (g.peer.key)}
        <ServerGroup peer={g.peer} sessions={g.sessions} />
      {/each}
      {#if app.archivedSessions.length > 0}
        <button class="archived-toggle" onclick={() => (app.showArchived = !app.showArchived)} aria-expanded={app.showArchived}>
          <Icon name="archive" size={13} />
          <span>{t('archive.list', { n: app.archivedSessions.length })}</span>
          <span class="chev" class:open={app.showArchived}><Icon name="chevron-right" size={12} /></span>
        </button>
        {#if app.showArchived}
          <div class="archived"><SidebarList sessions={app.archivedSessions} sortable={false} dim label={t('archive.title')} /></div>
        {/if}
      {/if}
    {/if}
  </nav>

  <UpdateNotice />
  <footer>
    <button class="btn ghost small settings" onclick={() => app.openSettings(attentionAt)} title={attentionTitle}>
      <span class="gear"><Icon name="settings" size={14} />{#if attentionAt}<span class="dot" aria-hidden="true"></span>{/if}</span>
      {#if attentionAt}<span class="visually-hidden">{t('attention.label')}</span>{/if}
      {t('settings.cli', { version: app.cli?.version ?? '…' })}
    </button>
    <button class="btn ghost small icon" onclick={cycleTheme} aria-label={t(`theme.${theme.value}`)} title={t(`theme.${theme.value}`)}>
      <Icon name={themeIcon} size={14} />
    </button>
  </footer>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    position: relative;
    flex: none;
    height: 100%;
    padding: var(--space-4) var(--space-3) var(--space-3);
    border-right: 1px solid var(--border);
    background: var(--bg);
  }
  /* The tagline drops under the wordmark when the sidebar is too narrow for
     both on one line. */
  .brand {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px var(--space-2);
    padding: var(--space-1) var(--space-2) var(--space-4);
  }
  .mark {
    align-self: center;
    width: 24px;
    height: 24px;
  }
  .wordmark {
    font-weight: 750;
    letter-spacing: 0.06em;
    font-size: 15px;
  }
  .tag {
    font-size: var(--text-xs);
    color: var(--text-faint);
    text-wrap: balance;
  }
  .new {
    justify-content: flex-start;
    height: 36px;
    margin: 0 var(--space-1) var(--space-4);
  }
  .new kbd {
    margin-left: auto;
    font-size: 10.5px;
    opacity: 0.7;
    font-family: var(--font);
  }
  nav {
    position: relative; /* clips absolutely positioned descendants (as .scroll in SessionPane) */
    flex: 1;
    overflow-y: auto;
    margin: 0 calc(-1 * var(--space-1));
    padding: 0 var(--space-1);
  }
  .empty {
    color: var(--text-faint);
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
  }
  .archived-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    margin-top: var(--space-3);
    padding: var(--space-1) var(--space-2);
    border: 0;
    background: none;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    text-align: left;
  }
  .archived-toggle:hover {
    color: var(--text);
  }
  .archived-toggle .chev {
    margin-left: auto;
    display: inline-flex;
    transition: transform var(--normal) var(--ease);
  }
  .archived-toggle .chev.open {
    transform: rotate(90deg);
  }
  .archived {
    margin-top: var(--space-1);
  }
  .gear {
    position: relative;
    display: inline-flex;
  }
  /* Something in Settings needs the user (lib/attention.ts). */
  .dot {
    position: absolute;
    top: -3px;
    right: -3px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--warning);
    box-shadow: 0 0 0 2px var(--bg);
  }
  footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: var(--space-3);
    border-top: 1px solid var(--border);
  }
</style>
