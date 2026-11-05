<script lang="ts">
  import { autosize } from '../lib/actions'
  import type { SessionView } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { attachmentIcon, formatSize, fromRefs, pendingKey, readImage, releasePreviews, shrinkPhoto, toRefs, type Pending } from '../lib/attachments'
  import { ATTACH_EVENT, INSERT_EVENT, quotePath } from '../lib/composerevents'
  import { loadDraft, saveDraft } from '../lib/keep'
  import { atServer } from '../lib/remoteonly'
  import { enterSetting, resolveEnter, touchScreen } from '../lib/enterkey.svelte'
  import Icon from './Icon.svelte'
  import SlashMenu from './SlashMenu.svelte'
  import MentionMenu from './MentionMenu.svelte'
  import ComposerControls from './ComposerControls.svelte'
  import { useShell } from '../lib/shell.svelte'

  const shell = useShell()

  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const chat = useChat()
  let text = $state('')
  let sending = $state(false)
  let input: HTMLTextAreaElement | undefined = $state()
  let menu: SlashMenu | undefined = $state() // the slash-command menu, which sees keys first
  let mentions: MentionMenu | undefined = $state() // a host's @mentions, next
  let attachments = $state<Pending[]>([]) // files going with the next turn
  // A remote client picks files (a photo, from the camera or gallery) and
  // sends each to the server first; the turn names them by upload id.
  const remote = $derived(!atServer(chat.backend))
  let picker: HTMLInputElement | undefined = $state()
  let uploading = $state(0)

  // Drafts are per session. The text is kept on every change, so it
  // survives switching sessions and restarting UNCLI (lib/keep.ts);
  // attachments survive switching sessions.
  // focusBox puts the cursor in the box, unless the side panel covers it
  // (the compact layout; the keyboard would open for a box the user can't
  // see), or, as a chat opens, on a touch screen, where the keyboard comes
  // up only when the user taps the box.
  function focusBox(opening = false) {
    if (chat.panelCovers || (opening && touchScreen())) return
    input?.focus()
  }

  const drafts = new Map<string, Pending[]>()
  let shownFor = ''
  $effect(() => {
    const id = session.id
    if (id !== shownFor) {
      if (shownFor) drafts.set(shownFor, attachments)
      text = loadDraft(id)
      attachments = drafts.get(id) ?? []
      shownFor = id
      queueMicrotask(() => focusBox(true))
    }
  })
  $effect(() => {
    if (shownFor) saveDraft(shownFor, text)
  })

  // attach adds files to a session's draft: the one shown, or, when the
  // user moved on while they were on their way (an upload), the one they
  // were added from.
  function attach(id: string, items: Pending[]) {
    if (id === shownFor) attachments = [...attachments, ...items]
    else drafts.set(id, [...(drafts.get(id) ?? []), ...items])
  }

  const canSend = $derived((text.trim() !== '' || attachments.length > 0) && !sending && uploading === 0)

  // send sends the message, or, while an answer runs, queues it to go when
  // the answer ends (session/queue.go). With a message held in the queue,
  // the two go together.
  const queueing = $derived(session.busy || !!session.queued)
  async function send() {
    const q = text.trim()
    if ((!q && attachments.length === 0) || sending) return
    sending = true
    try {
      if (queueing) await chat.queue(q, toRefs(attachments))
      else await chat.send(q, toRefs(attachments))
      text = ''
      releasePreviews(attachments)
      attachments = []
      drafts.delete(session.id)
      saveDraft(session.id, '')
    } catch (e) {
      shell.toast(String(e), 'error')
    } finally {
      sending = false
      queueMicrotask(() => focusBox())
    }
  }

  // edit takes the queued message back into the box, before what's there.
  async function editQueued() {
    try {
      const q = await chat.client.session(session.id).unqueue()
      if (q.text) text = text.trim() ? `${q.text}\n\n${text}` : q.text
      if (q.attachments.length) attachments = [...fromRefs(q.attachments), ...attachments]
      focusBox()
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  async function sendQueued() {
    try {
      await chat.client.session(session.id).sendQueued()
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  async function stop() {
    try {
      await chat.backend.interrupt(session.id)
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }

  // Files dropped or pasted on the message box attach to the next turn, as
  // copied then (upload), so a file gone by Send (a screenshot tool's
  // temporary file) still goes. Folders, and files that can't be attached,
  // go in as their paths, with a note saying why.
  async function attachPaths(paths: string[]) {
    const id = session.id
    let infos
    try {
      infos = await chat.backend.describeAttachments(paths)
    } catch (e) {
      shell.toast(String(e), 'error')
      return
    }
    const known = new Set((id === shownFor ? attachments : drafts.get(id) ?? []).map(a => a.path))
    const fresh = infos.filter(i => i.kind !== 'folder' && i.kind !== 'unsupported' && !known.has(i.path))
    attach(id, fresh.map(i => ({ key: pendingKey(), name: i.name, size: i.size, kind: i.kind, path: i.path, upload: i.upload, mediaType: i.mediaType })))
    const rest = infos.filter(i => i.kind === 'folder' || i.kind === 'unsupported')
    if (rest.length > 0) {
      input?.dispatchEvent(new CustomEvent(INSERT_EVENT, { detail: rest.map(i => quotePath(i.path)).join(' ') }))
      const r = rest[0]
      if (r) shell.toast(t(rest.length === 1 ? 'composer.notAttached' : 'composer.notAttachedMany', { name: r.name, reason: r.reason ?? '', n: rest.length }), 'info')
    }
  }

  async function upload(files: FileList | null) {
    const id = session.id
    for (const file of Array.from(files ?? [])) {
      uploading++
      try {
        const body = await shrinkPhoto(file)
        const name = body === file ? file.name : file.name.replace(/\.\w+$/, '') + '.jpg'
        const up = await chat.backend.upload(body, name)
        if (up.kind === 'folder' || up.kind === 'unsupported') {
          shell.toast(t('composer.notAttached', { name: up.name, reason: up.reason ?? '' }), 'info')
          continue
        }
        const preview = up.kind === 'image' ? URL.createObjectURL(body) : undefined
        attach(id, [{ key: pendingKey(), name: up.name, size: up.size, kind: up.kind, upload: up.id, mediaType: up.mediaType, preview }])
      } catch (e) {
        shell.toast(String(e), 'error')
      } finally {
        uploading--
      }
    }
    if (picker) picker.value = ''
  }

  function detach(key: string) {
    releasePreviews(attachments.filter(a => a.key === key))
    attachments = attachments.filter(a => a.key !== key)
    focusBox()
  }

  // Previews of uploaded photos hold their bytes until released.
  $effect(() => () => {
    releasePreviews(attachments)
    for (const list of drafts.values()) releasePreviews(list)
  })

  $effect(() => {
    const el = input
    if (!el) return
    const onattach = (e: Event) => void attachPaths((e as CustomEvent<string[]>).detail)
    el.addEventListener(ATTACH_EVENT, onattach)
    return () => el.removeEventListener(ATTACH_EVENT, onattach)
  })

  // Text for the cursor (a folder's path, say) arrives as an event too.
  $effect(() => {
    const el = input
    if (!el) return
    const insert = (e: Event) => {
      const add = (e as CustomEvent<string>).detail
      const start = el.selectionStart ?? text.length
      const end = el.selectionEnd ?? text.length
      const before = text.slice(0, start)
      const pad = before && !/\s$/.test(before) ? ' ' : ''
      text = before + pad + add + text.slice(end)
      queueMicrotask(() => {
        el.focus()
        const at = start + pad.length + add.length
        el.setSelectionRange(at, at)
        el.dispatchEvent(new Event('input')) // let autosize grow it
      })
    }
    el.addEventListener(INSERT_EVENT, insert)
    return () => el.removeEventListener(INSERT_EVENT, insert)
  })

  // Pasting files attaches them, as a drop does. Files copied in Explorer
  // reach the page only as their names, so their paths come from the OS
  // clipboard; an image with no file behind it (a screenshot) attaches as
  // data. Anything else pastes as usual.
  function onpaste(e: ClipboardEvent) {
    const data = e.clipboardData
    if (!data || !(Array.from(data.types).includes('Files') || data.files.length > 0)) return
    e.preventDefault()
    const el = e.currentTarget as HTMLTextAreaElement
    const pasted = data.getData('text/plain')
    const images = Array.from(data.files).filter(f => f.type.startsWith('image/'))
    const fallback = async () => {
      if (images.length > 0) {
        try {
          const read = await Promise.all(images.map((f, i) => readImage(f, images.length > 1 ? t('composer.pastedImageN', { n: i + 1 }) : t('composer.pastedImage'))))
          attachments = [...attachments, ...read]
        } catch (err) {
          shell.toast(String(err), 'error')
        }
      } else if (pasted) {
        el.dispatchEvent(new CustomEvent(INSERT_EVENT, { detail: pasted }))
      }
    }
    chat.backend.clipboardFiles().then(
      paths => (paths.length ? attachPaths(paths) : fallback()),
      () => fallback(),
    )
  }

  // What Enter does (lib/enterkey.svelte.ts): sends, with Shift+Enter for a
  // new line; or a new line, with Ctrl+Enter (or Send) to send. On a touch
  // screen, a new line unless the host or the viewer says otherwise.
  const touch = touchScreen()
  const enter = $derived(resolveEnter(chat.enterKey, enterSetting.value, touch))
  const placeholder = $derived(
    session.busy ? t('composer.waiting') : touch ? t('composer.placeholderShort') : enter === 'send' ? t('composer.placeholder') : t('composer.placeholderNewline'),
  )

  function onkeydown(e: KeyboardEvent) {
    if (menu?.onkey(e) || mentions?.onkey(e)) return
    if (e.key !== 'Enter' || e.isComposing) return
    const sends = enter === 'send' ? !e.shiftKey : e.ctrlKey || e.metaKey
    if (sends) {
      e.preventDefault()
      send()
    }
  }
</script>

<div class="composer">
  <SlashMenu bind:this={menu} {session} bind:text {input} />
  <MentionMenu bind:this={mentions} bind:text {input} />
  {#if session.queued}
    {@const q = session.queued}
    <!-- A message waiting for the answer to end, or held for the user. -->
    <div class="queued" class:held={q.held} role="status">
      <Icon name={q.held ? 'circle-pause' : 'clock'} size={13} />
      <span class="qbody">
        <span class="qlabel">{q.held ? t('composer.queuedHeld') : t('composer.queued')}</span>
        <span class="qtext">{q.text}{#if q.files?.length}{q.text ? ' · ' : ''}{t('composer.queuedFiles', { n: q.files.length })}{/if}</span>
      </span>
      {#if q.held && !session.busy}<button class="btn small primary" onclick={sendQueued}>{t('composer.sendNow')}</button>{/if}
      <button class="btn small ghost" onclick={editQueued} title={t('composer.editQueuedHint')}><Icon name="pencil" size={12} />{t('composer.editQueued')}</button>
    </div>
  {/if}
  <div class="field">
    {#if attachments.length > 0}
      <ul class="attachments" aria-label={t('composer.attachments')}>
        {#each attachments as a (a.key)}
          <li class="chip" title={a.path ?? a.name}>
            {#if a.preview}
              <img class="thumb" src={a.preview} alt="" />
            {:else}
              <span class="kicon"><Icon name={attachmentIcon(a.kind)} size={14} /></span>
            {/if}
            <span class="cname">{a.name}</span>
            {#if a.size > 0}<span class="csize">{formatSize(a.size)}</span>{/if}
            <button class="remove" onclick={() => detach(a.key)} aria-label={t('composer.remove', { name: a.name })} title={t('composer.remove', { name: a.name })}>
              <Icon name="x" size={12} />
            </button>
          </li>
        {/each}
      </ul>
    {/if}
    <textarea
      bind:this={input}
      bind:value={text}
      use:autosize={{ max: 240, value: text }}
      rows="2"
      {placeholder}
      aria-label={t('composer.label')}
      {onkeydown}
      {onpaste}
    ></textarea>
  </div>
  <!-- The right-hand column: a host's own controls above Attach and Send,
       so the text keeps all the room there is. -->
  <div class="side">
    <ComposerControls />
    <div class="buttons">
  {#if remote}
    <input bind:this={picker} type="file" multiple accept="image/*,application/pdf,text/*,.md,.json,.txt" class="visually-hidden" tabindex="-1" aria-hidden="true" onchange={e => upload(e.currentTarget.files)} />
    <button class="btn ghost icon attach" onclick={() => picker?.click()} disabled={uploading > 0} aria-label={t('composer.attach')} title={t('composer.attach')}>
      {#if uploading > 0}<Icon name="loader" spin />{:else}<Icon name="paperclip" />{/if}
    </button>
  {/if}
  {#if session.busy}
    <button class="btn stop" onclick={stop} title={t('composer.stop')}>
      <Icon name="square" size={13} />{t('composer.stop')}
    </button>
    <button class="btn primary icon send" onclick={send} disabled={!canSend} aria-label={t('composer.queue')} title={enter === 'send' ? t('composer.queueHint') : t('composer.queueHintCtrl')}>
      <Icon name="list-plus" />
    </button>
  {:else}
    <button class="btn primary icon send" onclick={send} disabled={!canSend} aria-label={t('composer.send')} title={enter === 'send' ? t('composer.sendHint') : t('composer.sendHintCtrl')}>
      <Icon name="arrow-up" />
    </button>
  {/if}
    </div>
  </div>
</div>

<style>
  .composer {
    position: relative; /* the slash-command menu sits above it */
    display: flex;
    flex-wrap: wrap; /* a queued message takes a line of its own, above */
    align-items: flex-end;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-2) var(--space-2) var(--space-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-sm);
    transition: border-color var(--fast) var(--ease), box-shadow var(--fast) var(--ease);
    container: composer / inline-size; /* a host's controls drop their labels when it's narrow */
    /* A container is its own stacking context, so a host's drop-down inside
       it can only rise above the answer if the box itself does. */
    z-index: 4;
  }
  .side {
    display: flex;
    flex: none;
    flex-direction: column;
    align-items: flex-end;
    gap: var(--space-2);
  }
  .buttons {
    display: flex;
    align-items: flex-end;
    gap: var(--space-2);
  }
  .composer:focus-within {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
  }
  /* Full height, so the text starts at the top even when the right-hand
     column (a host's controls over Attach and Send) is the taller. */
  .field {
    flex: 1;
    align-self: stretch;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  /* A message waiting in the queue: accent while it waits for the answer,
     warning while it's held for the user. */
  .queued {
    flex: 1 1 100%;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-right: calc(var(--space-4) - var(--space-2)); /* square with the box's left inset */
    padding: 6px 8px;
    border-radius: var(--radius-sm);
    border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--border));
    background: color-mix(in srgb, var(--accent-soft) 70%, var(--surface));
    font-size: var(--text-sm);
  }
  .queued > :global(.icon) {
    color: var(--accent);
  }
  .queued.held {
    border-color: color-mix(in srgb, var(--warning) 45%, var(--border));
    background: color-mix(in srgb, var(--warning-soft) 60%, var(--surface));
  }
  .queued.held > :global(.icon) {
    color: var(--warning);
  }
  .qbody {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .qlabel {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .qtext {
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    white-space: pre-line;
    overflow-wrap: anywhere;
  }
  .queued .btn {
    flex: none;
    gap: 4px;
  }
  .attachments {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 4px 0 2px;
    padding: 0;
    list-style: none;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 260px;
    height: 30px;
    padding: 0 4px 0 6px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    font-size: var(--text-xs);
  }
  .kicon {
    display: inline-flex;
    color: var(--accent);
  }
  .thumb {
    width: 22px;
    height: 22px;
    border-radius: 4px;
    object-fit: cover;
  }
  .cname {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 550;
  }
  .csize {
    flex: none;
    color: var(--text-faint);
  }
  .remove {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: 20px;
    height: 20px;
    padding: 0;
    border: 0;
    border-radius: 4px;
    background: none;
    color: var(--text-muted);
  }
  .remove:hover {
    background: var(--surface-3);
    color: var(--text);
  }
  /* Its height is its text's (autosize), and it grows into any room the
     field has beyond that. */
  textarea {
    flex: 1 0 auto;
    min-height: 56px;
    padding: 7px 0;
    border: 0;
    outline: none;
    resize: none;
    background: transparent;
    font-size: var(--text-md);
    line-height: 1.5;
  }
  textarea::placeholder {
    color: var(--text-faint);
  }
  .send {
    width: 34px;
    height: 34px;
    border-radius: 10px;
  }
  .stop {
    height: 34px;
    border-radius: 10px;
  }
</style>
