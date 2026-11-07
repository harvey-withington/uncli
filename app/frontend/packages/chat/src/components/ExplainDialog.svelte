<script lang="ts">
  import { untrack } from 'svelte'
  import type { ExplainOptions } from '../lib/api'
  import { useChat } from '../lib/chatcontext'
  import { tokensLabel } from '../lib/explainoptions'
  import { t } from '../lib/i18n.svelte'
  import { modelOptions } from '../lib/models'
  import Modal from './Modal.svelte'

  // Explain's options (decision 0029): the model, what it's told besides
  // the changes (this page, the last few pages, or the whole session), and
  // how much it says, with roughly how much that sends. It starts from the
  // options last used.
  interface Props {
    sessionId: string
    pageId: string
    start: ExplainOptions
    sessionModel: string // what '' means: the session's model, by name
    onexplain: (o: ExplainOptions) => void
    onclose: () => void
  }
  let { sessionId, pageId, start, sessionModel, onexplain, onclose }: Props = $props()
  const chat = useChat()
  let o = $state<ExplainOptions>(untrack(() => ({ ...start })))
  const models = $derived(modelOptions(chat.modelsFor(), o.model, t('explain.sessionModel', { model: sessionModel })))
  let size = $state<number | null>(null)

  // Roughly what it would send, asked again a moment after a change.
  $effect(() => {
    const opts = $state.snapshot(o)
    size = null
    const timer = setTimeout(() => {
      chat.backend.explainSize(sessionId, pageId, opts).then(n => (size = n), () => (size = null))
    }, 250)
    return () => clearTimeout(timer)
  })
</script>

<Modal title={t('explain.title')} width={460} {onclose}>
  <form class="form" onsubmit={e => { e.preventDefault(); onexplain($state.snapshot(o)) }}>
    <label class="field">
      <span class="label">{t('explain.model')}</span>
      <select class="select" bind:value={o.model} data-autofocus>
        {#if o.model !== ''}<option value="">{t('explain.sessionModel', { model: sessionModel })}</option>{/if}
        {#each models as m (m.value)}<option value={m.value}>{m.label}</option>{/each}
      </select>
    </label>

    <fieldset class="field">
      <legend class="label">{t('explain.context')}</legend>
      <label class="check"><input type="radio" bind:group={o.context} value="page" />{t('explain.context.page')}</label>
      <label class="check">
        <input type="radio" bind:group={o.context} value="pages" />{t('explain.context.pages')}
        <input class="input pages" type="number" min="1" max="50" bind:value={o.pages} aria-label={t('explain.pagesCount')} onfocus={() => (o.context = 'pages')} />
      </label>
      <label class="check"><input type="radio" bind:group={o.context} value="session" />{t('explain.context.session')}</label>
    </fieldset>

    <fieldset class="field">
      <legend class="label">{t('explain.detail')}</legend>
      <label class="check"><input type="radio" bind:group={o.detail} value="brief" />{t('explain.detail.brief')}</label>
      <label class="check"><input type="radio" bind:group={o.detail} value="thorough" />{t('explain.detail.thorough')}</label>
    </fieldset>

    <p class="size" aria-live="polite">{size === null ? ' ' : t('explain.size', { tokens: tokensLabel(size) })}</p>

    <div class="actions">
      <button type="button" class="btn" onclick={onclose}>{t('common.cancel')}</button>
      <button type="submit" class="btn primary">{t('explain.run')}</button>
    </div>
  </form>
</Modal>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0;
    padding: 0;
    border: 0;
  }
  .label {
    padding: 0;
    margin-bottom: 2px;
    font-size: var(--text-sm);
    font-weight: 500;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }
  .pages {
    width: 4.5em;
    height: 26px;
  }
  .size {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
