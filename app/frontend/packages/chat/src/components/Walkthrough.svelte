<script lang="ts">
  import type { TouchedFile } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { deckOf, recordedOf, type Deck } from '../lib/deck'
  import { codeHTML } from '../lib/deckcode'
  import { t } from '../lib/i18n.svelte'
  import DeckViewer from './DeckViewer.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'
  import { useShell } from '../lib/shell.svelte'

  // Walk through a page's changes (decisions 0029, 0030): a button by the
  // changed files, shown when the turn's changes were recorded, opening a
  // slide show of them, a changed place a slide, in a large dialog
  // (DeckViewer). Escape closes it (or first leaves full screen).
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

  let shown = $state<{ deck: Deck; codes: string[] } | null>(null)
  let loading = $state(false)

  async function open(e: MouseEvent) {
    e.stopPropagation()
    if (loading) return
    loading = true
    try {
      const changes = await chat.backend.pageChanges(sessionId, pageId)
      const workdir = chat.sessions.find(s => s.id === sessionId)?.workdir ?? ''
      const deck = deckOf(changes, files.filter(f => f.how !== 'deleted' || f.hunks), workdir)
      shown = { deck, codes: await Promise.all(deck.slides.map(codeHTML)) }
    } catch (err) {
      shell.toast(String(err), 'error')
    } finally {
      loading = false
    }
  }
</script>

{#if recorded.places > 0}
  <button class="btn small walk" onclick={open} disabled={loading} title={label}>
    {#if loading}<Icon name="loader" spin size={13} />{:else}<Icon name="presentation" size={13} />{/if}{t('walk.button')}
  </button>
{/if}

{#if shown}
  <Modal title={t('walk.title')} width={1100} onclose={() => (shown = null)}>
    <DeckViewer deck={shown.deck} codes={shown.codes} title={question.trim().split('\n')[0]!.slice(0, 200) || t('walk.title')} summary={count(shown.deck.slides.length, shown.deck.files)} />
  </Modal>
{/if}

<style>
  .walk {
    flex: none;
  }
</style>
