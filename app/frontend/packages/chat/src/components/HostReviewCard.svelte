<script lang="ts">
  import type { HostReview, ReviewAction } from '../lib/hostreview'
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // An embedding host's review of a batch of its own changes (decision
  // 0027, lib/hostreview.ts): its title and words, each change ticked to
  // untick, and its buttons (Apply and Discard unless it gave its own). In
  // the approval cards' place and look; the host gets the button chosen and
  // the changes still ticked. A primary button waits for at least one tick.
  interface Props {
    review: HostReview
  }

  let { review }: Props = $props()
  const chat = useChat()
  const items = $derived(review.items ?? [])
  let off = $state<Set<string>>(new Set())
  const chosen = $derived(items.filter(i => !off.has(i.id)).map(i => i.id))
  const actions = $derived<ReviewAction[]>(
    review.actions?.length
      ? review.actions
      : [
          { id: 'apply', label: t('review.apply'), tone: 'primary' },
          { id: 'discard', label: t('review.discard'), tone: 'danger' },
        ],
  )

  function toggle(id: string) {
    const next = new Set(off)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    off = next
  }
  const all = (on: boolean) => (off = on ? new Set() : new Set(items.map(i => i.id)))
  const choose = (a: ReviewAction) => chat.answerReview(review.id, { action: a.id, items: chosen })
</script>

<div class="card" role="alertdialog" aria-label={review.title} aria-describedby="review-{review.id}">
  <div class="head">
    <span class="icon"><Icon name="list-checks" size={16} /></span>
    <span class="title">{review.title}</span>
    {#if items.length > 1}
      <span class="hint">{t('review.chosen', { n: chosen.length, total: items.length })}</span>
    {/if}
  </div>
  <div class="what" id="review-{review.id}">
    {#if review.detail}<p class="detail">{review.detail}</p>{/if}
    {#if items.length}
      {#if items.length > 1}
        <div class="bulk">
          <button class="link" onclick={() => all(true)} disabled={off.size === 0}>{t('review.all')}</button>
          <button class="link" onclick={() => all(false)} disabled={chosen.length === 0}>{t('review.none')}</button>
        </div>
      {/if}
      <ul class="items" aria-label={t('review.items')}>
        {#each items as i (i.id)}
          <li>
            <label>
              <input type="checkbox" checked={!off.has(i.id)} onchange={() => toggle(i.id)} />
              <span class="text">
                <span class="label">{i.label}</span>
                {#if i.detail}<span class="more">{i.detail}</span>{/if}
              </span>
            </label>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <div class="actions">
    {#each actions as a (a.id)}
      <button
        class="btn small"
        class:primary={a.tone === 'primary'}
        class:deny={a.tone === 'danger'}
        disabled={a.tone === 'primary' && items.length > 0 && chosen.length === 0}
        onclick={() => choose(a)}>{a.label}</button
      >
    {/each}
  </div>
</div>

<style>
  /* The approval card's look (ApprovalCard.svelte). */
  .card {
    max-width: calc(var(--reading-width) + 2 * var(--space-6));
    margin: 0 auto var(--space-3);
    padding: var(--space-3) var(--space-4);
    border: 1px solid color-mix(in srgb, var(--warning) 45%, var(--border));
    border-radius: var(--radius-lg);
    background: color-mix(in srgb, var(--warning-soft) 60%, var(--surface));
    box-shadow: var(--shadow-md);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .icon {
    display: inline-flex;
    color: var(--warning);
  }
  .title {
    font-weight: 620;
  }
  .hint {
    margin-left: auto;
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
  }
  .what {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: var(--space-2) 0 var(--space-3);
    min-width: 0;
  }
  .detail {
    margin: 0;
    white-space: pre-line;
    font-size: var(--text-sm);
    color: var(--text);
  }
  .bulk {
    display: flex;
    gap: var(--space-3);
    font-size: var(--text-xs);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    cursor: pointer;
  }
  .link:disabled {
    color: var(--text-faint);
    cursor: default;
  }
  .items {
    margin: 0;
    padding: 4px 0;
    list-style: none;
    max-height: 240px;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
  }
  .items label {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: 4px 10px;
    cursor: pointer;
  }
  .items input {
    flex: none;
    margin: 3px 0 0;
    accent-color: var(--accent);
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .label {
    font-size: var(--text-sm);
    color: var(--text);
    overflow-wrap: anywhere;
  }
  .more {
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: pre-line;
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
  }
  .deny {
    margin-left: auto;
    color: var(--danger);
  }
</style>
