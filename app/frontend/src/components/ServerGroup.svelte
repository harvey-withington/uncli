<script lang="ts">
  // One paired server's sessions in the sidebar (decision 0031): its name
  // with a status dot, then its sessions, reordered among themselves. A
  // server that can't be reached keeps its heading and says why.
  import type { PeerState, SessionView } from '@uncli/chat/lib/api'
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { peerStatus } from '@uncli/chat/lib/servers'
  import PeerDot from './PeerDot.svelte'
  import SidebarList from './SidebarList.svelte'
  import SidebarHeading from './SidebarHeading.svelte'

  interface Props {
    peer: PeerState
    sessions: SessionView[]
  }

  let { peer, sessions }: Props = $props()
  const app = useApp()
  const status = $derived(peerStatus(peer))
</script>

<section class="group" aria-label={t('peers.group', { name: peer.name })}>
  <SidebarHeading icon="server" label={peer.name} title={status}>
    <PeerDot state={peer.state} />
    <span class="visually-hidden">{status}</span>
  </SidebarHeading>
  {#if peer.state === 'unpaired'}
    <p class="note">
      {t('peers.unpairedShort')}
      <button class="link" onclick={() => app.openSettings('peers')}>{t('peers.openSettings')}</button>
    </p>
  {:else if sessions.length === 0}
    <p class="note">{peer.state === 'online' ? t('peers.noSessions') : status}</p>
  {:else}
    <SidebarList {sessions} />
  {/if}
</section>

<style>
  .group {
    margin-top: var(--space-4);
  }
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
