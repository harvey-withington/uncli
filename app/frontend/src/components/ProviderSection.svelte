<script lang="ts">
  import type { CLIStatus, Provider } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { namesOf } from '../lib/providers'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import LinkSignIn from './LinkSignIn.svelte'

  // Settings → AI Providers: one of these per CLI sessions can run on. Its
  // version (anything but the pinned one is at the user's own risk), the
  // account it's signed in with, and, for the first provider on Windows,
  // its sign-in for containers (a token of its own, from a link approved on
  // any device and the code the page shows; decision 0011). A provider that
  // isn't the first can be installed here; one whose CLI signs in elsewhere
  // (in its own app or terminal) says so and checks again on request.
  interface Props {
    provider: Provider
  }

  let { provider }: Props = $props()
  const app = useApp()
  const names = $derived(namesOf(provider))
  // The first provider keeps the plain section ids palette commands open.
  const first = $derived(provider.id === app.boot?.providers[0]?.id)
  const sid = (part: string) => (first ? `settings-${part}` : `settings-${part}-${provider.id}`)
  const status = $derived(app.statusOf(provider.id))
  let version = $state('')
  let channels = $state<Record<string, string>>({})
  let busy = $state(false)
  let checking = $state(false)
  const pinned = $derived(status?.pinned ?? '')
  const isPinned = $derived(!version || version === pinned)
  const containers = $derived(app.boot?.platform === 'windows' && first ? app.containers : null)
  const built = $derived(!!containers?.containers.some(c => c.built))
  const downloading = $derived(busy && app.progress?.provider === provider.id ? app.progress : null)

  $effect(() => {
    version = app.statusOf(provider.id)?.version ?? ''
    const ch = provider.id === app.boot?.providers[0]?.id ? app.backend.cliChannels() : app.backend.providerChannels(provider.id)
    ch.then(c => (channels = c)).catch(() => {})
    if (app.boot?.platform === 'windows' && !app.containers) void app.loadContainers()
  })

  function update(st: CLIStatus) {
    if (first) app.cli = st
    else app.providerStatus[provider.id] = st
  }

  async function run(f: () => Promise<unknown>) {
    try {
      await f()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function apply(v: string) {
    busy = true
    await run(async () => {
      const st = first ? await app.backend.setCLIVersion(v) : await app.backend.setProviderVersion(provider.id, v)
      update(st)
      version = st.version
      showToast(t('settings.switched', { ...names, version: st.version }), 'success')
    })
    busy = false
  }

  async function install() {
    busy = true
    await run(async () => update(await app.backend.installProvider(provider.id)))
    busy = false
  }

  async function check() {
    checking = true
    await run(async () => update(await app.backend.providerStatus(provider.id, true)))
    checking = false
  }
</script>

<section id="settings-provider-{provider.id}" class="provider" aria-labelledby="provider-{provider.id}">
  <h3 id="provider-{provider.id}">{provider.name}</h3>

  {#if !first && !status?.installed}
    <div id={sid('cli')} class="part">
      <h4>{t('settings.cliTitle')}</h4>
      <p class="muted">{t('settings.notInstalled', { ...names, pinned })}</p>
      <button class="btn primary" onclick={install} disabled={busy}>
        {#if busy}<Icon name="loader" spin size={14} />{:else}<Icon name="download" size={14} />{/if}{t('settings.install', { ...names, pinned })}
      </button>
      {#if downloading?.total}
        <p class="muted small" aria-live="polite">{t('settings.downloading', { n: Math.round((100 * downloading.done) / downloading.total) })}</p>
      {/if}
    </div>
  {:else}
    <div id={sid('cli')} class="part">
      <h4>{t('settings.cliTitle')}</h4>
      <p class="muted">{t('settings.cliBody', { ...names, pinned })}</p>
      {#if status?.custom}
        <p class="note">{t('settings.custom', { id: provider.id })}</p>
      {/if}
      <div class="row">
        <input class="input" bind:value={version} aria-label={t('settings.version')} placeholder={pinned} />
        <button class="btn" onclick={() => apply(version)} disabled={busy || !version.trim()}>
          {#if busy}<Icon name="loader" spin size={14} />{/if}{t('settings.use')}
        </button>
      </div>
      <div class="channels">
        {#each Object.entries(channels) as [name, v] (name)}
          <button class="btn small ghost" onclick={() => (version = v)}>{t('settings.channel', { name, version: v })}</button>
        {/each}
      </div>
      {#if !isPinned}
        <p class="warn" role="note"><Icon name="triangle-alert" size={14} />{t('settings.risk')}</p>
      {/if}
      <button class="btn" onclick={() => apply('')} disabled={busy || status?.version === pinned}>{t('settings.backToPinned', { pinned })}</button>
    </div>

    <div id={sid('account')} class="part">
      <h4>{t('settings.account')}</h4>
      {#if provider.signIn === 'elsewhere'}
        <p class="muted">
          {#if status?.loggedIn}<Icon name="check" size={14} />{/if}
          {t(status?.loggedIn ? 'settings.signedInElsewhere' : 'settings.signInElsewhere', names)}
        </p>
        {#if status?.error}<p class="note">{status.error}</p>{/if}
        <button class="btn small" onclick={check} disabled={checking}>
          {#if checking}<Icon name="loader" spin size={13} />{/if}{t('settings.checkAgain')}
        </button>
      {:else if status?.email}
        <p class="muted">{t('settings.signedIn', { email: status.email, plan: status.subscription ?? '' })}</p>
      {:else}
        <p class="muted">{t('settings.notSignedIn', names)}</p>
      {/if}
    </div>
  {/if}

  {#if containers}
    <div id="settings-provider-containers" class="part">
      <h4>{t('settings.containerSignIn')}</h4>
      <LinkSignIn
        title={t('containers.tokenTitle')}
        note={t('containers.signInNote')}
        signedInNote={t('containers.signedInNote')}
        buttonLabel={t('containers.signIn')}
        signedIn={containers.signedIn}
        canStart={built}
        start={() => app.backend.startContainerSignIn()}
        finish={code => app.backend.finishContainerSignIn(code)}
        cancel={() => app.backend.cancelContainerSignIn()}
        signOut={() => app.backend.signOutContainers()}
      />
      <LinkSignIn
        title={t('containers.accountTitle', names)}
        note={t('containers.accountNote', names)}
        signedInNote={t('containers.accountSignedInNote', names)}
        buttonLabel={t('containers.accountSignIn', names)}
        warning={t('containers.accountWarning', names)}
        signedIn={!!containers.accountSignedIn}
        canStart={built}
        start={() => app.backend.startContainerAccountSignIn()}
        finish={code => app.backend.finishContainerAccountSignIn(code)}
        cancel={() => app.backend.cancelContainerAccountSignIn()}
        signOut={() => app.backend.signOutContainerAccount()}
      />
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
    margin: 0 0 var(--space-3);
    font-size: var(--text-lg);
  }
  h4 {
    margin: 0 0 var(--space-1);
    font-size: var(--text-md);
  }
  .part + .part {
    margin-top: var(--space-4);
  }
  .muted {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .small {
    font-size: var(--text-xs);
  }
  .note {
    font-size: var(--text-sm);
    color: var(--warning);
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
  .channels {
    display: flex;
    gap: var(--space-1);
    margin: 0 0 var(--space-3);
  }
  .warn {
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--warning-soft);
    color: var(--warning);
    font-size: var(--text-sm);
  }
</style>
