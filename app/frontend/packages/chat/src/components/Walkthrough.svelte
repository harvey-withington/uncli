<script lang="ts">
  import type { TouchedFile } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { deckOf, recordedOf } from '../lib/deck'
  import type { DeckMessage } from '../lib/deckhtml'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'
  import { useShell } from '../lib/shell.svelte'

  // Walk through a page's changes (decision 0029): a button by the changed
  // files, shown when the turn's changes were recorded, opening a slide
  // show of them, a changed place a slide, in a large dialog. reveal.js
  // plays it in a sandboxed iframe, built only when it's opened; Full
  // screen puts the frame on the whole screen. Escape (and F) inside the
  // deck come out here: the first leaves full screen, the next closes.
  interface Props {
    sessionId: string
    pageId: string
    question: string
    files: TouchedFile[]
  }
  let { sessionId, pageId, question, files }: Props = $props()
  const chat = useChat()
  const shell = useShell()
  const recorded = $derived(recordedOf(files))
  // "1 change in 1 file", "3 changes in 2 files".
  const count = (places: number, files: number) => t('walk.count', {
    changes: t(places === 1 ? 'walk.changes.one' : 'walk.changes.other', { n: places }),
    files: t(files === 1 ? 'walk.files.one' : 'walk.files.other', { n: files }),
  })
  const label = $derived(count(recorded.places, recorded.files))

  let doc = $state<string | null>(null)
  let loading = $state(false)
  let frame = $state<HTMLIFrameElement>()
  let host = $state<HTMLElement>()

  async function open(e: MouseEvent) {
    e.stopPropagation()
    if (loading) return
    loading = true
    try {
      const [changes, { deckHTML }] = await Promise.all([chat.backend.pageChanges(sessionId, pageId), import('../lib/deckhtml')])
      const workdir = chat.sessions.find(s => s.id === sessionId)?.workdir ?? ''
      const deck = deckOf(changes, files.filter(f => f.how !== 'deleted' || f.hunks), workdir)
      const scheme = getComputedStyle(host ?? document.documentElement).colorScheme || 'light dark'
      doc = await deckHTML(deck, {
        title: question.trim().split('\n')[0]!.slice(0, 200) || t('walk.title'),
        summary: count(deck.slides.length, deck.files),
        file: s => t('walk.file', { file: s.rel, from: s.first, to: s.last, part: s.part, parts: s.parts }),
        others: t('walk.others'),
        keys: t('walk.keys'),
      }, scheme)
    } catch (err) {
      shell.toast(String(err), 'error')
    } finally {
      loading = false
    }
  }

  function close() {
    if (document.fullscreenElement) void document.exitFullscreen()
    doc = null
  }

  function fullscreen() {
    if (document.fullscreenElement) void document.exitFullscreen()
    else void frame?.requestFullscreen().catch(() => {})
  }

  // Only this deck's frame may talk to it.
  function onmessage(e: MessageEvent) {
    if (!frame || e.source !== frame.contentWindow) return
    const what = (e.data as Partial<DeckMessage> | null)?.['uncli-deck']
    if (what === 'fullscreen') fullscreen()
    else if (what === 'escape') {
      if (document.fullscreenElement) void document.exitFullscreen()
      else close()
    }
  }
</script>

<svelte:window {onmessage} />

{#if recorded.places > 0}
  <button class="btn small walk" bind:this={host} onclick={open} disabled={loading} title={label}>
    {#if loading}<Icon name="loader" spin size={13} />{:else}<Icon name="presentation" size={13} />{/if}{t('walk.button')}
  </button>
{/if}

{#if doc}
  <Modal title={t('walk.title')} width={1200} onclose={close}>
    <div class="deck">
      <div class="bar">
        <span class="hint">{label}</span>
        <button class="btn small" onclick={fullscreen}><Icon name="maximize" size={13} />{t('walk.fullscreen')}</button>
      </div>
      <iframe bind:this={frame} title={t('walk.title')} srcdoc={doc} sandbox="allow-scripts" allow="fullscreen" allowfullscreen data-autofocus></iframe>
    </div>
  </Modal>
{/if}

<style>
  .walk {
    flex: none;
  }
  .deck {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .hint {
    flex: 1;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  iframe {
    width: 100%;
    height: min(72vh, 720px);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
  }
  iframe:fullscreen {
    border: 0;
    border-radius: 0;
  }
</style>
