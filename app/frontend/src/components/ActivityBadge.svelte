<script lang="ts">
  import type { ActivityState } from '../lib/api'
  import { t } from '../lib/i18n.svelte'

  interface Props {
    state: ActivityState
    showIdle?: boolean
  }

  let { state, showIdle = false }: Props = $props()

  const busy = $derived(state === 'starting' || state === 'thinking' || state === 'writing' || state === 'running_tools')
  // Colour is never the only signal: the label always says the state.
  const visible = $derived(showIdle || state !== 'idle')
</script>

{#if visible}
  <span class="badge {state}" class:busy>
    <span class="dot" aria-hidden="true"></span>
    <span class="label">{t(`state.${state}`)}</span>
  </span>
{/if}

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-faint);
    flex: none;
    transition: background var(--normal) var(--ease);
  }
  .busy .dot {
    background: var(--accent);
    animation: pulse 1.4s var(--ease) infinite;
  }
  .busy .label {
    color: var(--accent);
  }
  .unread .dot {
    background: var(--accent);
  }
  .unread .label {
    color: var(--text);
    font-weight: 600;
  }
  .error .dot,
  .exited .dot,
  .needs_approval .dot {
    background: var(--danger);
  }
  .error .label,
  .exited .label,
  .needs_approval .label {
    color: var(--danger);
  }
  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
      transform: scale(1);
    }
    50% {
      opacity: 0.45;
      transform: scale(0.8);
    }
  }
</style>
