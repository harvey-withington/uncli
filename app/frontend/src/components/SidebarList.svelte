<script lang="ts">
  // A list of sessions in the sidebar. Sortable, its sessions are dragged
  // (or moved with Alt+arrows) among themselves: one server's, since each
  // keeps its own order (decision 0031).
  import type { SessionView } from '@uncli/chat/lib/api'
  import { dragSort } from '@uncli/chat/lib/actions'
  import SessionItem from '@uncli/chat/components/SessionItem.svelte'
  import { useApp } from '../lib/context'

  interface Props {
    sessions: SessionView[]
    sortable?: boolean
    label?: string
    dim?: boolean // archived
  }

  let { sessions, sortable = true, label, dim = false }: Props = $props()
  const app = useApp()
</script>

{#snippet items()}
  {#each sessions as session (session.id)}
    <SessionItem {session} active={session.id === app.currentId} />
  {/each}
{/snippet}

{#if sortable}
  <ul class:dim aria-label={label} use:dragSort={{ item: 'li[data-id]', onmove: (id, to) => app.moveSession(id, to) }}>{@render items()}</ul>
{:else}
  <ul class:dim aria-label={label}>{@render items()}</ul>
{/if}

<style>
  ul {
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .dim {
    opacity: 0.75;
  }
</style>
