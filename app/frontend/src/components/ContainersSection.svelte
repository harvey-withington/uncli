<script lang="ts">
  import type { ContainerInfo } from '../lib/api'
  import { confirm } from '../lib/confirm.svelte'
  import { useApp } from '../lib/context'
  import { i18n, t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import { blank, idFor, profileOf } from '../lib/containers'
  import ContainerEditor from './ContainerEditor.svelte'
  import Icon from './Icon.svelte'

  // Settings → Containers (decision 0011): sessions can run in a WSL
  // distro UNCLI builds and locks down. This tab turns WSL on and builds or
  // removes each container profile, and edits them (ContainerEditor). Each AI provider signs in for
  // containers on its own (AI Providers); this says which have.
  const app = useApp()
  const info = $derived(app.containers)
  let installing = $state(false)
  // The container being edited, or 'new'; one at a time.
  let editing = $state('')
  const ids = $derived(info?.containers.map(c => c.id) ?? [])
  // Containers whose Build was just clicked: they show "Starting…" at once,
  // until the first progress arrives, so a second click can't happen.
  let starting = $state<Record<string, boolean>>({})
  const busy = (c: ContainerInfo) => !!c.step || !!starting[c.id]

  async function build(c: ContainerInfo) {
    if (busy(c)) return
    starting[c.id] = true
    try {
      await app.backend.buildContainer(c.id)
      // The build has started: show it even before its first update.
      const cur = app.containers?.containers.find(x => x.id === c.id)
      if (cur && !cur.step) cur.step = 'download'
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      starting[c.id] = false
    }
  }
  // Containers run the first provider only (for now).
  const providers = $derived((app.boot?.providers ?? []).slice(0, 1))

  $effect(() => {
    void app.loadContainers()
  })

  async function run(f: () => Promise<unknown>) {
    try {
      await f()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  let restarting = $state(false)

  async function stopAll() {
    const ok = await confirm({
      title: t('containers.stopTitle'),
      message: t('containers.stopMessage'),
      confirmLabel: t('containers.stop'),
      danger: true,
    })
    if (!ok) return
    restarting = true
    await run(async () => {
      await app.backend.stopContainers()
      await app.loadContainers()
    })
    restarting = false
  }

  async function installWSL() {
    installing = true
    await run(async () => {
      await app.backend.installWSL()
      showToast(t('containers.restart'))
    })
    installing = false
  }

  async function remove(c: ContainerInfo) {
    const ok = await confirm({
      title: t('containers.removeTitle', { label: c.label }),
      message: t('containers.removeMessage'),
      confirmLabel: t('containers.remove'),
      danger: true,
    })
    if (ok) await run(() => app.backend.removeContainer(c.id))
  }

  // What a container shares of the user (containers.yaml brain and mcp).
  function shares(c: ContainerInfo): string {
    const what = [
      c.brain === 'shared' && t('containers.share.brain'),
      c.mcp === 'shared' && t('containers.share.mcp'),
      c.connectors === 'shared' && t('containers.share.connectors'),
    ].filter((x): x is string => !!x)
    if (!what.length) return ''
    return t('containers.shares', { what: new Intl.ListFormat(i18n.locale, { type: 'conjunction' }).format(what) })
  }

  // Wants the user's connectors, but there's no full account sign-in yet:
  // it runs with the models-only token meanwhile.
  const waitsForAccount = (c: ContainerInfo) => c.connectors === 'shared' && !info?.accountSignedIn

  // Built, but from other packages, base, CLI or setup than now: a rebuild
  // brings it up to date.
  const changed = (c: ContainerInfo) => c.built && !busy(c) && (c.changes?.length ?? 0) > 0

  function changedLine(c: ContainerInfo): string {
    const ch = c.changes ?? []
    if (ch.includes('unknown')) return t('containers.changedUnknown')
    return t('containers.changedSince', { what: ch.map(k => t(`containers.changed.${k}`)).join(', ') })
  }

  function status(c: ContainerInfo): string {
    if (c.step === 'download' && c.percent) return t('containers.step.downloadPercent', { n: c.percent })
    if (c.step) return t(`containers.step.${c.step}`)
    if (starting[c.id]) return t('containers.starting')
    if (changed(c)) return t('containers.changed')
    return c.built ? t('containers.built') : t('containers.notBuilt')
  }
</script>

<section id="settings-containers">
  <h3>{t('containers.title')}</h3>
  <p class="muted">{t('containers.body')}</p>
  {#if !info}
    <p class="muted"><Icon name="loader" spin size={14} />{t('containers.checking')}</p>
  {:else if info.error}
    <p class="warn" role="alert"><Icon name="triangle-alert" size={14} />{info.error}</p>
  {:else if info.wsl.unresponsive}
    <p class="warn" role="alert"><Icon name="triangle-alert" size={14} />{t('containers.unresponsive')}</p>
    <button class="btn" onclick={stopAll} disabled={restarting}>
      {#if restarting}<Icon name="loader" spin size={14} />{/if}{t('containers.stop')}
    </button>
  {:else if !info.wsl.installed}
    <p class="note">{t('containers.noWSL')}</p>
    <button class="btn primary" onclick={installWSL} disabled={installing}>
      {#if installing}<Icon name="loader" spin size={14} />{:else}<Icon name="download" size={14} />{/if}{t('containers.installWSL')}
    </button>
  {:else}
    <p class="muted small">{t('containers.wslVersion', { version: info.wsl.version ?? '' })}</p>
    <ul class="list">
      {#each info.containers as c (c.id)}
        <li>
          <Icon name="layers" size={16} />
          <div class="what">
            <span class="name">{c.label}</span>
            <span class="detail" title={c.packages.join(' ')}>{[c.baseLabel, ...c.packages].join(' · ')}</span>
            {#if shares(c)}<span class="shares">{shares(c)}</span>{/if}
            {#if waitsForAccount(c)}<span class="changed">{t('containers.needsAccount')}</span>{/if}
            {#if changed(c)}<span class="changed">{changedLine(c)}</span>{/if}
            {#if c.error}<span class="error" role="alert">{c.error}</span>{/if}
          </div>
          <span class="status" class:ok={c.built && !busy(c) && !changed(c)} class:warn={changed(c)} aria-live="polite">
            {#if busy(c)}<Icon name="loader" spin size={14} />{/if}{status(c)}
          </span>
          <button class="btn small" class:primary={changed(c)} onclick={() => build(c)} disabled={busy(c)} aria-busy={busy(c)}>
            {c.built ? t('containers.rebuild') : t('containers.build')}
          </button>
          <button class="btn small ghost icon" onclick={() => (editing = editing === c.id ? '' : c.id)} aria-label={t('containerEdit.title', { label: c.label })} aria-expanded={editing === c.id} title={t('containerEdit.edit')}>
            <Icon name="pencil" size={14} />
          </button>
          {#if c.built && !busy(c)}
            <button class="btn small ghost icon" onclick={() => remove(c)} aria-label={t('containers.removeTitle', { label: c.label })} title={t('containers.remove')}>
              <Icon name="trash" size={14} />
            </button>
          {/if}
        </li>
        {#if editing === c.id}
          <li class="editing">
            <ContainerEditor start={profileOf(c)} container={c} bases={info.bases ?? []} taken={ids.filter(x => x !== c.id)} onclose={() => (editing = '')} />
          </li>
        {/if}
      {/each}
      {#if editing === 'new'}
        <li class="editing">
          <ContainerEditor start={blank(idFor('', ids), info.bases?.[0]?.id ?? '')} bases={info.bases ?? []} taken={ids} onclose={() => (editing = '')} />
        </li>
      {/if}
    </ul>
    {#if editing !== 'new'}
      <button class="btn small new" onclick={() => (editing = 'new')}><Icon name="plus" size={13} />{t('containerEdit.new')}</button>
    {/if}
    <div class="signin">
      {#each providers as p (p.id)}
        <p class="muted">
          {#if info.signedIn}<Icon name="check" size={14} />{:else}<Icon name="circle-alert" size={14} />{/if}
          {t(info.signedIn ? 'containers.providerSignedIn' : 'containers.providerNotSignedIn', { cli: p.name })}
        </p>
        {#if !info.signedIn}
          <button class="btn small" onclick={() => app.openSettings('provider-containers')}>{t('containers.signInThere')}</button>
        {/if}
      {/each}
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
  .note,
  .warn {
    display: flex;
    gap: var(--space-2);
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
  }
  .warn {
    color: var(--warning);
  }
  .list {
    list-style: none;
    margin: 0 0 var(--space-4);
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }
  .list li {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    color: var(--text-muted);
  }
  .list li + li {
    border-top: 1px solid var(--border);
  }
  .list li.editing {
    display: block;
    padding: 0;
    border-top: 0;
  }
  .new {
    margin: calc(-1 * var(--space-2)) 0 var(--space-4);
  }
  .what {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .name {
    color: var(--text);
    font-weight: 500;
  }
  .detail {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .shares {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .error {
    font-size: var(--text-xs);
    color: var(--danger);
  }
  .status {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-xs);
    white-space: nowrap;
  }
  .status.ok {
    color: var(--success);
  }
  .status.warn,
  .changed {
    color: var(--warning);
  }
  .changed {
    font-size: var(--text-xs);
  }
</style>
