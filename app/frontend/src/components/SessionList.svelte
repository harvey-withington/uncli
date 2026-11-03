<script lang="ts">
  import { onMount } from 'svelte'
  import { dragSort } from '../lib/actions'
  import { provideChat } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import type { ChatStore } from '../stores/chat.svelte'
  import ConfirmDialog from './ConfirmDialog.svelte'
  import Icon from './Icon.svelte'
  import SessionItem from './SessionItem.svelte'
  import Toasts from './Toasts.svelte'

  // A server's sessions as a list of their own (decision 0026): what the
  // <uncli-session-list> element shows. The window's sidebar list, without
  // the app around it: drag (or Alt+arrows) to reorder, rename, archive,
  // delete, and the archived ones apart. Picking one tells the host
  // (onpick), which shows it, in an <uncli-chat> say.

  interface Props {
    store: ChatStore
    selected: string // the session the host shows, marked as current
    onpick: (id: string) => void
  }

  let { store, selected, onpick }: Props = $props()
  // svelte-ignore state_referenced_locally
  provideChat(store)
  let failed = $state('')
  let showArchived = $state(false)

  onMount(() => {
    store.init().catch(e => (failed = e instanceof Error ? e.message : String(e)))
  })
</script>

<nav class="list" aria-label={t('session.list')}>
  {#if failed}
    <p class="note" role="alert"><Icon name="triangle-alert" />{failed}</p>
  {:else if !store.ready}
    <p class="note" aria-busy="true"><Icon name="loader" spin size={18} label={t('app.loading')} /></p>
  {:else}
    {#if store.activeSessions.length === 0}
      <p class="note">{t('session.none')}</p>
    {:else}
      <ul use:dragSort={{ item: 'li[data-id]', onmove: (id, to) => store.moveSession(id, to) }}>
        {#each store.activeSessions as session (session.id)}
          <SessionItem {session} active={session.id === selected} {onpick} />
        {/each}
      </ul>
    {/if}
    {#if store.archivedSessions.length > 0}
      <button class="archived-toggle" onclick={() => (showArchived = !showArchived)} aria-expanded={showArchived}>
        <Icon name="archive" size={13} />
        <span>{t('archive.list', { n: store.archivedSessions.length })}</span>
        <span class="chev" class:open={showArchived}><Icon name="chevron-right" size={12} /></span>
      </button>
      {#if showArchived}
        <ul class="archived" aria-label={t('archive.title')}>
          {#each store.archivedSessions as session (session.id)}
            <SessionItem {session} active={session.id === selected} {onpick} />
          {/each}
        </ul>
      {/if}
    {/if}
  {/if}
  <Toasts />
  <ConfirmDialog />
</nav>

<style>
  .list {
    position: relative;
    height: 100%;
    overflow-y: auto;
    padding: var(--space-2);
  }
  ul {
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  /* Drag to reorder, as in the sidebar. */
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
  .note {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
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
</style>
