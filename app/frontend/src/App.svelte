<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte'
  import CommandPalette from './components/CommandPalette.svelte'
  import ConfirmDialog from './components/ConfirmDialog.svelte'
  import ImportDialog from './components/ImportDialog.svelte'
  import KeyboardMap from './components/KeyboardMap.svelte'
  import Icon from './components/Icon.svelte'
  import NewSessionDialog from './components/NewSessionDialog.svelte'
  import SessionPane from './components/SessionPane.svelte'
  import SettingsDialog from './components/SettingsDialog.svelte'
  import UsageDashboard from './components/UsageDashboard.svelte'
  import SetupScreen from './components/SetupScreen.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Toasts from './components/Toasts.svelte'
  import type { Backend } from './lib/api'
  import { provideApp } from './lib/context'
  import { listenForDrops } from './lib/drops'
  import { tintStyle } from './lib/tint'
  import { t } from './lib/i18n.svelte'
  import { commands, matches } from './lib/commands'
  import { isTyping } from './lib/nav'
  import { AppStore } from './stores/app.svelte'

  interface Props {
    backend: Backend
  }

  let { backend }: Props = $props()
  // The backend is fixed for the app's lifetime.
  const app = new AppStore(untrack(() => backend))
  provideApp(app)
  // Dev builds expose the store for debugging and end-to-end checks.
  if (import.meta.env.DEV) (window as unknown as { __uncli: AppStore }).__uncli = app
  let failed = $state('')

  onMount(() => {
    app.init().catch(e => (failed = String(e)))
    return listenForDrops(app)
  })
  onDestroy(() => app.destroy())

  // Shortcuts come from the command registry (lib/commands.ts), the same
  // list the palette and the keyboard map show. A key a control already
  // handled (arrows on a radio group or tab strip) is left alone; plain keys
  // never fire while typing (function keys do: they type nothing); while a dialog is open it owns the keyboard,
  // except for the palette's own shortcut.
  function onkeydown(e: KeyboardEvent) {
    if (e.defaultPrevented || e.repeat && !e.key.startsWith('Arrow')) return
    const dialog = app.newSessionOpen || app.settingsOpen || app.usageOpen || app.paletteOpen || app.keysOpen || app.importOpen
    for (const c of commands(app)) {
      for (const k of c.keys ?? []) {
        if (!matches(k, e)) continue
        if (dialog && c.id !== 'palette') return
        if (!k.ctrl && !/^F\d+$/.test(k.key) && isTyping(e.target)) return
        e.preventDefault()
        c.run()
        return
      }
    }
  }

  // A session only counts as read while the window has focus.
  function onfocus() {
    if (app.currentId) app.backend.focus(app.currentId)
  }
  function onblur() {
    app.backend.focus('')
  }
</script>

<svelte:window {onkeydown} {onfocus} {onblur} />

{#if failed}
  <main class="center"><p class="fatal" role="alert"><Icon name="triangle-alert" />{failed}</p></main>
{:else if !app.ready}
  <main class="center" aria-busy="true"><Icon name="loader" spin size={20} label={t('app.loading')} /></main>
{:else if !app.cliReady}
  <SetupScreen />
{:else}
  <div class="shell">
    <Sidebar />
    {#if app.current}
      <SessionPane session={app.current} />
    {:else}
      <main class="welcome">
        <h1>{t('welcome.title')}</h1>
        <p>{t('welcome.body')}</p>
        <div class="cards">
          {#each app.boot?.profiles ?? [] as p (p.id)}
            <button class="card" style={tintStyle(p.hue)} onclick={() => app.openNewSession(p.id)}>
              <span class="cicon"><Icon name={p.icon} size={18} /></span>
              <strong>{p.label}</strong>
              <span>{t(`profile.${p.id}.desc`)}</span>
            </button>
          {/each}
        </div>
      </main>
    {/if}
  </div>
{/if}

{#if app.newSessionOpen}{#key app.newSessionSeq}<NewSessionDialog />{/key}{/if}
{#if app.settingsOpen}<SettingsDialog />{/if}
{#if app.usageOpen}<UsageDashboard />{/if}
{#if app.paletteOpen}<CommandPalette />{/if}
{#if app.keysOpen}<KeyboardMap />{/if}
{#if app.importOpen}<ImportDialog />{/if}
<ConfirmDialog />
<Toasts />

<style>
  .shell {
    display: flex;
    height: 100%;
  }
  .center {
    display: grid;
    place-items: center;
    height: 100%;
    color: var(--text-muted);
  }
  .fatal {
    display: flex;
    gap: var(--space-2);
    color: var(--danger);
    max-width: 560px;
  }
  .welcome {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: var(--space-6);
    text-align: center;
  }
  .welcome h1 {
    margin: 0 0 var(--space-2);
    font-size: 26px;
    letter-spacing: -0.02em;
  }
  .welcome > p {
    margin: 0 0 var(--space-6);
    color: var(--text-muted);
    max-width: 460px;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(3, 200px);
    gap: var(--space-3);
  }
  .card {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-1);
    padding: var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    text-align: left;
    box-shadow: var(--shadow-sm);
    transition: transform var(--fast) var(--ease), border-color var(--fast) var(--ease);
  }
  .card:hover {
    border-color: var(--tint, var(--accent));
    transform: translateY(-1px);
  }
  .card span:last-child {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .cicon {
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    margin-bottom: var(--space-2);
    border-radius: 10px;
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
  }
</style>
