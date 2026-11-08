<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { theme, cycleTheme } from '../lib/theme.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import PinnedList from './PinnedList.svelte'
  import ResizeHandle from '@uncli/chat/components/ResizeHandle.svelte'
  import SearchPanel from './SearchPanel.svelte'
  import ServerGroup from './ServerGroup.svelte'
  import SessionGroup from './SessionGroup.svelte'
  import UpdateNotice from './UpdateNotice.svelte'
  import { SIDEBAR_MAX, SIDEBAR_MIN } from '@uncli/chat/lib/panels'
  import { groupSessions } from '@uncli/chat/lib/servers'
  import { LOCAL } from '@uncli/chat/lib/api'

  const app = useApp()
  // The tab that needs the user, if any: the dot opens Settings there.
  const attentionAt = $derived((['containers', 'providers'] as const).find(k => (app.attention[k]?.length ?? 0) > 0) ?? '')
  const attentionTitle = $derived(
    attentionAt ? [t('attention.label'), ...Object.values(app.attention).flat().map(k => t(k as string))].join('\n') : t('settings.title'),
  )
  // This PC's sessions, then each paired server's (decision 0031), each
  // with its own archived ones.
  const groups = $derived(groupSessions(app.sessions, app.servers))
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
      <SessionGroup key={LOCAL} icon="monitor" label={t('peers.thisPC')} sessions={groups.local.sessions} archived={groups.local.archived}>
        {#snippet empty()}<p class="empty">{t('session.none')}</p>{/snippet}
      </SessionGroup>
      {#each groups.servers as g (g.peer.key)}
        <ServerGroup group={g} />
      {/each}
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
