<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { theme, cycleTheme } from '../lib/theme.svelte'
  import Icon from './Icon.svelte'
  import SessionItem from './SessionItem.svelte'

  const app = useApp()
  const themeIcon = $derived(theme.value === 'light' ? 'sun' : theme.value === 'dark' ? 'moon' : 'monitor')
</script>

<aside class="sidebar">
  <div class="brand">
    <span class="wordmark">UNCLI</span>
    <span class="tag">{t('app.tagline')}</span>
  </div>

  <button class="btn primary new" onclick={() => (app.newSessionOpen = true)}>
    <Icon name="plus" />
    {t('session.new')}
    <kbd>Ctrl N</kbd>
  </button>

  <nav aria-label={t('session.list')}>
    {#if app.sessions.length === 0}
      <p class="empty">{t('session.none')}</p>
    {:else}
      <ul>
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
    width: var(--sidebar-width);
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
