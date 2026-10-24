<script lang="ts">
  import type { Preferences } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'

  // Settings → Tray and startup: closing the window keeps UNCLI running in
  // the tray (Open and Quit on its icon), and it can start with Windows,
  // minimised. Elsewhere the switch shows, disabled, with why.

  interface Props {
    prefs: Preferences
    savePrefs: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  const supported = $derived(app.boot?.platform === 'windows')
</script>

<section id="settings-startup">
  <h3>{t('settings.startupTitle')}</h3>
  <p class="muted">{t('settings.startupBody')}</p>
  <label class="check">
    <input
      type="checkbox"
      checked={!!prefs.startWithWindows}
      disabled={!supported}
      onchange={e => savePrefs({ ...prefs, startWithWindows: e.currentTarget.checked })}
    />{t('settings.startWithWindows')}
  </label>
  {#if !supported}<p class="muted note">{t('settings.startupWindowsOnly')}</p>{/if}
</section>

<style>
  section {
    margin-top: var(--space-5);
    padding-top: var(--space-4);
    border-top: 1px solid var(--border);
  }
  h3 {
    margin: 0 0 var(--space-1);
    font-size: var(--text-md);
  }
  .muted {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }
  .note {
    margin: var(--space-2) 0 0;
  }
</style>
