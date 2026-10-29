<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte'
  import ConfirmDialog from '../components/ConfirmDialog.svelte'
  import Icon from '../components/Icon.svelte'
  import ImportDialog from '../components/ImportDialog.svelte'
  import NewSessionDialog from '../components/NewSessionDialog.svelte'
  import Toasts from '../components/Toasts.svelte'
  import type { Backend } from '../lib/api'
  import { LOCAL, parseRef, qualify } from '../lib/api/identity'
  import { take, watch } from '../lib/backguard'
  import { provideApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { AppStore } from '../stores/app.svelte'
  import ConnectionBanner from './ConnectionBanner.svelte'
  import MobileHome from './MobileHome.svelte'
  import MobileSession from './MobileSession.svelte'
  import MobileSettings from './MobileSettings.svelte'
  import PushSection from './PushSection.svelte'
  import { sessionFromHash } from './pairing'
  import { fetchVersion, mismatched, type ServerVersion } from './version'
  import { wide } from './wide.svelte'

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

  onMount(() => {
    app
      .init()
      .then(() => {
        const id = sessionFromHash(location.hash)
        if (id) {
          history.replaceState(null, '', location.pathname)
          void show(id)
        }
      })
      .catch(e => (failed = String(e)))
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
  <ConnectionBanner {server} mismatch={mismatched(version)} {updated} />
  {#if failed}
    <main class="center"><p class="fatal" role="alert"><Icon name="triangle-alert" />{failed}</p></main>
  {:else if !app.ready}
    <main class="center" aria-busy="true"><Icon name="loader" spin size={20} label={t('app.loading')} /></main>
  {:else if wide.on}
    <!-- A tablet: the list beside the open session (or a word to pick one).
         Below 1200 px the side panel takes the list's place while it's
         open, so the answer keeps a readable width. -->
    <div class="split" class:panel-open={open && app.outline.open}>
      <div class="home list"><MobileHome {server} onsettings={() => (settingsOpen = true)} /></div>
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
    <div class="home"><MobileHome {server} onsettings={() => (settingsOpen = true)} /></div>
  {/if}
</div>
{#if app.newSessionOpen}{#key app.newSessionSeq}<NewSessionDialog />{/key}{/if}
{#if app.importOpen}<ImportDialog />{/if}
{#if settingsOpen}
  <MobileSettings {server} {deviceName} version={version?.app ?? ''} {onforget} onclose={() => (settingsOpen = false)}>
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
  .fatal {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-4);
    color: var(--danger);
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
