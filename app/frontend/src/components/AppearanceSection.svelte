<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { setTheme, theme, type Theme } from '../lib/theme.svelte'
  import Icon from './Icon.svelte'

  // Settings → General → Appearance: the theme (the sidebar's toggle cycles
  // the same setting) and the theme file, which restyles UNCLI (decision
  // 0010), with where UNCLI looks for it and a reload.
  const app = useApp()
  const themes: Theme[] = ['system', 'light', 'dark']
  let file = $state<{ path: string; found: boolean } | null>(null)
  let reloading = $state(false)

  async function check() {
    try {
      const f = await app.backend.theme()
      file = { path: f.path, found: f.found }
    } catch {
      file = null
    }
  }

  $effect(() => {
    void check()
  })

  async function reload() {
    reloading = true
    await app.loadThemeFile(true)
    await check()
    reloading = false
  }
</script>

<section id="settings-appearance">
  <h3>{t('settings.appearanceTitle')}</h3>
  <div class="grid">
    <label for="theme">{t('settings.theme')}</label>
    <select id="theme" class="select" value={theme.value} onchange={e => setTheme(e.currentTarget.value as Theme)}>
      {#each themes as v (v)}
        <option value={v}>{t(`settings.theme.${v}`)}</option>
      {/each}
    </select>
    <span>{t('settings.themeFile')}</span>
    <div class="file">
      <span class="path" title={file?.path}>
        {#if file?.found}{t('settings.themeFileFound', { path: file.path })}{:else if file}{t('settings.themeFileNone', { path: file.path })}{/if}
      </span>
      <button class="btn small" onclick={reload} disabled={reloading}>
        {#if reloading}<Icon name="loader" spin size={13} />{/if}{t('settings.themeFileReload')}
      </button>
    </div>
  </div>
</section>

<style>
  section {
    margin-top: var(--space-5);
    padding-top: var(--space-4);
    border-top: 1px solid var(--border);
  }
  h3 {
    margin: 0 0 var(--space-3);
    font-size: var(--text-md);
  }
  .grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-3);
    align-items: center;
    font-size: var(--text-sm);
  }
  .file {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
</style>
