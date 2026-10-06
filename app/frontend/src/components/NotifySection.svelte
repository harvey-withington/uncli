<script lang="ts">
  import type { NotifySetting, Preferences } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // Settings → Notifications: which desktop notifications to show when a
  // session you aren't looking at finishes or needs approval, and a button
  // that shows a sample one.

  interface Props {
    prefs: Preferences
    savePrefs: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  const settings: NotifySetting[] = ['all', 'approvals', 'off']
  const current = $derived(prefs.notifications ?? 'all')
  let testing = $state(false)

  async function test() {
    testing = true
    try {
      await app.backend.testNotification()
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      testing = false
    }
  }
</script>

<section id="settings-notify">
  <h3>{t('settings.notifyTitle')}</h3>
  <p class="muted">{t('settings.notifyBody')}</p>
  <div class="row">
    <select
      class="select"
      aria-label={t('settings.notifyTitle')}
      value={current}
      onchange={e => savePrefs({ ...prefs, notifications: e.currentTarget.value as NotifySetting })}
    >
      {#each settings as s (s)}
        <option value={s}>{t(`settings.notify.${s}`)}</option>
      {/each}
    </select>
    <button class="btn" onclick={test} disabled={testing || current === 'off'}>
      {#if testing}<Icon name="loader" spin size={14} />{:else}<Icon name="bell" size={14} />{/if}{t('settings.notifyTest')}
    </button>
  </div>
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
  .row {
    display: flex;
    gap: var(--space-2);
  }
  .row .select {
    flex: 1;
  }
</style>
