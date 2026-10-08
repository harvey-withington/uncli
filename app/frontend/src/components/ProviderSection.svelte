<script lang="ts">
  import type { Provider } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { namesOf } from '../lib/providers'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import LinkSignIn from './LinkSignIn.svelte'

  // Settings → AI Providers: one of these per CLI sessions can run on. Its
  // version (anything but the pinned one is at the user's own risk), the
  // account it's signed in with, and on Windows its sign-in for containers
  // (a token of its own, from a link approved on any device and the code
  // the page shows; decision 0011).
  interface Props {
    provider: Provider
  }

  let { provider }: Props = $props()
  const app = useApp()
  const names = $derived(namesOf(provider))
  let version = $state(app.cli?.version ?? '')
  let channels = $state<Record<string, string>>({})
  let busy = $state(false)
  const pinned = $derived(app.cli?.pinned ?? '')
  const isPinned = $derived(!version || version === pinned)
  const containers = $derived(app.boot?.platform === 'windows' ? app.containers : null)
  const built = $derived(!!containers?.containers.some(c => c.built))

  $effect(() => {
    app.backend.cliChannels().then(c => (channels = c)).catch(() => {})
    if (app.boot?.platform === 'windows' && !app.containers) void app.loadContainers()
  })

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
      app.cli = await app.backend.setCLIVersion(v)
      version = app.cli.version
      showToast(t('settings.switched', { ...names, version: app.cli.version }), 'success')
    })
    busy = false
  }

</script>

<section id="settings-provider-{provider.id}" class="provider" aria-labelledby="provider-{provider.id}">
  <h3 id="provider-{provider.id}">{provider.name}</h3>

  <div id="settings-cli" class="part">
    <h4>{t('settings.cliTitle')}</h4>
    <p class="muted">{t('settings.cliBody', { ...names, pinned })}</p>
    {#if app.cli?.custom}
      <p class="note">{t('settings.custom')}</p>
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
    <button class="btn" onclick={() => apply('')} disabled={busy || app.cli?.version === pinned}>{t('settings.backToPinned', { pinned })}</button>
  </div>

  <div id="settings-account" class="part">
    <h4>{t('settings.account')}</h4>
    {#if app.cli?.email}
      <p class="muted">{t('settings.signedIn', { email: app.cli.email, plan: app.cli.subscription ?? '' })}</p>
    {:else}
      <p class="muted">{t('settings.notSignedIn', names)}</p>
    {/if}
  </div>

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
