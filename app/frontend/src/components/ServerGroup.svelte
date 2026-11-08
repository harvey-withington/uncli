<script lang="ts">
  // One paired server's group in the sidebar (decision 0031): a
  // SessionGroup with its status dot. A server that can't be reached keeps
  // its heading and says why; one that no longer accepts this PC says so,
  // with Settings to pair again.
  import type { ServerGroup } from '@uncli/chat/lib/servers'
  import { peerStatus } from '@uncli/chat/lib/servers'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { useApp } from '../lib/context'
  import SessionGroup from './SessionGroup.svelte'

  let { group }: { group: ServerGroup } = $props()
  const app = useApp()
  const peer = $derived(group.peer)
  const status = $derived(peerStatus(peer))
</script>

<SessionGroup key={peer.key} icon="server" label={peer.name} sessions={group.sessions} archived={group.archived} dot={peer.state} {status}>
  {#snippet empty()}
    {#if peer.state === 'unpaired'}
      <p class="note">
        {t('peers.unpairedShort')}
        <button class="link" onclick={() => app.openSettings('peers')}>{t('peers.openSettings')}</button>
      </p>
    {:else}
      <p class="note">{peer.state === 'online' ? t('peers.noSessions') : status}</p>
    {/if}
  {/snippet}
</SessionGroup>

<style>
  .note {
    margin: 0;
    padding: var(--space-1) var(--space-2) var(--space-1) var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .link {
    border: 0;
    padding: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
</style>
