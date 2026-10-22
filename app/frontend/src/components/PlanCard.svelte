<script lang="ts">
  import type { Approval, SessionView } from '../lib/api'
  import { planOf } from '../lib/asks'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import AnswerBlocks from './AnswerBlocks.svelte'
  import Icon from './Icon.svelte'

  // A plan the model wants approved before it acts (Approval.ask "plan"):
  // the plan as formatted text, then Approve to let it go ahead, or Keep
  // planning with a note on what to change, which it plans again from.
  interface Props {
    session: SessionView
    approval: Approval
  }

  let { session, approval }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const plan = $derived(planOf(approval.input))
  let note = $state('')
  let busy = $state(false)

  async function reply(allow: boolean) {
    if (busy) return
    busy = true
    try {
      await app.backend.answerAsk(session.id, approval.requestId, allow, null, allow ? '' : note)
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
</script>

<div class="card" role="alertdialog" aria-label={t('plan.title', names)}>
  <div class="head">
    <span class="icon"><Icon name="list-checks" size={16} /></span>
    <span class="title">{t('plan.title', names)}</span>
    <span class="hint">{t('approval.waiting', names)}</span>
  </div>
  {#if plan}
    <div class="plan"><AnswerBlocks markdown={plan} /></div>
  {:else}
    <p class="none">{t('plan.none', names)}</p>
  {/if}
  <textarea class="input note" rows="2" placeholder={t('plan.notePlaceholder')} aria-label={t('plan.note')} bind:value={note} disabled={busy}></textarea>
  <div class="actions">
    <button class="btn primary small" onclick={() => reply(true)} disabled={busy}>
      {#if busy}<Icon name="loader" spin size={13} />{/if}{t('plan.approve')}
    </button>
    <button class="btn small" onclick={() => reply(false)} disabled={busy}>{t('plan.keep')}</button>
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
  .plan {
    max-height: 320px;
    overflow: auto;
    margin: var(--space-2) 0;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    font-size: var(--text-sm);
  }
  /* A plan's headings are its sections, not the page's: smaller here. */
  .plan :global(h1) {
    font-size: var(--text-lg);
  }
  .plan :global(h2),
  .plan :global(h3) {
    font-size: var(--text-md);
  }
  .none {
    margin: var(--space-2) 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .note {
    width: 100%;
    height: auto;
    min-height: 44px;
    resize: vertical;
    padding: var(--space-1) var(--space-2);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
</style>
