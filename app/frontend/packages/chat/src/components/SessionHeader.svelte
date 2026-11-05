<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { attentionCount } from '../lib/attention'
  import { useShell } from '../lib/shell.svelte'
  import { tintStyle } from '../lib/tint'
  import ActivityBadge from './ActivityBadge.svelte'
  import Icon from './Icon.svelte'
  import McpServers from './McpServers.svelte'
  import ModeSwitch from './ModeSwitch.svelte'
  import RemoteButton from './RemoteButton.svelte'
  import SessionStatus from './SessionStatus.svelte'
  import SessionTitle from './SessionTitle.svelte'
  import SharesNote from './SharesNote.svelte'
  import Toolbar from './Toolbar.svelte'

  // A session's header, in the desktop's layout or the compact one (the
  // phone's, and an embedded chat's when narrow). Both have the type's
  // icon, the title and what it's doing, the toolbar with the prompt level
  // and Unattended, the session's status and error under it, and the
  // host's header-extra slot when embedded.
  // - The desktop: the session's actions on the title's line, the folder
  //   under it, and the toolbar always shown; it collapses to two lines.
  // - Compact: Back (where there's somewhere to go back to), and the toolbar
  //   and Restart folded away under Options.

  interface Props {
    session: SessionView
    compact?: boolean
    onback?: () => void
  }

  let { session, compact = false, onback }: Props = $props()
  const chat = useChat()
  const shell = useShell()
  const names = $derived(chat.namesFor(session.adapter))
  const profile = $derived(chat.boot?.profiles.find(p => p.id === session.profileId))
  // A session in a container says which (decision 0011).
  const place = $derived(session.runtimeRef ? ` · ${t('session.inContainer', { label: chat.containers?.containers.find(c => c.id === session.runtimeRef)?.label ?? session.runtimeRef })}` : '')
  const failed = $derived(!!session.error && session.state === 'error')
  // The status line goes when the header (desktop) or the question
  // (compact) is collapsed; an error still shows.
  const quiet = $derived(compact ? chat.questionCompact : chat.headerCompact)
  let options = $state(false)
  // A dot on what opens a folded header when something in it wants the
  // user (lib/attention.ts).
  const attention = $derived(attentionCount(session))
  const attentionLabel = (what: string) => (attention === 0 ? what : attention === 1 ? t('header.attentionOne', { what }) : t('header.attentionMany', { what, n: attention }))

  // In a host's page, with no session list to delete it from: Delete chat
  // here, confirmed, and the host told (it may keep its own record).
  async function removeChat() {
    const ok = await shell.confirm({
      title: t('session.deleteTitle'),
      message: t('session.deleteMessage', { title: session.title || t('session.untitled') }),
      confirmLabel: t('session.delete'),
      danger: true,
    })
    if (!ok) return
    // Taken now: once it's deleted, this header (and its session) goes.
    const id = session.id
    const told = chat.deleted
    try {
      await chat.remove(id)
      told?.(id)
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  // Unattended: rules still apply, but anything that would wait for the
  // user is declined, and the agent is told the user is away.
  async function toggleUnattended() {
    try {
      chat.upsertSession(await chat.backend.setUnattended(session.id, !session.unattended))
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

{#snippet toolbar()}
  <Toolbar {session}>
    {#snippet end()}
      <ModeSwitch {session} />
      <button class="btn ghost small away" class:on={session.unattended} onclick={toggleUnattended} aria-pressed={!!session.unattended} title={t('unattended.hint', names)}>
        <Icon name="coffee" size={15} />{t('unattended.label')}
      </button>
    {/snippet}
  </Toolbar>
{/snippet}

{#snippet notes()}
  {#if !quiet || failed}
    <div class="notes" class:desk={!compact}>
      {#if !quiet}<SessionStatus {session} />{/if}
      {#if failed}<p class="err" role="alert"><Icon name="triangle-alert" size={14} />{session.error}</p>{/if}
    </div>
  {/if}
{/snippet}

{#snippet extra()}
  <!-- An embedding host's own controls (decision 0026). -->
  {#if chat.embedded}<svelte:element this={'slot'} name="header-extra" />{/if}
{/snippet}

{#snippet hostStart()}
  <!-- An embedding host's own controls at the start and end of the
       header's top line (its close button, say), so its header and UNCLI's
       can be one. -->
  {#if chat.embedded}<svelte:element this={'slot'} name="header-start" />{/if}
{/snippet}

{#snippet hostEnd()}
  {#if chat.embedded}<svelte:element this={'slot'} name="header-end" />{/if}
{/snippet}

{#snippet chatActions(labelled: boolean)}
  <!-- In a host's page, where there may be no session list: words in the
       compact Options, where there's room; icons on the desktop's title line. -->
  {#if chat.newChat}
    {@const start = chat.newChat}
    <button class="btn ghost small" class:icon={!labelled} onclick={() => start()} aria-label={t('chat.new')} title={t('chat.new')}><Icon name="plus" size={labelled ? 15 : 16} />{#if labelled}{t('chat.new')}{/if}</button>
  {/if}
  {#if chat.deleted}
    <button class="btn ghost small" class:icon={!labelled} onclick={removeChat} aria-label={t('chat.delete')} title={t('chat.delete')}><Icon name="trash" size={labelled ? 15 : 16} />{#if labelled}{t('chat.delete')}{/if}</button>
  {/if}
{/snippet}

{#if compact}
  <!-- One panel down to the page's question bar, which sits flush under it. -->
  <div class="head" part="header">
    <header class="bar">
      {@render hostStart()}
      <!-- Pressing Back leaves the focus where it is: a rename box losing it
           would save, where Back cancels (lib/backguard.ts). -->
      {#if onback}<button class="btn ghost icon touch" onmousedown={e => e.preventDefault()} onclick={onback} aria-label={t('mobile.back')} title={t('mobile.back')}><Icon name="arrow-left" /></button>{/if}
      <span class="mode" style={tintStyle(profile?.hue)}><Icon name={profile?.icon ?? 'message-circle'} size={15} /></span>
      <div class="title">
        <h1><SessionTitle {session} /></h1>
        <ActivityBadge state={session.state} />
      </div>
      {#if options}
        <!-- Open, Options shows every icon the desktop's title line has, each
             with its own warning: the MCP servers (whose dot the Options
             button carried while closed) and the safe list. -->
        <McpServers {session} />
        {#if chat.openSettings}
          {@const open = chat.openSettings}
          <button class="btn ghost icon touch" onclick={() => open('safe')} aria-label={t('safe.open')} title={t('safe.open')}><Icon name="shield" /></button>
        {/if}
      {/if}
      <button class="btn ghost icon touch dotted" onclick={() => (options = !options)} aria-expanded={options} aria-label={options ? t('mobile.options') : attentionLabel(t('mobile.options'))} title={options ? t('mobile.options') : attentionLabel(t('mobile.options'))}><Icon name="sliders-horizontal" />{#if !options && attention}<span class="dot" aria-hidden="true"></span>{/if}</button>
      <button class="btn ghost icon touch" onclick={() => chat.togglePanel()} aria-pressed={chat.outline.open} aria-label={t('outline.toggle')} title={t('outline.toggle')}><Icon name={chat.outline.open ? 'panel-right-close' : 'panel-right-open'} /></button>
      {@render hostEnd()}
    </header>
    {@render extra()}
    {#if options}
      <div class="options">
        {@render toolbar()}
        {#if chat.newChat || chat.deleted}
          <p class="chat-actions">{@render chatActions(true)}</p>
        {/if}
        {#if !session.archived}
          <p class="restart">
            <button class="btn ghost small away" onclick={() => chat.restart(session.id)}><Icon name="rotate-ccw" size={15} />{t('restart.label')}</button>
            <span>{t('restart.hintShort', names)}</span>
          </p>
        {/if}
      </div>
    {/if}
    {@render notes()}
  </div>
{:else}
  <header class="top" class:collapsed={chat.headerCompact} part="header">
    <div class="title-row">
      {@render hostStart()}
      <span class="mode" style={tintStyle(profile?.hue)}><Icon name={profile?.icon ?? 'message-circle'} size={15} /></span>
      <h1><SessionTitle {session} /></h1>
      <ActivityBadge state={session.state} />
      {#if chat.headerCompact}
        <!-- Collapsed: the folder follows the title on its line. -->
        <span class="path" title={session.workdir}>{profile?.label} · {session.workdir}{place}</span>
        <SharesNote {session} compact />
      {/if}
      <span class="gap"></span>
      <RemoteButton />
      <McpServers {session} />
      {#if !session.archived}
        <button class="btn ghost small icon" onclick={() => chat.restart(session.id)} aria-label={t('restart.label')} title={t('restart.hint', names)}>
          <Icon name="rotate-ccw" />
        </button>
      {/if}
      {#if chat.openSettings}
        {@const open = chat.openSettings}
        <button class="btn ghost small icon" onclick={() => open('safe')} aria-label={t('safe.open')} title={t('safe.open')}>
          <Icon name="shield" />
        </button>
      {/if}
      {@render chatActions(false)}
      <button
        class="btn ghost small icon"
        onclick={() => chat.togglePanel()}
        aria-pressed={chat.outline.open}
        aria-label={t(chat.hasArtifacts(session.id) ? 'panel.toggleBoth' : 'outline.toggle')}
        title={t(chat.hasArtifacts(session.id) ? 'panel.toggleBoth' : 'outline.toggle')}
      >
        <Icon name={chat.outline.open ? 'panel-right-close' : 'panel-right-open'} />
      </button>
      <!-- Always last on the line, expanded or collapsed. -->
      <button
        class="btn ghost small icon h-toggle dotted"
        onclick={() => chat.toggleHeaderCompact()}
        aria-expanded={!chat.headerCompact}
        aria-label={chat.headerCompact ? attentionLabel(t('header.expand')) : t('header.collapse')}
        title={chat.headerCompact ? attentionLabel(t('header.expand')) : t('header.collapse')}
      >
        <Icon name={chat.headerCompact ? 'chevron-down' : 'chevron-up'} size={14} />
        {#if chat.headerCompact && attention}<span class="dot" aria-hidden="true"></span>{/if}
      </button>
      {@render hostEnd()}
    </div>
    {#if !chat.headerCompact}
      <div class="where">
        <span class="workdir" title={session.workdir}>{profile?.label} · {session.workdir}{place}</span>
        <SharesNote {session} />
      </div>
    {/if}
    <!-- The toolbar, with the prompt level and Unattended at the right end
         of its last line (a line of their own, still at the right, when
         there's no room). -->
    {@render toolbar()}
    {@render extra()}
  </header>
  <!-- Status and warnings, in a strip of their own under the header, all
       aligned on the left. Collapsed shows only an error there. MCP servers
       that need the user are left to the plug button's dot. -->
  {@render notes()}
{/if}

<style>
  .mode {
    display: grid;
    place-items: center;
    flex: none;
    width: 26px;
    height: 26px;
    border-radius: 8px;
    background: var(--tint-soft, var(--accent-soft));
    color: var(--tint, var(--accent));
  }
  h1 {
    margin: 0;
    font-weight: 620;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .away {
    gap: 6px;
    color: var(--text-muted);
  }
  .away.on {
    background: var(--warning-soft);
    color: var(--warning);
  }
  .err {
    display: flex;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--danger);
  }

  /* The desktop. */
  .top {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-6) var(--space-3);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .title-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }
  .top h1 {
    font-size: var(--text-lg);
    letter-spacing: -0.01em;
  }
  /* Collapsed: the folder takes the room between the title and the
     controls, cut short (from the end) before anything else gives way. */
  .path {
    flex: 0 1 auto;
    min-width: 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .collapsed {
    padding-top: var(--space-3);
  }
  .collapsed h1 {
    flex: 0 1 auto;
    max-width: 45%;
  }
  .gap {
    flex: 1;
  }
  .h-toggle {
    flex: none;
    margin-left: calc(-1 * var(--space-2));
    color: var(--text-faint);
  }
  .where {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    margin: -4px 0 2px 38px;
    font-size: var(--text-xs);
  }
  .workdir {
    display: block;
    min-width: 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .notes {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 var(--space-3) var(--space-2);
  }
  .notes:not(.desk):empty {
    display: none;
  }
  .notes .err {
    margin: var(--space-1) 0;
  }
  .notes.desk {
    padding: var(--space-2) var(--space-6);
    border-bottom: 1px solid var(--border);
    background: var(--surface-2);
  }
  .notes.desk .err {
    align-items: center;
    margin: 0;
  }

  /* Compact. */
  .head {
    background: var(--surface);
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-1);
  }
  .touch {
    width: 44px;
    height: 44px;
    flex: none;
  }
  .title {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-left: var(--space-2);
  }
  .bar h1 {
    font-size: var(--text-md);
  }
  /* At most a third of the screen, scrolling past that, so the answer
     keeps most of a phone even with Options open. */
  .options {
    max-height: 33vh;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: var(--space-2) var(--space-3);
    border-top: 1px solid var(--border);
  }
  /* Folded away, the prompt level and Unattended start their line, like
     everything else in Options, rather than sitting at the right end of it. */
  .options :global(.end) {
    margin-left: 0;
    padding-left: 0;
  }
  /* The toolbar wraps here, so a divider between groups would end a line. */
  .options :global(.sep) {
    display: none;
  }
  /* The attention dot: small, top right of the button, in the warning tone. */
  .dotted {
    position: relative;
  }
  .dot {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--warning);
    box-shadow: 0 0 0 2px var(--surface);
  }
  .chat-actions {
    display: flex;
    gap: var(--space-1);
    margin: var(--space-2) 0 0;
  }
  .restart {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .restart .btn {
    flex: none;
  }
</style>
