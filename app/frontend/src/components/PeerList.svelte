<script lang="ts">
  import { useShell } from '@uncli/chat/lib/shell.svelte'
  import type { PeerState } from '@uncli/chat/lib/api'
  import { useApp } from '../lib/context'
  import { relativeTime } from '@uncli/chat/lib/format'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { peerStatus } from '@uncli/chat/lib/servers'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import PeerDot from './PeerDot.svelte'

  // Settings → Remote servers → the paired servers (decision 0031): each
  // with its address, whether it's reachable (or why not), its UNCLI
  // version and Remove. Empty, it says how to add one.
  const app = useApp()
  const shell = useShell()
  const line = (...parts: (string | false | undefined)[]) => parts.filter(Boolean).join(' · ')

  async function remove(p: PeerState) {
    const ok = await shell.confirm({ title: t('peers.removeTitle', { name: p.name }), message: t('peers.removeBody', { name: p.name }), confirmLabel: t('peers.remove'), danger: true })
    if (!ok) return
    try {
      await app.backend.removeRemoteServer(p.key)
      app.servers = app.servers.filter(s => s.key !== p.key)
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

<h4>{t('peers.list')}</h4>
{#if app.servers.length === 0}
  <p class="empty">{t('peers.none')}</p>
{:else}
  <ul aria-label={t('peers.list')}>
    {#each app.servers as p (p.key)}
      <li>
        <PeerDot state={p.state} size={8} />
        <div class="who">
          <strong>{p.name}</strong>
          <span>{line(p.key, p.app && t('peers.version', { version: p.app }))}</span>
          <span>{line(peerStatus(p), p.state !== 'online' && p.lastOnline && t('peers.lastOnline', { when: relativeTime(Date.parse(p.lastOnline)) }))}</span>
        </div>
        <button class="btn small" onclick={() => remove(p)}><Icon name="trash" size={13} />{t('peers.remove')}</button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  h4 {
    margin: var(--space-4) 0 var(--space-2);
    font-size: var(--text-sm);
  }
  .empty {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    color: var(--text-muted);
  }
  li + li {
    border-top: 1px solid var(--border);
  }
  .who {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    font-size: var(--text-sm);
  }
  .who strong {
    color: var(--text);
    font-weight: 500;
  }
  .who span {
    font-size: var(--text-xs);
    overflow-wrap: anywhere;
  }
</style>
