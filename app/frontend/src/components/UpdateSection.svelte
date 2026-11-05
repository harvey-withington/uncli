<script lang="ts">
  import type { Preferences } from '@uncli/chat/lib/api'
  import { useApp } from '../lib/context'
  import { relativeTime } from '@uncli/chat/lib/format'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import { useShell } from '@uncli/chat/lib/shell.svelte'

  // Settings → Updates (decision 0025): the version this is, whether it
  // looks for newer releases by itself, Check now, and Update now when
  // there's one. A development build says it's updated another way. The
  // phone's Settings has it without prefs: the PC's version, Check now and
  // Update now, but not the automatic box (the PC's preferences are set at
  // the PC); it does have "Carry on answers cut off by a restart", the one
  // the phone may change (on the desktop it's under Tray and startup), since
  // the phone is where a cut-off answer gets noticed.

  interface Props {
    prefs?: Preferences
    savePrefs?: (next: Preferences) => Promise<boolean>
  }

  let { prefs, savePrefs }: Props = $props()
  const app = useApp()
  const shell = useShell()

  async function resume(on: boolean) {
    try {
      app.patchBoot({ preferences: await app.backend.setResumeCutOff(on) })
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
  const u = $derived(app.update)
  const busy = $derived(u?.state === 'checking' || u?.state === 'downloading' || u?.state === 'installing')
  const line = $derived.by(() => {
    if (!u) return ''
    if (u.dev) return t('update.devBuild')
    switch (u.state) {
      case 'checking': return t('update.checking')
      case 'downloading': return t('update.downloading', { version: u.latest?.version ?? '', percent: u.percent ?? 0 })
      case 'installing': return t('update.installing', { version: u.latest?.version ?? '' })
      case 'error': return t('update.failed', { error: u.error ?? '' })
    }
    if (u.available) return t('update.available', { version: u.latest?.version ?? '' })
    if (u.checkedAt) return t('update.upToDate', { when: relativeTime(u.checkedAt) })
    return t('update.notChecked')
  })
</script>

<section id="settings-updates">
  <h3>{t('update.title')}</h3>
  {#if prefs && savePrefs}
    <p class="muted">{u && !u.dev ? t('update.version', { version: u.current }) : t('update.body')}</p>
    <label class="check">
      <input
        type="checkbox"
        checked={(prefs.updateCheck ?? 'auto') === 'auto'}
        disabled={!u || u.dev}
        onchange={e => savePrefs({ ...prefs, updateCheck: e.currentTarget.checked ? 'auto' : 'off' })}
      />{t('update.auto')}
    </label>
  {:else if u && !u.dev}
    <p class="muted">{t('update.versionRemote', { version: u.current })}</p>
  {/if}
  {#if !prefs}
    <label class="check resume">
      <input
        type="checkbox"
        checked={!!app.boot?.preferences.resumeCutOff}
        aria-describedby="phone-resume-hint"
        onchange={e => void resume(e.currentTarget.checked)}
      />{t('settings.resumeCutOff')}
    </label>
    <p class="muted hint" id="phone-resume-hint">{t('settings.resumeCutOffHint')}</p>
  {/if}
  {#if line}
    <p class="line" class:err={u?.state === 'error'} role="status">
      {#if busy}<Icon name="loader" spin size={13} />{/if}{line}
    </p>
  {/if}
  <div class="acts">
    <button class="btn small" onclick={() => app.checkForUpdates()} disabled={!u || u.dev || busy}>{t('update.checkNow')}</button>
    {#if u?.available && !busy}
      <button class="btn primary small" onclick={() => app.installUpdate()}>{t('update.now')}</button>
      {#if u.latest?.url}
        <button class="btn ghost small" onclick={() => app.backend.openURL(u.latest?.url ?? '')}>{t('update.whatsNew')}</button>
      {/if}
    {/if}
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
  .resume {
    margin-top: var(--space-3);
  }
  .hint {
    margin: var(--space-1) 0 0;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-3) 0 0;
    font-size: var(--text-sm);
  }
  .line.err {
    color: var(--danger);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
</style>
