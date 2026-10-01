<script lang="ts">
  import { tick } from 'svelte'
  import type { Profile } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'
  import { modelOptions } from '../lib/models'
  import { tintStyle } from '../lib/tint'

  const app = useApp()
  const profiles = $derived(app.boot?.profiles ?? [])
  // The welcome card clicked, else what was picked last time, else the first.
  const last = app.boot?.lastNewSession
  const known = (id: string | null | undefined) => (id && profiles.some(p => p.id === id) ? id : null)
  const firstProfile = known(app.newSessionProfile) ?? known(last?.profileId) ?? app.boot?.profiles[0]?.id ?? 'chat'
  let profileId = $state(firstProfile)
  // A folder dropped onto the window opens the dialog with it filled in
  // (for the type it opens on).
  const dropped = app.newSessionFolder
  let folder = $state('')
  let model = $state('')
  let creating = $state(false)

  const profile = $derived<Profile | undefined>(profiles.find(p => p.id === profileId))
  const needsFolder = $derived(profile?.folder !== 'scratch')
  const options = $derived(modelOptions(app.boot?.models ?? null, model || undefined))

  $effect(() => {
    // On each profile, start from the model and folder used with it last
    // time, else the profile's default model and no folder.
    const id = profileId
    model = last?.models?.[id] ?? profile?.model ?? 'sonnet'
    folder = (id === firstProfile && dropped) || last?.folders?.[id] || ''
  })

  let startButton: HTMLButtonElement | undefined = $state()

  async function pick() {
    try {
      const dir = await app.backend.pickFolder(profile?.folder === 'repo' ? t('newSession.pickRepo') : t('newSession.pickFolder'))
      if (dir) {
        folder = dir
        // Ready to go: the next Enter starts the session.
        await tick()
        startButton?.focus()
      }
    } catch (e) {
      showToast(String(e), 'error')
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
    if (!profile || (needsFolder && !folder)) return
    creating = true
    try {
      await app.createSession(profile.id, needsFolder ? folder : '', model)
      app.newSessionOpen = false
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      creating = false
    }
  }
</script>

<Modal title={t('newSession.title')} width={560} onclose={() => (app.newSessionOpen = false)}>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div {onkeydown}>
    <fieldset class="profiles">
      <legend class="visually-hidden">{t('newSession.mode')}</legend>
      {#each profiles as p (p.id)}
        <label class="profile" class:selected={p.id === profileId} style={tintStyle(p.hue)}>
          <input type="radio" name="profile" value={p.id} bind:group={profileId} class="visually-hidden" data-autofocus={p.id === profileId ? '' : undefined} />
          <span class="picon"><Icon name={p.icon} size={18} /></span>
          <span class="plabel">{p.label}</span>
          <span class="pdesc">{t(`profile.${p.id}.desc`)}</span>
        </label>
      {/each}
    </fieldset>

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
    <button class="btn" onclick={() => (app.newSessionOpen = false)}>{t('common.cancel')}</button>
    <button class="btn primary" bind:this={startButton} onclick={create} disabled={creating || (needsFolder && !folder)}>
      {#if creating}<Icon name="loader" spin size={14} />{/if}
      {t('newSession.create')}
    </button>
  {/snippet}
</Modal>

<style>
  .profiles {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-2);
    border: 0;
    margin: 0 0 var(--space-4);
    padding: 0;
  }
  .profile {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: border-color var(--fast) var(--ease), background var(--fast) var(--ease);
  }
  .profile:hover {
    border-color: var(--border-strong);
  }
  .profile.selected {
    border-color: var(--tint, var(--accent));
    background: var(--tint-soft, var(--accent-soft));
  }
  .profile:has(input:focus-visible) {
    outline: 2px solid var(--tint, var(--accent));
    outline-offset: 2px;
  }
  .picon {
    color: var(--tint, var(--text-muted));
    margin-bottom: var(--space-1);
  }
  .plabel {
    font-weight: 600;
  }
  .pdesc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.4;
  }
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
</style>
