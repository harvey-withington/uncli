<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import SessionItem from '../components/SessionItem.svelte'
  import UpdateNotice from '../components/UpdateNotice.svelte'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'

  // The phone's first screen: the server's sessions, newest activity
  // first as on the desktop, a button for a new one, and Settings. Tapping
  // a session opens it.

  interface Props {
    server: string
    onsettings: () => void
  }

  let { server, onsettings }: Props = $props()
  const app = useApp()
  let showArchived = $state(false)
  const online = $derived(!app.connection || app.connection.state === 'online')
</script>

<header class="bar">
  <img src="./icon-192.png" alt="" width="28" height="28" />
  <div class="who">
    <strong>UNCLI</strong>
    <span class="server"><span class="dot" class:online aria-hidden="true"></span>{server}</span>
  </div>
  <button class="btn ghost icon" onclick={onsettings} aria-label={t('mobile.settings')} title={t('mobile.settings')}><Icon name="settings" /></button>
</header>

<main class="home">
  {#if !app.cliReady}
    <p class="note" role="status"><Icon name="triangle-alert" size={15} />{t('mobile.cliNotReady')}</p>
  {/if}
  <UpdateNotice />
  <button class="btn primary new" onclick={() => app.openNewSession()} disabled={!app.cliReady}>
    <Icon name="plus" size={16} />{t('session.new')}
  </button>
  {#if app.activeSessions.length === 0}
    <p class="empty">{t('mobile.noSessions')}</p>
  {:else}
    <ul class="list" aria-label={t('mobile.sessions')}>
      {#each app.activeSessions as s (s.id)}
        <SessionItem session={s} active={false} />
      {/each}
    </ul>
  {/if}
  {#if app.archivedSessions.length}
    <button class="btn ghost small more" onclick={() => (showArchived = !showArchived)} aria-expanded={showArchived}>
      <Icon name="archive" size={14} />{t('archive.list', { n: app.archivedSessions.length })}
    </button>
    {#if showArchived}
      <ul class="list" aria-label={t('archive.title')}>
        {#each app.archivedSessions as s (s.id)}
          <SessionItem session={s} active={false} />
        {/each}
      </ul>
    {/if}
  {/if}
</main>

<style>
  .bar {
    position: sticky;
    top: 0;
    z-index: 5;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-2) var(--space-2) var(--space-4);
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }
  .who {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .server {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dot {
    flex: none;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--warning);
  }
  .dot.online {
    background: var(--success);
  }
  .home {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-3) var(--space-6);
  }
  .new {
    min-height: 44px;
    justify-content: center;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0;
    padding: 0;
  }
  .empty,
  .note {
    margin: var(--space-4) var(--space-2);
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .note {
    display: flex;
    gap: var(--space-2);
    margin: 0;
  }
  .more {
    align-self: flex-start;
    min-height: 44px;
  }
</style>
