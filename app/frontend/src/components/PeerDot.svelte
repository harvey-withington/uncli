<script lang="ts">
  // A paired server's connection as a dot: green online, amber connecting,
  // red unpaired, grey offline. The words go beside it or in a title.
  import type { PeerState } from '@uncli/chat/lib/api'

  let { state, size = 7 }: { state: PeerState['state']; size?: number } = $props()
</script>

<span class="dot {state}" style:width="{size}px" style:height="{size}px" aria-hidden="true"></span>

<style>
  .dot {
    flex: none;
    border-radius: 50%;
    background: var(--text-faint);
  }
  .online {
    background: var(--success);
  }
  .connecting {
    background: var(--warning);
  }
  .unpaired {
    background: var(--danger);
  }
</style>
