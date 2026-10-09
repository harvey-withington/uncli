<script lang="ts">
  import type { CLIStatus, Provider } from '../lib/api'
  import { confirm } from '../lib/confirm.svelte'
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
  let opened = $state(false) // the CLI's sign-in is open in a terminal window
  let device = $state<{ url: string; code: string } | null>(null) // a device sign-in waiting for approval
  let starting = $state(false)
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

  // A plugin (decision 0013) runs nothing until the user enables it, as its
  // manifest is now; Settings shows what it would run first.
  const plugin = $derived(provider.plugin)

  async function enable() {
    const p = provider.plugin
    if (!p?.hash) return
    const ok = await confirm({
      title: t('plugin.enableTitle', names),
      message: t(p.bypasses ? 'plugin.enableBypasses' : 'plugin.enableMessage', names),
      confirmLabel: t('plugin.enable', names),
      danger: p.bypasses,
    })
    if (!ok) return
    await run(async () => {
      await app.backend.enablePlugin(provider.id, p.hash ?? '')
      p.enabled = true
      p.changed = false
      await app.loadProviders(true)
    })
  }

  async function disable() {
    await run(async () => {
      await app.backend.disablePlugin(provider.id)
      if (provider.plugin) provider.plugin.enabled = false
      await app.loadProviders(true)
    })
  }

  async function signInTerminal() {
    await run(async () => {
      await app.backend.signInTerminal(provider.id)
      opened = true
    })
  }

  async function deviceSignIn() {
    starting = true
    await run(async () => {
      const d = await app.backend.startDeviceSignIn(provider.id)
      device = d.code ? d : null
    })
    starting = false
  }

  async function cancelDevice() {
    device = null
    await run(() => app.backend.cancelDeviceSignIn(provider.id))
  }

  // Signed in: the code has done its job.
  $effect(() => {
    if (status?.loggedIn) device = null
  })

  async function check() {
    checking = true
    await run(async () => update(await app.backend.providerStatus(provider.id, true)))
    checking = false
  }
</script>

<section id="settings-provider-{provider.id}" class="provider" aria-labelledby="provider-{provider.id}">
  <h3 id="provider-{provider.id}">{provider.name}{#if plugin}<span class="badge">{t('plugin.badge')}</span>{/if}</h3>

  {#if plugin?.error}
    <p class="warn" role="alert"><Icon name="triangle-alert" size={14} />{t('plugin.broken', { error: plugin.error })}</p>
    <p class="muted small">{t('plugin.folder', { folder: plugin.folder })}</p>
  {:else if plugin && !plugin.enabled}
    <div id={sid('plugin')} class="part">
      {#if plugin.changed}<p class="note">{t('plugin.changed', names)}</p>{/if}
      <p class="muted">{t('plugin.review', { ...names, folder: plugin.folder })}</p>
      <dl class="facts">
        <dt>{t('plugin.sources')}</dt>
        <dd>{(plugin.sources ?? []).join(', ')}</dd>
        <dt>{t('plugin.runs')}</dt>
        <dd><code>{(plugin.command ?? []).join(' ')}</code></dd>
      </dl>
      {#if plugin.bypasses}
        <p class="warn" role="note"><Icon name="triangle-alert" size={14} />{t('plugin.bypasses', names)}</p>
      {/if}
      <button class="btn primary" onclick={enable}>{t('plugin.enable', names)}</button>
    </div>
  {:else if !first && !status?.installed}
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
      {#if provider.signIn === 'device'}
        <p class="muted">
          {#if status?.loggedIn}<Icon name="check" size={14} />{/if}
          {t(status?.loggedIn ? 'settings.deviceSignedIn' : 'settings.deviceSignIn', names)}
        </p>
        {#if device}
          <div class="device" role="status">
            <span class="muted small">{t('settings.deviceApprove', names)}</span>
            <code class="code">{device.code}</code>
            <span class="muted small"><Icon name="loader" spin size={13} />{t('settings.deviceWaiting')}</span>
          </div>
        {/if}
        <div class="buttons">
          {#if !status?.loggedIn && !device}
            <button class="btn small primary" onclick={deviceSignIn} disabled={starting}>
              {#if starting}<Icon name="loader" spin size={13} />{:else}<Icon name="log-in" size={13} />{/if}{t('settings.deviceStart')}
            </button>
          {/if}
          {#if device}
            <button class="btn small" onclick={() => device && app.backend.openURL(device.url)}><Icon name="external-link" size={13} />{t('settings.deviceOpen')}</button>
            <button class="btn small ghost" onclick={cancelDevice}>{t('common.cancel')}</button>
          {/if}
          <button class="btn small" onclick={check} disabled={checking}>
            {#if checking}<Icon name="loader" spin size={13} />{/if}{t('settings.checkAgain')}
          </button>
        </div>
      {:else if provider.signIn === 'elsewhere'}
        <p class="muted">
          {#if status?.loggedIn}<Icon name="check" size={14} />{/if}
          {t(status?.loggedIn ? 'settings.signedInElsewhere' : 'settings.signInElsewhere', names)}
        </p>
        {#if status?.error}<p class="note">{status.error}</p>{/if}
        {#if opened && !status?.loggedIn}
          <p class="muted" role="status"><Icon name="loader" spin size={14} />{t('settings.signInOpened', names)}</p>
        {/if}
        <div class="buttons">
          {#if !status?.loggedIn}
            <button class="btn small" class:primary={!opened} onclick={signInTerminal}>
              <Icon name="terminal" size={13} />{t(opened ? 'settings.signInAgain' : 'settings.signInTerminal')}
            </button>
          {/if}
          <button class="btn small" onclick={check} disabled={checking}>
            {#if checking}<Icon name="loader" spin size={13} />{/if}{t('settings.checkAgain')}
          </button>
          <button class="btn small ghost" onclick={() => run(() => app.backend.revealCLI(provider.id))}>
            <Icon name="folder-open" size={13} />{t('settings.showCLI', names)}
          </button>
        </div>
      {:else if status?.email}
        <p class="muted">{t('settings.signedIn', { email: status.email, plan: status.subscription ?? '' })}</p>
      {:else}
        <p class="muted">{t('settings.notSignedIn', names)}</p>
      {/if}
    </div>
  {/if}

  {#if plugin?.enabled}
    <div class="part">
      <button class="btn small" onclick={disable}>{t('plugin.disable', names)}</button>
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
  .device {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-1);
    margin: 0 0 var(--space-3);
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }
  .device .muted {
    margin: 0;
  }
  .code {
    font-family: var(--mono);
    font-size: var(--text-lg);
    font-weight: 600;
    letter-spacing: 0.08em;
    user-select: all;
  }
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .badge {
    margin-left: var(--space-2);
    padding: 1px var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: 500;
    vertical-align: middle;
  }
  .facts {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
  }
  .facts dt {
    color: var(--text-muted);
  }
  .facts dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .warn + .btn {
    margin-top: var(--space-3);
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
