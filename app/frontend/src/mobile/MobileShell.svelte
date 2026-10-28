<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte'
  import ConfirmDialog from '../components/ConfirmDialog.svelte'
  import Icon from '../components/Icon.svelte'
  import ImportDialog from '../components/ImportDialog.svelte'
  import NewSessionDialog from '../components/NewSessionDialog.svelte'
  import Toasts from '../components/Toasts.svelte'
  import type { Backend } from '../lib/api'
  import { LOCAL, parseRef, qualify } from '../lib/api/identity'
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

  // Tapping a session in the list selects it: show it.
  let shown = ''
  $effect(() => {
    const id = app.currentId
    if (id && id !== shown && !open) void show(id)
    shown = id ?? ''
  })

  const onpop = () => {
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
