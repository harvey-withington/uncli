<script lang="ts">
  import { onMount } from 'svelte'
  import { provideChat } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import type { ChatStore } from '../stores/chat.svelte'
  import ConfirmDialog from './ConfirmDialog.svelte'
  import Icon from './Icon.svelte'
  import SessionPane from './SessionPane.svelte'
  import Toasts from './Toasts.svelte'

  // One chat (decision 0026): a session's view on a ChatStore, with its own
  // toasts and confirm dialog. The <uncli-chat> element mounts it in its
  // shadow root; it shows the session given, once the store has it.

  interface Props {
    store: ChatStore
    session: string
  }

  let { store, session }: Props = $props()
  // svelte-ignore state_referenced_locally
  provideChat(store)
  let failed = $state('')

  const view = $derived(store.sessions.find(s => s.id === session) ?? null)

  onMount(() => {
    store.init().catch(e => (failed = e instanceof Error ? e.message : String(e)))
  })

  // The session shown follows the attribute, once the first state is in.
  $effect(() => {
    if (store.ready && session && store.currentId !== session) void store.select(session)
  })
</script>

<div class="chat">
  {#if failed}
    <p class="note" role="alert"><Icon name="triangle-alert" />{failed}</p>
  {:else if !store.ready}
    <p class="note" aria-busy="true"><Icon name="loader" spin size={18} label={t('app.loading')} /></p>
  {:else if view}
    <SessionPane session={view} />
  {:else}
    <p class="note">{t('chat.noSession')}</p>
  {/if}
  <Toasts />
  <ConfirmDialog />
</div>

<style>
  .chat {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .chat > :global(.pane) {
    flex: 1;
    min-height: 0;
  }
  .note {
    display: flex;
    gap: var(--space-2);
    margin: auto;
    padding: var(--space-4);
    color: var(--text-muted);
  }
</style>
