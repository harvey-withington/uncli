<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // A sign-in by link and code (containers: decision 0011): the CLI offers
  // a link, the user approves it on any device, then pastes back the code
  // the page shows. Signed in, it says so with Sign out.
  interface Props {
    title: string
    note: string // what this sign-in is for, before signing in
    signedInNote: string
    buttonLabel: string
    warning?: string // what it lets the container do, shown before signing in
    signedIn: boolean
    canStart: boolean // a container is built (sign-in runs inside one)
    start: () => Promise<string>
    finish: (code: string) => Promise<void>
    cancel: () => Promise<void>
    signOut: () => Promise<void>
  }

  let { title, note, signedInNote, buttonLabel, warning, signedIn, canStart, start, finish, cancel, signOut }: Props = $props()
  const app = useApp()
  let link = $state('')
  let code = $state('')
  let busy = $state(false)

  async function run(f: () => Promise<unknown>) {
    try {
      await f()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function begin() {
    busy = true
    await run(async () => {
      link = await start()
      code = ''
    })
    busy = false
  }

  async function done() {
    busy = true
    await run(async () => {
      await finish(code.trim())
      link = ''
      code = ''
      showToast(t('containers.signedIn'))
    })
    busy = false
  }

  function stop() {
    link = ''
    code = ''
    void cancel()
  }
</script>

<div class="signin">
  <h5>{title}</h5>
  {#if signedIn}
    <p class="muted"><Icon name="check" size={14} />{signedInNote}</p>
    <button class="btn small ghost" onclick={() => run(signOut)}>{t('containers.signOut')}</button>
  {:else if link}
    <p class="muted">{t('containers.approve')}</p>
    <div class="row">
      <button class="btn" onclick={() => run(() => app.backend.openURL(link))}><Icon name="external-link" size={14} />{t('containers.openLink')}</button>
      <button class="btn ghost" onclick={() => run(() => app.backend.copyText(link))}><Icon name="copy" size={14} />{t('containers.copyLink')}</button>
    </div>
    <div class="row">
      <input class="input" bind:value={code} placeholder={t('containers.codePlaceholder')} aria-label={t('containers.codePlaceholder')} autocomplete="off" spellcheck="false" />
      <button class="btn primary" onclick={done} disabled={busy || !code.trim()}>
        {#if busy}<Icon name="loader" spin size={14} />{/if}{t('containers.finish')}
      </button>
      <button class="btn ghost" onclick={stop}>{t('containers.cancel')}</button>
    </div>
  {:else}
    <p class="muted">{note}</p>
    {#if warning}<p class="warn" role="note"><Icon name="triangle-alert" size={14} />{warning}</p>{/if}
    {#if !canStart}<p class="muted small">{t('containers.signInNeedsBuild')}</p>{/if}
    <button class="btn" onclick={begin} disabled={busy || !canStart}>
      {#if busy}<Icon name="loader" spin size={14} />{:else}<Icon name="log-in" size={14} />{/if}{buttonLabel}
    </button>
  {/if}
</div>

<style>
  .signin + :global(.signin) {
    margin-top: var(--space-4);
  }
  h5 {
    margin: 0 0 var(--space-1);
    font-size: var(--text-sm);
    font-weight: 600;
  }
  .muted {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    margin: 0 0 var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .small {
    font-size: var(--text-xs);
  }
  .warn {
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
    margin: 0 0 var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--warning-soft);
    color: var(--warning);
    font-size: var(--text-sm);
  }
  .row {
    display: flex;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
  }
  .row .input {
    flex: 1;
    font-family: var(--mono);
  }
</style>
