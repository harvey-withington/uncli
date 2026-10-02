<script lang="ts">
  import { autosize } from '../lib/actions'
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { attachmentIcon, formatSize, pendingKey, readImage, toRefs, type Pending } from '../lib/attachments'
  import { ATTACH_EVENT, INSERT_EVENT, quotePath } from '../lib/drops'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  let text = $state('')
  let sending = $state(false)
  let input: HTMLTextAreaElement | undefined = $state()
  let attachments = $state<Pending[]>([]) // files going with the next turn

  // Drafts (text and attachments) are per session and survive switching
  // between sessions.
  const drafts = new Map<string, { text: string; attachments: Pending[] }>()
  let shownFor = ''
  $effect(() => {
    const id = session.id
    if (id !== shownFor) {
      if (shownFor) drafts.set(shownFor, { text, attachments })
      const d = drafts.get(id)
      text = d?.text ?? ''
      attachments = d?.attachments ?? []
      shownFor = id
      queueMicrotask(() => input?.focus())
    }
  })

  const canSend = $derived((text.trim() !== '' || attachments.length > 0) && !sending)

  async function send() {
    const q = text.trim()
    if ((!q && attachments.length === 0) || session.busy || sending) return
    sending = true
    try {
      await app.send(q, toRefs(attachments))
      text = ''
      attachments = []
      drafts.delete(session.id)
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      sending = false
      queueMicrotask(() => input?.focus())
    }
  }

  async function stop() {
    try {
      await app.backend.interrupt(session.id)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  // Files dropped or pasted on the message box attach to the next turn.
  // Folders, and files that can't be attached, go in as their paths, with a
  // note saying why.
  async function attachPaths(paths: string[]) {
    let infos
    try {
      infos = await app.backend.describeAttachments(paths)
    } catch (e) {
      showToast(String(e), 'error')
      return
    }
    const known = new Set(attachments.map(a => a.path))
    const fresh = infos.filter(i => i.kind !== 'folder' && i.kind !== 'unsupported' && !known.has(i.path))
    attachments = [...attachments, ...fresh.map(i => ({ key: pendingKey(), name: i.name, size: i.size, kind: i.kind, path: i.path, mediaType: i.mediaType }))]
    const rest = infos.filter(i => i.kind === 'folder' || i.kind === 'unsupported')
    if (rest.length > 0) {
      input?.dispatchEvent(new CustomEvent(INSERT_EVENT, { detail: rest.map(i => quotePath(i.path)).join(' ') }))
      const r = rest[0]
      if (r) showToast(t(rest.length === 1 ? 'composer.notAttached' : 'composer.notAttachedMany', { name: r.name, reason: r.reason ?? '', n: rest.length }), 'info')
    }
  }

  function detach(key: string) {
    attachments = attachments.filter(a => a.key !== key)
    input?.focus()
  }

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
          showToast(String(err), 'error')
        }
      } else if (pasted) {
        el.dispatchEvent(new CustomEvent(INSERT_EVENT, { detail: pasted }))
      }
    }
    app.backend.clipboardFiles().then(
      paths => (paths.length ? attachPaths(paths) : fallback()),
      () => fallback(),
    )
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      send()
    }
  }
</script>

<div class="composer">
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
            <span class="csize">{formatSize(a.size)}</span>
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
      use:autosize={240}
      rows="2"
      placeholder={session.busy ? t('composer.waiting') : t('composer.placeholder')}
      aria-label={t('composer.label')}
      {onkeydown}
      {onpaste}
    ></textarea>
  </div>
  {#if session.busy}
    <button class="btn stop" onclick={stop} title={t('composer.stop')}>
      <Icon name="square" size={13} />{t('composer.stop')}
    </button>
  {:else}
    <button class="btn primary icon send" onclick={send} disabled={!canSend} aria-label={t('composer.send')} title={t('composer.sendHint')}>
      <Icon name="arrow-up" />
    </button>
  {/if}
</div>

<style>
  .composer {
    display: flex;
    align-items: flex-end;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-2) var(--space-2) var(--space-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-sm);
    transition: border-color var(--fast) var(--ease), box-shadow var(--fast) var(--ease);
  }
  .composer:focus-within {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
  }
  .field {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
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
  textarea {
    flex: 1;
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
