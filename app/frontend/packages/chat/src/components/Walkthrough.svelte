<script lang="ts">
  import type { ExplainOptions, PageExplanation, TouchedFile } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { deckOf, recordedOf, type Deck } from '../lib/deck'
  import { codeHTML } from '../lib/deckcode'
  import { loadExplainOptions, saveExplainOptions } from '../lib/explainoptions'
  import { cost } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { modelLabel } from '../lib/models'
  import { walkContext } from '../lib/walkcontext'
  import DeckViewer from './DeckViewer.svelte'
  import ExplainDialog from './ExplainDialog.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'
  import { useShell } from '../lib/shell.svelte'

  // Walk through a page's changes (decisions 0029, 0030): a button by the
  // changed files, shown when the turn's changes were recorded, opening a
  // slide show of them, a changed place a slide, in a large dialog
  // (DeckViewer). Escape closes it (or first leaves full screen). Explain
  // asks a model to explain them, with the options last used; the arrow
  // beside it opens the options (ExplainDialog) first. The explanation is
  // kept with the page. An embedding host can leave Explain out. New
  // session starts one in the same place with what the walkthrough knows
  // (lib/walkcontext.ts), where the app offers it (chat.newSessionFrom).
  interface Props {
    sessionId: string
    pageId: string
    question: string
    files: TouchedFile[]
    explanation?: PageExplanation
  }
  let { sessionId, pageId, question, files, explanation }: Props = $props()
  const chat = useChat()
  const shell = useShell()
  const recorded = $derived(recordedOf(files))
  // "1 change in 1 file", "3 changes in 2 files".
  const count = (places: number, files: number) => t('walk.count', {
    changes: t(places === 1 ? 'walk.changes.one' : 'walk.changes.other', { n: places }),
    files: t(files === 1 ? 'walk.files.one' : 'walk.files.other', { n: files }),
  })
  const label = $derived(count(recorded.places, recorded.files))
  const session = $derived(chat.sessions.find(s => s.id === sessionId))
  const models = $derived(chat.boot?.models ?? null)

  let shown = $state<{ deck: Deck; codes: string[] } | null>(null)
  let loading = $state(false)
  let explained = $state<PageExplanation | null>(null)
  const current = $derived(explained ?? explanation ?? null)
  const by = $derived(current ? t('explain.by', { model: modelLabel(models, current.model), cost: cost(current.costUsd) }) : '')
  let busy = $state(false)
  let choosing = $state(false)

  async function open(e: MouseEvent) {
    e.stopPropagation()
    if (loading) return
    loading = true
    try {
      const changes = await chat.backend.pageChanges(sessionId, pageId)
      const deck = deckOf(changes, files.filter(f => f.how !== 'deleted' || f.hunks), session?.workdir ?? '')
      shown = { deck, codes: await Promise.all(deck.slides.map(codeHTML)) }
    } catch (err) {
      shell.toast(String(err), 'error')
    } finally {
      loading = false
    }
  }

  async function run(o: ExplainOptions) {
    choosing = false
    saveExplainOptions(o)
    busy = true
    try {
      const page = await chat.backend.explainPage(sessionId, pageId, o)
      explained = page.explanation ?? null
    } catch (err) {
      shell.toast(String(err), 'error')
    } finally {
      busy = false
    }
  }

  // A new session from what the walkthrough knows (the explanation, when
  // there is one), started from the app's new session dialog.
  function branch() {
    if (!shown || !chat.newSessionFrom) return
    const first = question.trim().split('\n')[0]!.slice(0, 80)
    const context = walkContext(question, shown.deck, current)
    shown = null
    chat.newSessionFrom(sessionId, context, first ? t('walk.branchTitle', { title: first }) : '')
  }
</script>

{#if recorded.places > 0}
  <button class="btn small walk" onclick={open} disabled={loading} title={label}>
    {#if loading}<Icon name="loader" spin size={13} />{:else}<Icon name="presentation" size={13} />{/if}{t('walk.button')}
  </button>
{/if}

{#if shown}
  <Modal title={t('walk.title')} width={1100} onclose={() => (shown = null)}>
    <DeckViewer deck={shown.deck} codes={shown.codes} title={question.trim().split('\n')[0]!.slice(0, 200) || t('walk.title')}
      summary={count(shown.deck.slides.length, shown.deck.files)} explanation={current} {by}
      explain={chat.explain ? { busy, run: () => run(loadExplainOptions()), options: () => (choosing = true) } : undefined}
      branch={chat.newSessionFrom ? branch : undefined} />
  </Modal>
{/if}

{#if choosing}
  <ExplainDialog {sessionId} {pageId} start={loadExplainOptions()} sessionModel={modelLabel(models, session?.model ?? '')}
    onexplain={run} onclose={() => (choosing = false)} />
{/if}

<style>
  .walk {
    flex: none;
  }
</style>
