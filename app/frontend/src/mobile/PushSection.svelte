<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import type { NotifySetting, PushState } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import { disablePush, enablePush, pushEnv, pushSupport, type PushEnv } from './push'

  // The phone's notifications (decision 0021): turned on in this browser,
  // then which to get (as the desktop's setting: all, approvals only, off),
  // a test, and turning them off here. When this browser can't, the button
  // stays, disabled, with why.

  interface Props {
    env?: PushEnv // the browser's abilities (tests pass their own)
  }

  let { env = pushEnv() }: Props = $props()
  const app = useApp()
  const support = $derived(pushSupport(env))
  const settings: NotifySetting[] = ['all', 'approvals', 'off']
  let info = $state<PushState | null>(null)
  let busy = $state(false)

  async function load() {
    try {
      info = await app.backend.pushState()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
  onMount(load)

  async function run(f: () => Promise<unknown>) {
    busy = true
    try {
      await f()
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
      await load()
    }
  }

  const turnOn = () => run(async () => {
    if (!(await enablePush(app.backend))) showToast(t('push.refused'), 'info')
  })
</script>

<section id="settings-push">
  <h3>{t('settings.notifyTitle')}</h3>
  <p class="muted">{t('push.body')}</p>
  {#if support !== 'ok' && !info?.subscribed}
    <button class="btn" disabled><Icon name="bell" size={14} />{t('push.turnOn')}</button>
    <p class="note">{t(`push.why.${support}`)}</p>
  {:else if !info?.subscribed}
    <button class="btn primary" onclick={turnOn} disabled={busy || !info}>
      {#if busy}<Icon name="loader" spin size={14} />{:else}<Icon name="bell" size={14} />{/if}{t('push.turnOn')}
    </button>
  {:else}
    <select
      class="select"
      aria-label={t('settings.notifyTitle')}
      value={info.setting}
      disabled={busy}
      onchange={e => run(() => app.backend.setPushNotifications(e.currentTarget.value as NotifySetting))}
    >
      {#each settings as s (s)}
        <option value={s}>{t(`settings.notify.${s}`)}</option>
      {/each}
    </select>
    {#if info.lastFail}<p class="note danger" role="status">{t('push.lastFail', { error: info.lastFail })}</p>{/if}
    <div class="row">
      <button class="btn" onclick={() => run(() => app.backend.testPush())} disabled={busy || info.setting === 'off'}><Icon name="bell" size={14} />{t('settings.notifyTest')}</button>
      <button class="btn ghost" onclick={() => run(() => disablePush(app.backend))} disabled={busy}>{t('push.turnOff')}</button>
    </div>
  {/if}
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
  .btn,
  .select {
    min-height: 44px;
  }
  .select {
    width: 100%;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .note {
    margin: var(--space-2) 0 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .note.danger {
    color: var(--danger);
  }
</style>
