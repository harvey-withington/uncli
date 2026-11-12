<script lang="ts">
  import { tick, untrack } from 'svelte'
  import type { Profile, Share } from '@uncli/chat/lib/api'
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import FolderBrowser from './FolderBrowser.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import Modal from '@uncli/chat/components/Modal.svelte'
  import SuggestedContainers from './SuggestedContainers.svelte'
  import ImportContainerDialog from './ImportContainerDialog.svelte'
  import type { SuggestedContainer } from '@uncli/chat/lib/api'
  import { atServer, onlyAtServer } from '@uncli/chat/lib/remoteonly'
  import { modelOptions } from '@uncli/chat/lib/models'
  import { LOCAL, providerOf } from '@uncli/chat/lib/api'
  import { SHARES, shareableFor, startingShares } from '@uncli/chat/lib/shares'
  import { useShell } from '@uncli/chat/lib/shell.svelte'
  import { ServerPlace, thisPC, type Place } from '../stores/place.svelte'
  import ProfilePicker from './ProfilePicker.svelte'
  import SharesField from './SharesField.svelte'
  import WhereField from './WhereField.svelte'

  const shell = useShell()

  const app = useApp()
  // Where it runs (decision 0031): this PC, or a paired server, whose
  // profiles, providers, models and folders the dialog then offers.
  const local = thisPC(app)
  const remote: Record<string, ServerPlace> = {}
  let where = $state(LOCAL)
  const place = $derived<Place>(where === LOCAL ? local : remote[where]!)
  const here = $derived(where === LOCAL && atServer(app.backend))
  function choose(key: string) {
    if (key !== LOCAL && !remote[key]) {
      remote[key] = new ServerPlace(key, app.backend.peer(key))
      void remote[key].load()
    }
    where = key
  }
  // A walkthrough's New session opens where its session is, and Start
  // sends what the walkthrough knows as the first message.
  const seed = app.newSessionSeed
  const seedWhere = seed?.where ?? LOCAL
  if (seedWhere !== LOCAL) choose(seedWhere)
  const profiles = $derived(place.boot?.profiles ?? [])
  // The welcome card clicked, else what was picked last time, else the first.
  const last = $derived(place.boot?.lastNewSession)
  const known = (id: string | null | undefined) => (id && profiles.some(p => p.id === id) ? id : null)
  const firstProfile = known(app.newSessionProfile) ?? known(app.boot?.lastNewSession?.profileId) ?? app.boot?.profiles[0]?.id ?? 'chat'
  let profileId = $state(firstProfile)
  // A folder dropped onto the window opens the dialog with it filled in
  // (for the type it opens on).
  const dropped = app.newSessionFolder
  let folder = $state('')
  let model = $state('')
  // Where the CLI runs: '' for this computer, else a built container
  // profile (decision 0011). Offered once one is built.
  let container = $state('')
  const built = $derived((place.containers?.containers ?? []).filter(c => c.built))
  if (app.boot?.platform === 'windows' && !app.containers) void app.loadContainers()
  let creating = $state(false)
  // The AI provider (its CLI) the session runs on, when there's a choice.
  // Plugins only once enabled (decision 0013).
  const providers = $derived((place.boot?.providers ?? []).filter(p => !p.plugin || (p.plugin.enabled && !p.plugin.error)))
  let provider = $state('')
  const isFirst = $derived(!provider || provider === providers[0]?.id)
  const status = $derived(place.statusOf(provider))
  const ready = $derived(!!status?.installed && !!status?.loggedIn)
  const names = $derived(place.namesFor(provider))

  const profile = $derived<Profile | undefined>(profiles.find(p => p.id === profileId))
  const needsFolder = $derived(profile?.folder !== 'scratch')
  const options = $derived(modelOptions(place.modelsFor(provider), model, t('model.cliDefault', names)))
  // What an isolated type can use of the user's own (decision 0016).
  const caps = $derived(providerOf(place.boot, provider)?.capabilities ?? (isFirst ? place.boot?.capabilities : undefined))
  const shareable = $derived(shareableFor(profile, caps, container))
  let shares = $state<Share[]>([])

  // The model a provider starts on: the last one used with this profile on
  // it, else the profile's (the first provider's) or the CLI's first.
  function modelFor(id: string, p: string): string {
    const lastModel = last?.models?.[id]
    if ((last?.providers?.[id] ?? providers[0]?.id) === p && lastModel !== undefined) return lastModel
    if (!p || p === providers[0]?.id) return profile?.model ?? 'sonnet'
    return place.modelsFor(p)?.[0]?.value ?? ''
  }

  function chooseProvider(p: string) {
    provider = p
    model = modelFor(profileId, p)
    if (!isFirst) container = '' // containers run the first provider only
  }

  $effect(() => {
    // On each profile, start from the provider, model and folder used with
    // it last time, else the first provider, the profile's model and no
    // folder. Another place (once loaded) starts over on its own profiles.
    if (!place.boot) return
    const id = known(profileId) ?? known(last?.profileId) ?? profiles[0]?.id ?? profileId
    if (id !== profileId) {
      profileId = id
      return
    }
    untrack(() => {
      const was = last?.providers?.[id]
      provider = was && providers.some(p => p.id === was) ? was : (providers[0]?.id ?? '')
      model = modelFor(id, provider)
      const opened = where === seedWhere && id === firstProfile
      folder = (opened && dropped) || last?.folders?.[id] || ''
      const wasIn = opened && seed ? seed.container : (last?.containers?.[id] ?? '')
      container = isFirst && built.some(c => c.id === wasIn) ? wasIn : ''
      shares = startingShares(profiles.find(p => p.id === id), last, SHARES)
    })
  })

  // A container the folder's repository suggests, being imported (decision 0028).
  let importing = $state<{ s: SuggestedContainer; repo: string; name: string } | null>(null)
  let imported = $state(0)

  let startButton: HTMLButtonElement | undefined = $state()
  // Away from the server there's no folder dialog: its folders are browsed.
  let browsing = $state(false)
  const pickTitle = $derived(profile?.folder === 'repo' ? t('newSession.pickRepo') : t('newSession.pickFolder'))

  async function picked(dir: string) {
    browsing = false
    folder = dir
    // Ready to go: the next Enter starts the session.
    await tick()
    startButton?.focus()
  }

  async function pick() {
    if (!here) {
      browsing = true
      return
    }
    try {
      const dir = await app.backend.pickFolder(pickTitle)
      if (dir) await picked(dir)
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  // Enter anywhere in the dialog body does the next useful thing: start the
  // session, or first ask for the folder it still needs. Buttons keep their
  // own Enter.
  function onkeydown(e: KeyboardEvent) {
    if (e.key !== 'Enter' || e.isComposing || e.shiftKey || e.ctrlKey || e.altKey || e.metaKey) return
    if ((e.target as HTMLElement).tagName === 'BUTTON') return
    e.preventDefault()
    if (creating) return
    if (needsFolder && !folder) pick()
    else create()
  }

  async function create() {
    if (!profile || (needsFolder && !folder) || !ready) return
    creating = true
    try {
      await app.createSession(profile.id, needsFolder ? folder : '', model, container, isFirst ? '' : provider, shareable.length ? shares.filter(s => shareable.includes(s)) : null, place.backend, seed?.title ?? '')
      app.newSessionOpen = false
      if (seed?.prompt) await app.send(seed.prompt)
    } catch (e) {
      shell.toast(String(e), 'error')
    } finally {
      creating = false
    }
  }
</script>

<Modal title={t('newSession.title')} width={560} onclose={() => (app.newSessionOpen = false)}>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div {onkeydown}>
    {#if seed}<p class="hint seed"><Icon name="git-branch-plus" size={13} />{t('newSession.fromWalk')}</p>{/if}
    <WhereField {where} onchoose={choose} error={where === LOCAL ? '' : remote[where]?.error ?? ''} loading={!place.boot} />
    <ProfilePicker {profiles} bind:value={profileId} />

    {#if needsFolder}
      <div class="field">
        <span class="flabel">{profile?.folder === 'repo' ? t('newSession.repo') : t('newSession.folder')}</span>
        <div class="folder">
          <span class="path" class:placeholder={!folder} title={folder}>{folder || t('newSession.noFolder')}</span>
          <button class="btn" onclick={pick}><Icon name="folder-open" size={14} />{t('newSession.choose')}</button>
        </div>
      </div>
    {:else}
      <p class="hint">{t('newSession.scratchHint')}</p>
    {/if}

    {#if providers.length > 1}
      <label class="field">
        <span class="flabel">{t('newSession.provider')}</span>
        <select class="select" value={provider} onchange={e => chooseProvider(e.currentTarget.value)}>
          {#each providers as p (p.id)}
            {@const st = place.statusOf(p.id)}
            <option value={p.id}>{st?.installed && st.loggedIn ? p.name : t('newSession.notSetUp', { cli: p.name })}</option>
          {/each}
        </select>
      </label>
      {#if !ready}
        <p class="hint setup">
          {t(status?.installed ? 'newSession.providerSignIn' : 'newSession.providerInstall', names)}
          <button class="btn small" onclick={() => { app.newSessionOpen = false; app.openSettings(`provider-${provider}`) }} disabled={!here} title={here ? undefined : onlyAtServer()}>{t('newSession.setUp')}</button>
        </p>
      {/if}
    {/if}

    {#if built.length && isFirst}
      <label class="field">
        <span class="flabel">{t('newSession.runIn')}</span>
        <select class="select" bind:value={container}>
          <option value="">{t('newSession.thisComputer')}</option>
          {#each built as c (c.id)}
            <option value={c.id}>{c.label}</option>
          {/each}
        </select>
      </label>
      {#if container && !place.containers?.signedIn}
        <p class="hint">{t('newSession.containerSignIn')}</p>
      {/if}
    {/if}
    {#if needsFolder && folder && isFirst && app.boot?.platform === 'windows' && here}
      <SuggestedContainers {folder} bind:container refresh={imported} onimport={(s, repo, name) => (importing = { s, repo, name })} />
    {/if}

    {#if shareable.length}
      <SharesField {shareable} bind:shares {names} />
    {/if}

    <label class="field">
      <span class="flabel">{t('newSession.model')}</span>
      <select class="select" bind:value={model}>
        {#each options as o (o.value)}
          <option value={o.value}>{o.label}</option>
        {/each}
      </select>
    </label>
  </div>

  {#snippet footer()}
    {#if app.boot?.capabilities.import}
      <button class="btn ghost small import-link" onclick={() => { app.newSessionOpen = false; app.importOpen = true }}>
        <Icon name="download" size={13} />{t('import.fromNew')}
      </button>
    {/if}
    <button class="btn" onclick={() => (app.newSessionOpen = false)}>{t('common.cancel')}</button>
    <button class="btn primary" bind:this={startButton} onclick={create} disabled={creating || (needsFolder && !folder) || !ready}>
      {#if creating}<Icon name="loader" spin size={14} />{/if}
      {t('newSession.create')}
    </button>
  {/snippet}
</Modal>

{#if browsing}
  <FolderBrowser backend={place.backend} title={pickTitle} start={folder} onpick={picked} onclose={() => (browsing = false)} />
{/if}
{#if importing}
  <ImportContainerDialog suggestion={importing.s} repo={importing.repo} name={importing.name} onclose={() => { importing = null; imported++ }} />
{/if}

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: var(--space-4);
  }
  .flabel {
    font-size: var(--text-sm);
    font-weight: 500;
  }
  .folder {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .path {
    flex: 1;
    min-width: 0;
    height: 32px;
    line-height: 30px;
    padding: 0 var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    font-family: var(--mono);
    font-size: var(--text-xs);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .placeholder {
    color: var(--text-faint);
    font-family: var(--font);
  }
  .hint {
    margin: 0 0 var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .hint.seed {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .hint.setup {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: calc(-1 * var(--space-2));
    color: var(--warning);
  }
  .hint.setup .btn {
    flex: none;
  }
  .import-link {
    margin-right: auto;
    color: var(--text-muted);
  }
</style>
