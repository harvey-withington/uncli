<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // Signing the first provider's CLI in again, by link and code, where it's
  // needed: a turn its sign-in failed, or Settings. The CLI opens the link
  // in the browser; the page there shows a code to paste back. Signed in,
  // the backend restarts the provider's idle sessions at their next message
  // (Manager.Reconnect), so they use the new sign-in.
  interface Props {
    label: string
    primary?: boolean
    onsignedin?: () => void
    active?: boolean // bindable: a code is being asked for
  }

  let { label, primary = false, onsignedin, active = $bindable(false) }: Props = $props()
  const app = useApp()
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
      const s = await app.backend.signIn()
      if (s.signedIn) {
        app.cli = s.status
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
      app.cli = await app.backend.submitLoginCode(code)
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
    await app.backend.cancelSignIn().catch(() => {})
    step = 'idle'
    code = ''
  }
</script>

<div class="code-signin">
  {#if step === 'code' || step === 'verifying'}
    <p class="opened">
      {t('setup.signin.opened')}
      <button class="link" onclick={() => app.backend.openURL(url)}>{t('setup.signin.openAgain')}</button>
      ·
      <button class="link" onclick={() => app.backend.copyText(url)}>{t('setup.signin.copyLink')}</button>
    </p>
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
  .opened {
    margin: 0;
    font-size: var(--text-sm);
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
