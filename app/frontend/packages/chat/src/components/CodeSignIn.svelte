<script lang="ts">
  import type { Backend } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { atServer } from '../lib/remoteonly'
  import Icon from './Icon.svelte'

  // Signing the first provider's CLI in again, by link and code, where it's
  // needed: a turn its sign-in failed, or Settings. The page at the link
  // shows a code to paste back. Signed in, the backend restarts the
  // provider's idle sessions at their next message (Manager.Reconnect), so
  // they use the new sign-in. Away from the PC (the phone, or a paired
  // server's CLI from the desktop; decision 0034) it signs in that PC's CLI,
  // and offers the link to open here rather than saying it was opened.
  interface Props {
    label: string
    primary?: boolean
    onsignedin?: () => void
    active?: boolean // bindable: a code is being asked for
    backend?: Backend // the CLI's PC: the window's by default, a paired server's for its sessions
  }

  const chat = useChat()
  let { label, primary = false, onsignedin, active = $bindable(false), backend = chat.backend }: Props = $props()
  // Another PC's CLI: its status isn't the one this window shows.
  const elsewhere = $derived(backend !== chat.backend)
  // The window opens the link itself; anywhere else, it's offered to open.
  const opened = $derived(atServer(chat.backend) && !elsewhere)
  function openPage() {
    if (!chat.openLink?.(url)) void chat.backend.openURL(url)
  }
  let step = $state<'idle' | 'starting' | 'code' | 'verifying'>('idle')
  $effect(() => {
    active = step === 'code' || step === 'verifying'
  })
  let url = $state('')
  let code = $state('')
  let error = $state('')
  const clean = (e: unknown) => String(e).replace(/^Error:\s*/, '')

  async function start() {
    step = 'starting'
    error = ''
    try {
      const s = await backend.signIn()
      if (s.signedIn) {
        if (!elsewhere) chat.cli = s.status
        step = 'idle'
        onsignedin?.()
        return
      }
      url = s.url ?? ''
      step = 'code'
    } catch (e) {
      error = clean(e)
      step = 'idle'
    }
  }

  async function finish() {
    step = 'verifying'
    error = ''
    try {
      const st = await backend.submitLoginCode(code)
      if (!elsewhere) chat.cli = st
      step = 'idle'
      code = ''
      onsignedin?.()
    } catch (e) {
      error = clean(e)
      step = 'idle' // the CLI ends the attempt after one code: a new one needs a new link
      code = ''
    }
  }

  async function cancel() {
    await backend.cancelSignIn().catch(() => {})
    step = 'idle'
    code = ''
  }
</script>

<!-- Asking for the code, it takes the row, so the code fits. -->
<div class="code-signin" class:wide={step === 'code' || step === 'verifying'}>
  {#if step === 'code' || step === 'verifying'}
    {#if opened}
      <p class="opened">
        {t('setup.signin.opened')}
        <button class="link" onclick={openPage}>{t('setup.signin.openAgain')}</button>
        ·
        <button class="link" onclick={() => chat.backend.copyText(url)}>{t('setup.signin.copyLink')}</button>
      </p>
    {:else}
      <p class="opened">
        <button class="btn small" onclick={openPage}><Icon name="external-link" size={13} />{t('setup.signin.openPage')}</button>
        <button class="link" onclick={() => chat.backend.copyText(url)}>{t('setup.signin.copyLink')}</button>
      </p>
      <p class="hint">{t('setup.signin.pasteHint')}</p>
    {/if}
    <form class="row" onsubmit={e => { e.preventDefault(); void finish() }}>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input"
        bind:value={code}
        placeholder={t('setup.signin.codePlaceholder')}
        aria-label={t('setup.signin.codeLabel')}
        autofocus
        disabled={step === 'verifying'}
        spellcheck="false"
        autocomplete="off"
      />
      <button class="btn small primary" type="submit" disabled={!code.trim() || step === 'verifying'}>
        {#if step === 'verifying'}<Icon name="loader" spin size={13} />{/if}{t('setup.signin.finish')}
      </button>
      <button class="btn small ghost" type="button" onclick={cancel} disabled={step === 'verifying'}>{t('common.cancel')}</button>
    </form>
  {:else}
    <button class="btn small" class:primary onclick={start} disabled={step === 'starting'}>
      {#if step === 'starting'}<Icon name="loader" spin size={13} />{:else}<Icon name="log-in" size={13} />{/if}{label}
    </button>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
</div>

<style>
  .code-signin {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .code-signin.wide {
    flex: 1 1 100%;
    min-width: 0;
  }
  .opened {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .row {
    display: flex;
    gap: var(--space-2);
    width: 100%;
    max-width: 520px;
  }
  .row .input {
    flex: 1;
    min-width: 0;
    font-family: var(--mono);
    font-size: var(--text-xs);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    cursor: pointer;
  }
  .link:hover {
    text-decoration: underline;
  }
  .error {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--danger);
  }
</style>
