<script lang="ts">
  import type { PinnedPage } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { tintStyle } from '../lib/tint'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // The sidebar's Pinned group: pages pinned in any session, newest first.
  // Bookmarks mark places within a session for paging; pins keep answers
  // at hand across sessions. Clicking one opens it; the pin-off button unpins.

  const app = useApp()
  let open = $state(true)

  const sessionOf = (p: PinnedPage) => app.sessions.find(s => s.id === p.sessionId)
  const profileOf = (p: PinnedPage) => app.boot?.profiles.find(x => x.id === p.profileId)
  const line = (q: string) => q.replace(/\s+/g, ' ').trim()

  async function unpin(p: PinnedPage) {
    try {
      await app.setPinned(p.sessionId, p.pageId, false)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

{#if app.pins.length > 0}
  <section class="pinned">
    <button class="head" onclick={() => (open = !open)} aria-expanded={open}>
      <Icon name="pin" size={12} />
      <span>{t('pins.title')}</span>
      <span class="count">{app.pins.length}</span>
      <span class="chev" class:open><Icon name="chevron-right" size={12} /></span>
    </button>
    {#if open}
      <ul aria-label={t('pins.title')}>
        {#each app.pins as p (p.pageId)}
          {@const profile = profileOf(p)}
          {@const title = sessionOf(p)?.title || p.sessionTitle || t('session.untitled')}
          <li style={tintStyle(profile?.hue)}>
            <button
              class="pin"
              class:current={app.currentPage?.id === p.pageId}
              onclick={() => app.openPin(p)}
              title={`${line(p.question)} · ${title}, ${t('page.number', { n: p.seq })}`}
            >
              <span class="mode"><Icon name={profile?.icon ?? 'message-circle'} size={13} label={profile?.label} /></span>
              <span class="text">
                <span class="q">{line(p.question) || t('session.untitled')}</span>
                <span class="where">{p.archived ? `${title} · ${t('archive.archived')}` : title}</span>
              </span>
            </button>
            <button class="btn ghost small icon unpin" onclick={() => unpin(p)} aria-label={t('pins.unpin', { q: line(p.question) })} title={t('pins.unpinShort')}>
              <Icon name="pin-off" size={13} />
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

<style>
  .pinned {
    margin: 0 0 var(--space-2);
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--border);
  }
  .head {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: var(--space-1) var(--space-2);
    border: 0;
    background: none;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    text-align: left;
  }
  .head:hover {
    color: var(--text);
  }
  .count {
    padding: 0 6px;
    border-radius: 999px;
    background: var(--surface-3);
    font-size: 11px;
    line-height: 16px;
  }
  .chev {
    margin-left: auto;
    display: inline-flex;
    transition: transform var(--normal) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    position: relative;
    display: flex;
    align-items: center;
  }
  .pin {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: 6px var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    text-align: left;
  }
  .pin:hover,
  .pin.current {
    background: var(--surface-2);
  }
  .mode {
    display: inline-flex;
    padding-top: 2px;
    color: var(--tint, var(--accent));
  }
  .text {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .q,
  .where {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .q {
    font-size: var(--text-sm);
    color: var(--text);
  }
  .where {
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .unpin {
    position: absolute;
    right: 2px;
    opacity: 0;
    background: var(--surface-2);
  }
  li:hover .unpin,
  .unpin:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .unpin {
      opacity: 1;
    }
  }
</style>
