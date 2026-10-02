<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { theme, cycleTheme } from '../lib/theme.svelte'
  import Icon from './Icon.svelte'
  import ResizeHandle from './ResizeHandle.svelte'
  import SearchPanel from './SearchPanel.svelte'
  import SessionItem from './SessionItem.svelte'
  import { SIDEBAR_MAX, SIDEBAR_MIN } from '../lib/panels'
  import { dragSort } from '../lib/actions'

  const app = useApp()
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
    {:else if app.sessions.length === 0}
      <p class="empty">{t('session.none')}</p>
    {:else}
      <ul use:dragSort={{ item: 'li[data-id]', onmove: (id, to) => app.moveSession(id, to) }}>
        {#each app.sessions as session (session.id)}
          <SessionItem {session} active={session.id === app.currentId} />
        {/each}
      </ul>
    {/if}
  </nav>

  <footer>
    <button class="btn ghost small" onclick={() => (app.settingsOpen = true)} title={t('settings.title')}>
      <Icon name="settings" size={14} />
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
  .brand {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
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
  footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: var(--space-3);
    border-top: 1px solid var(--border);
  }
</style>
