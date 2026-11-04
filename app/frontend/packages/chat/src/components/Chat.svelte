<script lang="ts">
  import { onMount } from 'svelte'
  import { provideChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import type { ChatStore } from '../stores/chat.svelte'
  import ConfirmDialog from './ConfirmDialog.svelte'
  import Icon from './Icon.svelte'
  import SessionPane from './SessionPane.svelte'
  import Setup from './Setup.svelte'
  import { canSetUp } from '../lib/remoteonly'
  import Toasts from './Toasts.svelte'
  import MobileSession from '../mobile/MobileSession.svelte'

  // One chat (decision 0026): a session's view on a ChatStore, with its own
  // toasts and confirm dialog. The <uncli-chat> element mounts it in its
  // shadow root; it shows the session given, once the store has it. Its
  // layout follows its own width, not the window's: the desktop's session
  // view when there's room, the phone's compact one below COMPACT_AT, or
  // whichever compact says. On a host's own UNCLI whose CLI isn't installed
  // or signed in yet, it shows the setup steps instead (decision 0027).

  const COMPACT_AT = 720

  interface Props {
    store: ChatStore
    session: string
    compact?: boolean // force a layout; by width when left out
  }

  let { store, session, compact }: Props = $props()
  let width = $state(0)
  const narrow = $derived(compact ?? (width > 0 && width < COMPACT_AT))
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

<div class="chat" bind:clientWidth={width}>
  {#if failed}
    <p class="note" role="alert"><Icon name="triangle-alert" />{failed}</p>
  {:else if !store.ready}
    <p class="note" aria-busy="true"><Icon name="loader" spin size={18} label={t('app.loading')} /></p>
  {:else if store.cli && !store.cliReady && canSetUp(store.backend)}
    <Setup />
  {:else if view && narrow}
    <MobileSession session={view} wide={false} />
  {:else if view}
    <SessionPane session={view} />
  {:else if session}
    <p class="note">{t('chat.noSession')}</p>
  {:else}
    <p class="note">{t('chat.noSessionGiven')}</p>
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
  .chat > :global(.pane),
  .chat > :global(.session) {
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
