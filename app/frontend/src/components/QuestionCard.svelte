<script lang="ts">
  import type { Approval, SessionView } from '../lib/api'
  import { answersFor, pick, questionsOf } from '../lib/asks'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // A question the model asks (Approval.ask "question"): each question with
  // its options as buttons (one, or several where it allows), and Other to
  // type an answer of one's own. Answer sends them all back; Skip declines.
  interface Props {
    session: SessionView
    approval: Approval
  }

  let { session, approval }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const questions = $derived(questionsOf(approval.input))
  let picked = $state<Record<string, string[]>>({})
  let other = $state<Record<string, string>>({})
  let busy = $state(false)
  const answers = $derived(answersFor(questions, picked, other))

  async function reply(allow: boolean) {
    if (busy || (allow && !answers)) return
    busy = true
    try {
      await app.backend.answerAsk(session.id, approval.requestId, allow, allow ? answers : null, '')
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
</script>

<div class="card" role="alertdialog" aria-label={t('ask.title', names)}>
  <div class="head">
    <span class="icon"><Icon name="message-circle-question" size={16} /></span>
    <span class="title">{t('ask.title', names)}</span>
    <span class="hint">{t('approval.waiting', names)}</span>
  </div>
  {#each questions as q, qi (q.question)}
    <fieldset class="q">
      <legend>
        {#if q.header}<span class="chip">{q.header}</span>{/if}
        <span class="text">{q.question}</span>
        {#if q.multiSelect}<span class="many">{t('ask.many')}</span>{/if}
      </legend>
      <div class="options" role="group">
        {#each q.options as o (o.label)}
          {@const on = (picked[q.question] ?? []).includes(o.label)}
          <button
            class="opt"
            class:on
            role={q.multiSelect ? 'checkbox' : 'radio'}
            aria-checked={on}
            disabled={busy}
            onclick={() => (picked[q.question] = pick(q, picked[q.question] ?? [], o.label))}
          >
            <span class="label">{o.label}</span>
            {#if o.description}<span class="desc">{o.description}</span>{/if}
          </button>
        {/each}
      </div>
      <input
        class="input other"
        placeholder={t('ask.other')}
        aria-label={t('ask.otherFor', { n: qi + 1 })}
        bind:value={other[q.question]}
        disabled={busy}
        onkeydown={e => e.key === 'Enter' && answers && void reply(true)}
      />
    </fieldset>
  {/each}
  <div class="actions">
    <button class="btn primary small" onclick={() => reply(true)} disabled={busy || !answers}>
      {#if busy}<Icon name="loader" spin size={13} />{/if}{t('ask.answer')}
    </button>
    <button class="btn small skip" onclick={() => reply(false)} disabled={busy}>{t('ask.skip')}</button>
  </div>
</div>

<style>
  .card {
    max-width: calc(var(--reading-width) + 2 * var(--space-6));
    margin: 0 auto var(--space-3);
    padding: var(--space-3) var(--space-4);
    border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--border));
    border-radius: var(--radius-lg);
    background: color-mix(in srgb, var(--accent-soft) 60%, var(--surface));
    box-shadow: var(--shadow-md);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .icon {
    display: inline-flex;
    color: var(--accent);
  }
  .title {
    font-weight: 620;
  }
  .hint {
    margin-left: auto;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .q {
    margin: var(--space-3) 0 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-2);
    padding: 0;
    margin-bottom: var(--space-2);
  }
  .chip {
    padding: 0 6px;
    border-radius: var(--radius-sm);
    background: var(--surface-3);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .text {
    font-size: var(--text-sm);
    font-weight: 560;
  }
  .many {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .options {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .opt {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    max-width: 260px;
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .opt:hover {
    border-color: var(--border-strong);
  }
  .opt.on {
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .opt .label {
    font-size: var(--text-sm);
    font-weight: 560;
  }
  .opt .desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .other {
    width: 100%;
    margin-top: var(--space-2);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .skip {
    margin-left: auto;
  }
</style>
