<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { confirm } from '../lib/confirm.svelte'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { tintStyle } from '../lib/tint'
  import { showToast } from '../lib/toasts.svelte'
  import ActivityBadge from './ActivityBadge.svelte'
  import Icon from './Icon.svelte'

  interface Props {
    session: SessionView
    active: boolean
  }

  let { session, active }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const profile = $derived(app.boot?.profiles.find(p => p.id === session.profileId))
  let editing = $state(false)
  let draft = $state('')

  function startRename() {
    draft = session.title
    editing = true
  }

  async function commitRename() {
    if (!editing) return
    editing = false
    const title = draft.trim()
    if (title === session.title) return
    try {
      app.upsertSession(await app.backend.rename(session.id, title))
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  // Alt+Up / Alt+Down move the session: the keyboard twin of dragging it.
  function moveWithKeys(e: KeyboardEvent) {
    if (!e.altKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return
    e.preventDefault()
    const from = app.activeSessions.findIndex(s => s.id === session.id)
    app.moveSession(session.id, e.key === 'ArrowUp' ? from - 1 : from + 2).then(() => {
      document.querySelector<HTMLElement>(`[data-id="${session.id}"] .main`)?.focus()
    })
  }

  async function archive(e: MouseEvent, on: boolean) {
    e.stopPropagation()
    try {
      await app.archive(session.id, on)
      showToast(t(on ? 'archive.done' : 'archive.restored', { title: session.title || t('session.untitled') }), 'success')
    } catch (err) {
      showToast(String(err), 'error')
    }
  }

  async function remove(e: MouseEvent) {
    e.stopPropagation()
    const ok = await confirm({
      title: t('session.deleteTitle'),
      message: t('session.deleteMessage', { title: session.title || t('session.untitled') }),
      confirmLabel: t('session.delete'),
      danger: true,
    })
    if (!ok) return
    try {
      await app.remove(session.id)
    } catch (err) {
      showToast(String(err), 'error')
    }
  }
</script>

<li class="item" class:active data-id={session.id} style={tintStyle(profile?.hue)}>
  {#if editing}
    <div class="row">
      <span class="mode"><Icon name={profile?.icon ?? 'message-circle'} /></span>
      <!-- svelte-ignore a11y_autofocus -->
      <input
        class="input rename"
        bind:value={draft}
        autofocus
        aria-label={t('session.rename')}
        onblur={commitRename}
        onkeydown={e => {
          if (e.key === 'Enter') commitRename()
          if (e.key === 'Escape') { e.stopPropagation(); editing = false }
        }}
      />
    </div>
  {:else}
    <button
      class="row main"
      onclick={() => app.select(session.id)}
      ondblclick={startRename}
      onkeydown={moveWithKeys}
      aria-current={active ? 'true' : undefined}
      aria-keyshortcuts="Alt+ArrowUp Alt+ArrowDown"
      title={t('session.moveHint')}
    >
      <span class="mode" title={profile?.label}><Icon name={profile?.icon ?? 'message-circle'} label={profile?.label} /></span>
      <span class="text">
        <span class="title" class:untitled={!session.title}>{session.title || t('session.untitled')}</span>
        <span class="meta">
          <ActivityBadge state={session.state} />
          {#if !session.busy && (session.background?.length ?? 0) > 0}
            <span class="bg" title={t('background.hint', names)}><Icon name="loader" spin size={11} />{t('background.short')}</span>
          {/if}
          {#if session.state === 'idle'}<span class="model">{session.model}</span>{/if}
        </span>
      </span>
    </button>
    <span class="actions" data-no-drag>
      <button class="btn ghost small icon" onclick={startRename} aria-label={t('session.rename')} title={t('session.rename')}><Icon name="pencil" size={13} /></button>
      {#if session.archived}
        <button class="btn ghost small icon" onclick={e => archive(e, false)} aria-label={t('archive.restore')} title={t('archive.restore')}><Icon name="archive-restore" size={13} /></button>
      {:else}
        <button class="btn ghost small icon" onclick={e => archive(e, true)} aria-label={t('archive.archive')} title={t('archive.archive')}><Icon name="archive" size={13} /></button>
      {/if}
      <button class="btn ghost small icon" onclick={remove} aria-label={t('session.delete')} title={t('session.delete')}><Icon name="trash" size={13} /></button>
    </span>
  {/if}
</li>

<style>
  .item {
    position: relative;
    list-style: none;
    border-radius: var(--radius);
    transition: background var(--fast) var(--ease);
  }
  .item:hover {
    background: var(--surface-2);
  }
  .item.active {
    background: var(--surface);
    box-shadow: var(--shadow-sm), inset 0 0 0 1px var(--border), inset 3px 0 0 var(--tint, var(--accent));
  }
  .row {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    width: 100%;
    padding: 9px var(--space-3);
    border: 0;
    background: none;
    text-align: left;
    border-radius: var(--radius);
  }
  .mode {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    margin-top: 1px;
    border-radius: 8px;
    background: var(--tint-soft, var(--surface-2));
    color: var(--tint, var(--text-muted));
    flex: none;
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: 1px;
  }
  .title {
    font-size: var(--text-sm);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .untitled {
    color: var(--text-muted);
    font-style: italic;
  }
  .meta {
    display: flex;
    gap: var(--space-2);
    min-height: 18px;
    align-items: center;
  }
  .model {
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .actions {
    position: absolute;
    top: 6px;
    right: 6px;
    display: flex;
    padding-left: var(--space-3);
    border-radius: var(--radius-sm);
    background: linear-gradient(to right, transparent, var(--surface-2) 12px);
    opacity: 0;
    transition: opacity var(--fast) var(--ease);
  }
  .item.active .actions {
    background: linear-gradient(to right, transparent, var(--surface) 12px);
  }
  .item:hover .actions,
  .item:focus-within .actions {
    opacity: 1;
  }
  .bg {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: var(--text-xs);
    color: var(--accent);
  }
  .rename {
    flex: 1;
    height: 28px;
  }
</style>
