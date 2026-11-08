<script lang="ts">
  // One group of the sidebar's sessions (decision 0031): This PC's, or a
  // paired server's. Its heading folds the group away; the archive button
  // at its right end hides the group's archived sessions altogether. Under
  // the active sessions, the group's own Archived (n) opens its archived
  // ones. All three are remembered per group (lib/sidebargroups.ts).
  import type { Snippet } from 'svelte'
  import type { PeerState, SessionView } from '@uncli/chat/lib/api'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import { useApp } from '../lib/context'
  import PeerDot from './PeerDot.svelte'
  import SidebarList from './SidebarList.svelte'

  interface Props {
    key: string // LOCAL, or the server's key
    icon: string
    label: string
    sessions: SessionView[] // active
    archived: SessionView[]
    dot?: PeerState['state'] // a server's connection
    status?: string // what the dot means, for its title and screen readers
    empty: Snippet // what shows with no active sessions
  }

  let { key, icon, label, sessions, archived, dot, status, empty }: Props = $props()
  const app = useApp()
  const g = $derived(app.groupState(key))
  const hideLabel = $derived(
    archived.length === 0 ? t('group.noArchived') : g.hideArchived ? t('group.showArchived', { n: archived.length }) : t('group.hideArchived'),
  )
</script>

<section class="group" aria-label={t('group.label', { name: label })}>
  <div class="head">
    <button class="fold" onclick={() => app.toggleGroup(key, 'collapsed')} aria-expanded={!g.collapsed} title={status}>
      <span class="chev" class:open={!g.collapsed}><Icon name="chevron-right" size={12} /></span>
      <Icon name={icon} size={12} />
      <span class="label">{label}</span>
      {#if dot}<PeerDot state={dot} />{/if}
      {#if status}<span class="visually-hidden">{status}</span>{/if}
      {#if g.collapsed && sessions.length}<span class="count">{sessions.length}</span>{/if}
    </button>
    <button
      class="btn ghost small icon hide"
      class:on={g.hideArchived}
      onclick={() => app.toggleGroup(key, 'hideArchived')}
      disabled={archived.length === 0}
      aria-pressed={!!g.hideArchived}
      aria-label={hideLabel}
      title={hideLabel}
    >
      <Icon name={g.hideArchived ? 'archive-x' : 'archive'} size={13} />
    </button>
  </div>
  {#if !g.collapsed}
    {#if sessions.length}
      <SidebarList {sessions} />
    {:else}
      {@render empty()}
    {/if}
    {#if archived.length && !g.hideArchived}
      <button class="archived-toggle" onclick={() => app.toggleGroup(key, 'archivedOpen')} aria-expanded={!!g.archivedOpen}>
        <Icon name="archive" size={13} />
        <span>{t('archive.list', { n: archived.length })}</span>
        <span class="chev" class:open={g.archivedOpen}><Icon name="chevron-right" size={12} /></span>
      </button>
      {#if g.archivedOpen}
        <div class="archived"><SidebarList sessions={archived} sortable={false} dim label={t('group.archived', { name: label })} /></div>
      {/if}
    {/if}
  {/if}
</section>

<style>
  .group + :global(.group) {
    margin-top: var(--space-4);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    margin: 0 0 var(--space-1);
  }
  .fold {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    padding: 2px var(--space-2) 2px 2px;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    text-align: left;
  }
  .fold:hover {
    color: var(--text);
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .count {
    margin-left: auto;
    font-weight: 500;
    letter-spacing: 0;
  }
  .chev {
    display: inline-flex;
    transition: transform var(--normal) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .hide {
    flex: none;
    color: var(--text-faint);
  }
  .hide.on {
    color: var(--accent);
  }
  .archived-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    margin-top: var(--space-2);
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
  }
  .archived {
    margin-top: var(--space-1);
  }
  @media (prefers-reduced-motion: reduce) {
    .chev {
      transition: none;
    }
  }
</style>
