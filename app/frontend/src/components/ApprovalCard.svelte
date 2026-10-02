<script lang="ts">
  import type { Approval, ApprovalDecision, SessionView } from '../lib/api'
  import { describeApproval, ruleLabel } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // A tool use waiting for the user's answer: what Claude wants to do, in
  // plain words, and Allow / Always allow … / Deny. "Always" adds a rule
  // for this project (the session's folder); the choice of rule starts at
  // the most specific suggestion.
  interface Props {
    session: SessionView
    approval: Approval
  }

  let { session, approval }: Props = $props()
  const app = useApp()
  const view = $derived(describeApproval(approval))
  let choice = $state(0)
  let scope = $state<'session' | 'always'>('session') // how long "Always allow" lasts
  let busy = $state(false)

  async function answer(decision: ApprovalDecision) {
    if (busy) return
    busy = true
    try {
      const rule = decision === 'always' || decision === 'session' ? approval.suggestions[choice] : undefined
      await app.backend.answerApproval(session.id, approval.requestId, decision, rule)
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
</script>

<div class="card" role="alertdialog" aria-label={view.title} aria-describedby="approval-{approval.requestId}">
  <div class="head">
    <span class="icon"><Icon name="shield" size={16} /></span>
    <span class="title">{view.title}</span>
    <span class="hint">{t('approval.waiting')}</span>
  </div>
  <div class="what" id="approval-{approval.requestId}">
    {#if view.target}<code class="target">{view.target}</code>{/if}
    {#if view.note}<span class="note">{view.note}</span>{/if}
    {#if view.body}
      {#if view.bodyLabel}<span class="label">{view.bodyLabel}</span>{/if}
      <pre class="body" class:diff={view.lang === 'diff'}>{view.body}</pre>
    {/if}
  </div>
  <div class="actions">
    <button class="btn primary small" onclick={() => answer('allow')} disabled={busy}>{t('approval.allow')}</button>
    {#if approval.suggestions.length > 0}
      <span class="always">
        <button class="btn small" onclick={() => answer(scope)} disabled={busy}>{t('approval.always')}</button>
        {#if approval.suggestions.length > 1}
          <select class="scope" bind:value={choice} aria-label={t('approval.alwaysWhat')} disabled={busy}>
            {#each approval.suggestions as s, i (i)}
              <option value={i}>{ruleLabel(s)}</option>
            {/each}
          </select>
        {:else}
          <span class="scope-one">{ruleLabel(approval.suggestions[0] ?? { tool: approval.tool })}</span>
        {/if}
        <select class="scope" bind:value={scope} aria-label={t('approval.howLong')} disabled={busy}>
          <option value="session">{t('approval.forSession')}</option>
          <option value="always">{t('approval.forProject')}</option>
        </select>
      </span>
    {/if}
    {#if approval.suggestions.length === 0}
      <span class="once">{t('approval.onceOnly')}</span>
    {/if}
    <button class="btn small deny" onclick={() => answer('deny')} disabled={busy}>{t('approval.deny')}</button>
  </div>
</div>

<style>
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
  }
  .what {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin: var(--space-2) 0 var(--space-3);
    min-width: 0;
  }
  .target {
    display: block;
    padding: 6px 10px;
    border-radius: var(--radius-sm);
    background: var(--surface);
    border: 1px solid var(--border);
    font-family: var(--mono);
    font-size: var(--text-sm);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .note {
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .label {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .body {
    margin: 0;
    max-height: 180px;
    overflow: auto;
    padding: 6px 10px;
    border-radius: var(--radius-sm);
    background: var(--surface);
    border: 1px solid var(--border);
    font-family: var(--mono);
    font-size: var(--text-xs);
    line-height: 1.5;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
  }
  .always {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .scope {
    height: 26px;
    max-width: 280px;
    padding: 0 4px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font-size: var(--text-xs);
  }
  .scope-one {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .once {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .deny {
    margin-left: auto;
    color: var(--danger);
  }
</style>
