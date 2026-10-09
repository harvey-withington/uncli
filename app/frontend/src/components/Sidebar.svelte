<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { theme, cycleTheme } from '../lib/theme.svelte'
  import Icon from './Icon.svelte'
  import PinnedList from './PinnedList.svelte'
  import ResizeHandle from './ResizeHandle.svelte'
  import SearchPanel from './SearchPanel.svelte'
  import SessionItem from './SessionItem.svelte'
  import { SIDEBAR_MAX, SIDEBAR_MIN } from '../lib/panels'
  import { dragSort } from '../lib/actions'

  const app = useApp()
  // The tab that needs the user, if any: the dot opens Settings there.
  const attentionAt = $derived((['containers', 'providers'] as const).find(k => (app.attention[k]?.length ?? 0) > 0) ?? '')
  const attentionTitle = $derived(
    attentionAt ? [t('attention.label'), ...Object.values(app.attention).flat().map(k => t(k as string))].join('\n') : t('settings.title'),
  )
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
    <img class="mark" src="/uncli-mark.svg" alt="" width="20" height="24" />
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
      {#if app.activeSessions.length === 0}
        <p class="empty">{t('session.none')}</p>
      {:else}
        <ul use:dragSort={{ item: 'li[data-id]', onmove: (id, to) => app.moveSession(id, to) }}>
          {#each app.activeSessions as session (session.id)}
            <SessionItem {session} active={session.id === app.currentId} />
          {/each}
        </ul>
      {/if}
      {#if app.archivedSessions.length > 0}
        <button class="archived-toggle" onclick={() => (app.showArchived = !app.showArchived)} aria-expanded={app.showArchived}>
          <Icon name="archive" size={13} />
          <span>{t('archive.list', { n: app.archivedSessions.length })}</span>
          <span class="chev" class:open={app.showArchived}><Icon name="chevron-right" size={12} /></span>
        </button>
        {#if app.showArchived}
          <ul class="archived" aria-label={t('archive.title')}>
            {#each app.archivedSessions as session (session.id)}
              <SessionItem {session} active={session.id === app.currentId} />
            {/each}
          </ul>
        {/if}
      {/if}
    {/if}
  </nav>

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
    width: 20px;
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
  ul {
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  /* Drag to reorder: the others slide out of the way, and the dragged
     item's own place becomes an empty slot where it will land. */
  ul:global(.sorting) :global(li) {
    transition: transform 180ms var(--ease), background var(--fast) var(--ease);
  }
  ul:global(.settled) :global(li) {
    transition: none;
  }
  ul :global(li.dragging) {
    background: var(--accent-soft);
    box-shadow: inset 0 0 0 1.5px color-mix(in srgb, var(--accent) 45%, transparent);
  }
  ul :global(li.dragging > *) {
    visibility: hidden;
  }
  @media (prefers-reduced-motion: reduce) {
    ul:global(.sorting) :global(li) {
      transition: none;
    }
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
    opacity: 0.75;
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
