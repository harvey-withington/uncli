<script lang="ts">
  import { useApp } from '../lib/context'
  import { bytes } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // First run: download UNCLI's own copy of the Claude CLI, then sign in.
  // No terminal at any point.
  const app = useApp()
  let installing = $state(false)
  // Sign-in: the CLI prints a link (opened in the browser for you) and
  // waits for the code the page shows after signing in.
  let signin = $state<'idle' | 'starting' | 'code' | 'verifying'>('idle')
  let url = $state('')
  let code = $state('')
  let error = $state('')

  const installed = $derived(!!app.cli?.installed)
  const pct = $derived(app.progress && app.progress.total > 0 ? Math.round((app.progress.done / app.progress.total) * 100) : 0)
  const clean = (e: unknown) => String(e).replace(/^Error:\s*/, '')

  async function install() {
    installing = true
    error = ''
    try {
      app.cli = await app.backend.installCLI()
    } catch (e) {
      error = clean(e)
    } finally {
      installing = false
    }
  }

  async function signIn() {
    signin = 'starting'
    error = ''
    try {
      const start = await app.backend.signIn()
      if (start.signedIn) {
        app.cli = start.status
        signin = 'idle'
        return
      }
      url = start.url ?? ''
      signin = 'code'
    } catch (e) {
      error = clean(e)
      signin = 'idle'
    }
  }

  async function submit() {
    if (!code.trim()) return
    signin = 'verifying'
    error = ''
    try {
      app.cli = await app.backend.submitLoginCode(code)
    } catch (e) {
      error = clean(e)
      // The CLI ends the attempt after one code; trying again needs a new link.
      signin = 'idle'
      code = ''
    }
  }

  async function cancel() {
    await app.backend.cancelSignIn().catch(() => {})
    signin = 'idle'
    code = ''
  }

  async function recheck() {
    error = ''
    try {
      app.cli = await app.backend.cliStatus(true)
    } catch (e) {
      error = clean(e)
    }
  }
</script>

<main class="setup">
  <div class="card">
    <span class="wordmark">UNCLI</span>
    <h1>{t('setup.title')}</h1>
    <p class="lead">{t('setup.lead')}</p>

    <ol class="steps">
      <li class:done={installed} class:current={!installed}>
        <span class="num">{#if installed}<Icon name="check" size={14} />{:else}1{/if}</span>
        <div class="step">
          <h2>{t('setup.download.title')}</h2>
          <p>{t('setup.download.body', { version: app.cli?.version ?? '' })}</p>
          {#if !installed}
            {#if installing}
              <div class="progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label={t('setup.download.title')}>
                <span style:width="{pct}%"></span>
              </div>
              <p class="small">
                {#if app.progress}{t('setup.download.progress', { done: bytes(app.progress.done), total: bytes(app.progress.total) })}{:else}{t('setup.download.starting')}{/if}
              </p>
            {:else}
              <button class="btn primary" onclick={install}><Icon name="download" size={14} />{t('setup.download.button')}</button>
            {/if}
          {/if}
        </div>
      </li>
      <li class:current={installed && !app.cli?.loggedIn} class:pending={!installed}>
        <span class="num">2</span>
        <div class="step">
          <h2>{t('setup.signin.title')}</h2>
          <p>{t('setup.signin.body')}</p>
          {#if installed}
            {#if signin === 'code' || signin === 'verifying'}
              <p class="opened">
                {t('setup.signin.opened')}
                <button class="link" onclick={() => app.backend.openURL(url)}>{t('setup.signin.openAgain')}</button>
                ·
                <button class="link" onclick={() => app.backend.copyText(url)}>{t('setup.signin.copyLink')}</button>
              </p>
              <form class="row" onsubmit={e => { e.preventDefault(); submit() }}>
                <!-- svelte-ignore a11y_autofocus -->
                <input
                  class="input code"
                  bind:value={code}
                  placeholder={t('setup.signin.codePlaceholder')}
                  aria-label={t('setup.signin.codeLabel')}
                  autofocus
                  disabled={signin === 'verifying'}
                  spellcheck="false"
                  autocomplete="off"
                />
                <button class="btn primary" type="submit" disabled={!code.trim() || signin === 'verifying'}>
                  {#if signin === 'verifying'}<Icon name="loader" spin size={14} />{/if}{t('setup.signin.finish')}
                </button>
                <button class="btn ghost" type="button" onclick={cancel} disabled={signin === 'verifying'}>{t('common.cancel')}</button>
              </form>
            {:else}
              <div class="row">
                <button class="btn primary" onclick={signIn} disabled={signin === 'starting'}>
                  {#if signin === 'starting'}<Icon name="loader" spin size={14} />{t('setup.signin.starting')}{:else}<Icon name="log-in" size={14} />{t('setup.signin.button')}{/if}
                </button>
                <button class="btn ghost" onclick={recheck}>{t('setup.signin.recheck')}</button>
              </div>
            {/if}
          {/if}
        </div>
      </li>
    </ol>

    {#if error || app.cli?.error}
      <p class="error" role="alert"><Icon name="triangle-alert" size={14} />{error || app.cli?.error}</p>
    {/if}
  </div>
</main>

<style>
  .setup {
    display: grid;
    place-items: center;
    height: 100%;
    padding: var(--space-6);
    background: var(--bg);
  }
  .card {
    width: 540px;
    max-width: 100%;
    padding: var(--space-6) var(--space-6) var(--space-5);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
  }
  .wordmark {
    font-weight: 750;
    letter-spacing: 0.06em;
    font-size: 13px;
    color: var(--accent);
  }
  h1 {
    margin: var(--space-2) 0 var(--space-2);
    font-size: 24px;
    letter-spacing: -0.02em;
  }
  .lead {
    margin: 0 0 var(--space-5);
    color: var(--text-muted);
  }
  .steps {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  li {
    display: flex;
    gap: var(--space-3);
  }
  li.pending {
    opacity: 0.5;
  }
  .num {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    flex: none;
    border: 1px solid var(--border-strong);
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-muted);
  }
  .current .num {
    border-color: var(--accent);
    color: var(--accent);
  }
  .done .num {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-text);
  }
  .step {
    flex: 1;
    min-width: 0;
  }
  h2 {
    margin: 2px 0 2px;
    font-size: var(--text-md);
  }
  .step p {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .step p.small {
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }
  .row {
    display: flex;
    gap: var(--space-2);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font-size: inherit;
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .code {
    flex: 1;
    min-width: 0;
    font-family: var(--mono);
  }
  .progress {
    height: 6px;
    margin-bottom: var(--space-2);
    border-radius: 999px;
    background: var(--surface-3);
    overflow: hidden;
  }
  .progress span {
    display: block;
    height: 100%;
    background: var(--accent);
    transition: width var(--normal) var(--ease);
  }
  .error {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin: var(--space-5) 0 0;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--danger-soft);
    color: var(--danger);
    font-size: var(--text-sm);
  }
</style>
