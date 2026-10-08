<script lang="ts">
  import { tick } from 'svelte'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Modal from './Modal.svelte'
  import AppearanceSection from './AppearanceSection.svelte'
  import ContainersSection from './ContainersSection.svelte'
  import DecisionSection from './DecisionSection.svelte'
  import EditorSection from './EditorSection.svelte'
  import NotifySection from './NotifySection.svelte'
  import ProviderSection from './ProviderSection.svelte'
  import QuickSection from './QuickSection.svelte'
  import SafeSection from './SafeSection.svelte'
  import type { Preferences } from '../lib/api'
  import { loadSettingsTab, nextTab, saveSettingsTab, tabOf, tabsFor, type SettingsTab } from '../lib/settings'

  // Settings, in tabs down the left (lib/settings.ts): General (appearance,
  // notifications, editor, quick tasks), Approvals (safe and unsafe, the
  // decision model), AI Providers (each CLI's version, account and
  // container sign-in) and, on Windows, Containers. It opens on the last
  // tab, or on a section's tab when opened at one (the header's shield,
  // palette commands).
  const app = useApp()
  const tabs = $derived(tabsFor(app.boot?.platform))
  let tab = $state<SettingsTab>(loadSettingsTab(tabsFor(app.boot?.platform)))
  let panel: HTMLElement | undefined = $state()

  function choose(next: SettingsTab) {
    tab = next
    saveSettingsTab(next)
    panel?.scrollTo?.({ top: 0 })
  }

  // Opened at a section: its tab, scrolled to it.
  $effect(() => {
    const at = app.settingsAt
    if (!at) return
    app.settingsAt = ''
    const to = tabOf(at)
    if (to && tabs.includes(to)) choose(to)
    void tick().then(() => document.getElementById(`settings-${at}`)?.scrollIntoView?.({ block: 'start' }))
  })

  // ↑ / ↓ / Home / End move between the tabs (one tab stop).
  function onkeydown(e: KeyboardEvent) {
    const next = nextTab(tabs, tab, e.key)
    if (!next) return
    e.preventDefault()
    choose(next)
    ;(e.currentTarget as HTMLElement).querySelector<HTMLElement>(`[data-tab="${next}"]`)?.focus()
  }

  const prefs = $derived(app.boot?.preferences)

  async function savePrefs(next: Preferences): Promise<boolean> {
    try {
      const saved = await app.backend.setPreferences($state.snapshot(next)) // plain data, not reactive proxies
      if (app.boot) app.boot.preferences = saved
      return true
    } catch (e) {
      showToast(String(e), 'error')
      return false
    }
  }
</script>

<Modal title={t('settings.title')} width={820} onclose={() => (app.settingsOpen = false)}>
  <div class="settings">
    <div class="tabs" role="tablist" aria-orientation="vertical" aria-label={t('settings.tabs')} tabindex="-1" {onkeydown}>
      {#each tabs as k (k)}
        <button
          role="tab"
          data-tab={k}
          id="settings-tab-{k}"
          aria-selected={tab === k}
          aria-controls="settings-panel"
          tabindex={tab === k ? 0 : -1}
          class:on={tab === k}
          onclick={() => choose(k)}
          aria-describedby={app.attention[k]?.length ? `settings-attention-${k}` : undefined}
          title={app.attention[k]?.length ? (app.attention[k] ?? []).map(x => t(x)).join('\n') : undefined}
        >{t(`settings.tab.${k}`)}{#if app.attention[k]?.length}<span class="dot" aria-hidden="true"></span>{/if}</button>
      {/each}
    </div>
    <!-- What a tab's dot means, read out with the tab (aria-describedby). -->
    {#each tabs as k (k)}
      {#if app.attention[k]?.length}
        <span id="settings-attention-{k}" class="visually-hidden">{[t('attention.label'), ...(app.attention[k] ?? []).map(x => t(x))].join('. ')}</span>
      {/if}
    {/each}
    <div class="panel" id="settings-panel" role="tabpanel" aria-labelledby="settings-tab-{tab}" tabindex="-1" bind:this={panel}>
      {#if tab === 'general'}
        <AppearanceSection />
        {#if prefs}
          <NotifySection {prefs} {savePrefs} />
          <EditorSection {prefs} {savePrefs} />
          <QuickSection {prefs} {savePrefs} />
        {/if}
      {:else if tab === 'approvals'}
        <SafeSection {savePrefs} />
        {#if prefs}<DecisionSection {prefs} {savePrefs} />{/if}
      {:else if tab === 'providers'}
        {#each app.boot?.providers ?? [] as p (p.id)}
          <ProviderSection provider={p} />
        {/each}
      {:else if tab === 'containers'}
        <ContainersSection />
      {/if}
    </div>
  </div>
</Modal>

<style>
  .settings {
    display: grid;
    grid-template-columns: 168px 1fr;
    gap: var(--space-5);
    /* One size for every tab, so switching doesn't make the dialog jump. */
    height: min(620px, calc(100vh - 160px));
  }
  .tabs {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-top: var(--space-3);
  }
  .tabs button {
    text-align: left;
    padding: var(--space-2) var(--space-3);
    border: none;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-muted);
    font-size: var(--text-sm);
    transition: background var(--fast) var(--ease), color var(--fast) var(--ease);
  }
  .tabs button:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  .tabs button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--warning);
    flex: none;
  }
  .tabs button.on {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 500;
  }
  .panel {
    min-width: 0;
    overflow: auto;
    padding-right: var(--space-2);
    outline: none;
  }
  /* The first section of a tab needs no divider above it. */
  .panel > :global(section:first-child) {
    margin-top: 0;
    padding-top: var(--space-3);
    border-top: 0;
  }
  @media (prefers-reduced-motion: reduce) {
    .tabs button {
      transition: none;
    }
  }
</style>
