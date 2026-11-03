<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte'
  import ConfirmDialog from '@uncli/chat/components/ConfirmDialog.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import ImportDialog from '../components/ImportDialog.svelte'
  import NewSessionDialog from '../components/NewSessionDialog.svelte'
  import Toasts from '@uncli/chat/components/Toasts.svelte'
  import type { Backend } from '@uncli/chat/lib/api'
  import { LOCAL, parseRef, qualify } from '@uncli/chat/lib/api'
  import { take, watch } from '@uncli/chat/lib/backguard'
  import { provideApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { AppStore } from '../stores/app.svelte'
  import ConnectionBanner from './ConnectionBanner.svelte'
  import MobileHome from './MobileHome.svelte'
  import MobileSession from '@uncli/chat/mobile/MobileSession.svelte'
  import MobileSettings from './MobileSettings.svelte'
  import PushSection from './PushSection.svelte'
  import { loadNickname, saveNickname, sessionFromHash } from './pairing'
  import { fetchVersion, mismatched, type ServerVersion } from './version'
  import { wide } from '@uncli/chat/mobile/wide.svelte'

  // The paired phone's app around the window's own store and components:
  // the session list, or one session, with Back to the list (the phone's
  // back gesture too, through history), Settings, and the connection's
  // state over both. A link to #s=<session> (a notification) opens it.

  interface Props {
    backend: Backend
    server: string
    deviceName: string
    onforget: () => void
  }

  let { backend, server, deviceName, onforget }: Props = $props()
  const app = new AppStore(untrack(() => backend), { openLast: false })
  provideApp(app)
  // The side panel covers the page on a phone: it starts closed.
  app.outline.open = false
  let failed = $state('')
  let settingsOpen = $state(false)
  // The server as shown: its nickname when it has one, else its address.
  let nickname = $state(loadNickname())
  const label = $derived(nickname || server)
  function rename(n: string) {
    saveNickname(n)
    nickname = n.trim()
  }
  let version = $state<ServerVersion | null>(null)
  // The build this page came with: once the server runs another (it was
  // updated, decision 0025), the banner offers Reload.
  let loadedWith: ServerVersion | null = null
  let updated = $state(false)
  async function readVersion() {
    const v = await fetchVersion(fetch)
    if (!v) return
    version = v
    if (!loadedWith) loadedWith = v
    else if (v.app !== loadedWith.app || v.commit !== loadedWith.commit) updated = true
  }
  // Each reconnect may follow a restart of the server, new build and all.
  $effect(() => {
    if (app.connection?.state === 'online') untrack(() => void readVersion())
  })
  // The session shown; the store's currentId may lag behind a Back.
  let open = $state(false)

  // show opens a session: a key from the list, or the server's own id from
  // a notification's link. One the phone doesn't know yet (made while it
  // slept) is looked for again after fetching the list afresh.
  async function show(ref: string) {
    const id = parseRef(ref).server === LOCAL ? qualify(server, ref) : ref
    if (!app.sessions.some(s => s.id === id)) {
      await app.resync()
      if (!app.sessions.some(s => s.id === id)) return
    }
    void app.select(id)
    if (!open) history.pushState({ session: id }, '', `#s=${encodeURIComponent(id)}`)
    open = true
  }

  function back() {
    if (take()) return
    if (history.state?.session) history.back()
    else close()
  }

  function close() {
    open = false
    app.currentId = null
    void showing()
    if (location.hash) history.replaceState(null, '', location.pathname)
  }

  // showing tells the server which session the phone shows, if any: a turn
  // ending there is read, not Unread. None while the app is in the
  // background; again when it's back, and after a reconnect (the server
  // forgets when the phone's connection drops).
  async function showing() {
    const id = open && document.visibilityState === 'visible' ? app.currentId ?? '' : ''
    try {
      await app.backend.focus(id)
    } catch {
      // Offline: the reconnect says it again.
    }
  }
  const onvisible = () => void showing()
  $effect(() => {
    if (app.connection?.state === 'online') untrack(() => void showing())
  })

  // Tapping a session in the list selects it: show it. On a tablet the
  // list stays beside the session, so another can be tapped while one is
  // open: the link follows it.
  let shown = ''
  $effect(() => {
    const id = app.currentId
    if (id && id !== shown && !open) void show(id)
    else if (id && id !== shown && open) history.replaceState({ session: id }, '', `#s=${encodeURIComponent(id)}`)
    shown = id ?? ''
  })

  // While an edit is open (a rename), Back cancels it and the page stays
  // (lib/backguard.ts): a history entry of its own gives the back gesture
  // something to undo, and goes when the edit ends some other way.
  // history.back() lands later: an edit starting before it has must wait
  // for it (wantEntry), or the late pop would take the new edit's entry.
  let editEntry = false
  let wantEntry = false
  let skipPops = 0
  function pushEditEntry() {
    history.pushState({ ...history.state, edit: true }, '', location.href)
    editEntry = true
  }
  const stopWatching = watch(held => {
    if (held) {
      if (skipPops > 0) wantEntry = true
      else if (!editEntry) pushEditEntry()
      return
    }
    wantEntry = false
    if (editEntry) {
      editEntry = false
      skipPops++
      history.back()
    }
  })

  const onpop = () => {
    if (skipPops > 0) {
      skipPops--
      if (skipPops === 0 && wantEntry) {
        wantEntry = false
        pushEditEntry()
      }
      return
    }
    if (editEntry) {
      // The back gesture took the edit's entry: cancel the edit, stay here.
      editEntry = false
      take()
      return
    }
    const id = sessionFromHash(location.hash)
    if (id) void show(id)
    else close()
  }

  // A session a notification's link asked for, read as the page opens and
  // shown once the list is in (by then the hash may be one a tap set).
  let linked = sessionFromHash(location.hash)
  function opened() {
    const id = linked
    linked = ''
    if (id) {
      history.replaceState(null, '', location.pathname)
      void show(id)
    }
  }
  const failure = (e: unknown) => (e instanceof Error ? e.message : String(e))

  // The list couldn't be fetched (the PC's network down, say): Try again
  // fetches it again, as does the connection coming back by itself.
  let loading = false
  async function retry() {
    if (loading) return
    loading = true
    failed = ''
    try {
      await app.load()
      opened()
    } catch (e) {
      failed = failure(e)
    } finally {
      loading = false
    }
  }
  let wasOnline = false
  $effect(() => {
    const on = app.connection?.state === 'online'
    if (on && !wasOnline) untrack(() => { if (failed) void retry() })
    wasOnline = on
  })

  onMount(() => {
    loading = true
    app
      .init()
      .then(opened)
      .catch(e => (failed = failure(e)))
      .finally(() => (loading = false))
    void readVersion()
    addEventListener('popstate', onpop)
    document.addEventListener('visibilitychange', onvisible)
  })
  onDestroy(() => {
    removeEventListener('popstate', onpop)
    stopWatching()
    document.removeEventListener('visibilitychange', onvisible)
    app.destroy()
  })
</script>

<div class="shell mobile">
  <ConnectionBanner server={label} mismatch={mismatched(version)} {updated} />
  {#if failed}
    <main class="center">
      <div class="failed">
        <p class="fatal" role="alert"><Icon name="triangle-alert" />{failed}</p>
        <button class="btn primary retry" onclick={retry}><Icon name="rotate-ccw" size={16} />{t('mobile.tryAgain')}</button>
      </div>
    </main>
  {:else if !app.ready}
    <main class="center" aria-busy="true"><Icon name="loader" spin size={20} label={t('app.loading')} /></main>
  {:else if wide.on}
    <!-- A tablet: the list beside the open session (or a word to pick one).
         Below 1200 px the side panel takes the list's place while it's
         open, so the answer keeps a readable width. -->
    <div class="split" class:panel-open={open && app.outline.open}>
      <div class="home list"><MobileHome server={label} onsettings={() => (settingsOpen = true)} /></div>
      <div class="pane">
        {#if open && app.current}
          <MobileSession session={app.current} onback={back} />
        {:else}
          <p class="pick">{t('mobile.pickSession')}</p>
        {/if}
      </div>
    </div>
  {:else if open && app.current}
    <MobileSession session={app.current} onback={back} />
  {:else}
    <div class="home"><MobileHome server={label} onsettings={() => (settingsOpen = true)} /></div>
  {/if}
</div>
{#if app.newSessionOpen}{#key app.newSessionSeq}<NewSessionDialog />{/key}{/if}
{#if app.importOpen}<ImportDialog />{/if}
{#if settingsOpen}
  <MobileSettings {server} {nickname} onrename={rename} {deviceName} version={version?.app ?? ''} {onforget} onclose={() => (settingsOpen = false)}>
    {#snippet notifications()}<PushSection />{/snippet}
  </MobileSettings>
{/if}
<ConfirmDialog />
<Toasts />

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .home {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
  .split {
    flex: 1;
    min-height: 0;
    display: flex;
  }
  .list {
    flex: none;
    width: 340px;
    border-right: 1px solid var(--border);
  }
  .pane {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .pane > :global(.session) {
    flex: 1;
  }
  @media (max-width: 1199px) {
    .split.panel-open .list {
      display: none;
    }
  }
  .pick {
    margin: auto;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
  .center {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--text-muted);
  }
  .failed {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
  }
  .fatal {
    display: flex;
    gap: var(--space-2);
    margin: 0;
    color: var(--danger);
  }
  .retry {
    min-height: 44px;
  }
  /* Touch: the session list's buttons are big enough to tap, and its
     actions show without hovering. */
  .mobile :global(.item .row.main) {
    min-height: 52px;
  }
  .mobile :global(.item .actions .btn.icon) {
    width: 40px;
    height: 40px;
  }
</style>
