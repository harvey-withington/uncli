<script lang="ts">
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { remoteLine } from '../lib/remote'
  import { atServer } from '../lib/remoteonly'
  import Icon from './Icon.svelte'

  // The header's Remote access button (decision 0021): green while paired
  // devices can reach this PC, warning while only this PC can (Tailscale
  // isn't up), danger when it can't listen, plain when off. It opens
  // Settings → Remote access. Only in the server's own window.
  const chat = useChat()
  const here = $derived(atServer(chat.backend))
  const tone = $derived(chat.remote ? remoteLine(chat.remote).tone : 'muted')
  const label = $derived(t(`remote.button.${tone}`))
</script>

{#if here && chat.remote && chat.openSettings}
  {@const open = chat.openSettings}
  <button class="btn ghost small icon {tone}" onclick={() => open('remote')} aria-label={label} title={label}>
    <Icon name="radio-tower" />
  </button>
{/if}

<style>
  .ok {
    color: var(--success);
  }
  .warn {
    color: var(--warning);
  }
  .danger {
    color: var(--danger);
  }
</style>
