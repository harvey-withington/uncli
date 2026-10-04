<script lang="ts">
  import type { Approval, ApprovalDecision, Scope, SessionView } from '../lib/api'
  import { askingReasons, classLabel, describeApproval, fixedReason, loadScope, modeOf, saveScope } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // A tool use waiting for the user's answer: why Claude is asking, what it
  // wants to do in plain words, and Allow once / This is safe / Deny.
  // "This is safe" remembers the class of each part that prompted (shown
  // under the buttons) on the safe list, for this project or all projects;
  // the scope starts where the user last left it. Under Always there is no
  // "This is safe" (the user asked to be prompted), and a part whose risk
  // comes from what it touches can only be allowed once, which a line says.
  interface Props {
    session: SessionView
    approval: Approval
  }

  let { session, approval }: Props = $props()
  const app = useApp()
  const view = $derived(describeApproval(approval))
  // Why it needs the user, in plain words, before the command itself.
  const reasons = $derived(askingReasons(approval))
  const always = $derived(modeOf(session) === 'always')
  const learn = $derived(always ? [] : approval.learn)
  const fixed = $derived(always || learn.length > 0 ? undefined : fixedReason(approval))
  let scope = $state<Scope>(loadScope())
  let busy = $state(false)

  async function answer(decision: ApprovalDecision) {
    if (busy) return
    busy = true
    try {
      if (decision === 'safe') saveScope(scope)
      await app.backend.answerApproval(session.id, approval.requestId, decision, decision === 'safe' ? scope : undefined)
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
  {#if reasons.length > 0}
    <div class="asking" aria-label={t('asking.title')}>
      <Icon name="triangle-alert" size={14} />
      <ul>
        {#each reasons as r, i (i)}
          <li>{#if r.part && reasons.length > 1}<code>{r.part}</code> {/if}{r.text}</li>
        {/each}
      </ul>
    </div>
  {/if}
  <div class="what" id="approval-{approval.requestId}">
    {#if view.target}<code class="target">{view.target}</code>{/if}
    {#if view.note}<span class="note">{view.note}</span>{/if}
    {#if view.body}
      {#if view.bodyLabel}<span class="label">{view.bodyLabel}</span>{/if}
      <pre class="body" class:diff={view.lang === 'diff'}>{view.body}</pre>
    {/if}
  </div>
  <div class="actions">
    <button class="btn primary small" onclick={() => answer('allow')} disabled={busy}>{t('approval.allowOnce')}</button>
    {#if learn.length > 0}
      <span class="safe">
        <button class="btn small" onclick={() => answer('safe')} disabled={busy}><Icon name="shield-check" size={13} />{t('approval.safe')}</button>
        <select class="scope" bind:value={scope} aria-label={t('scope.label')} disabled={busy}>
          <option value="project">{t('scope.project')}</option>
          <option value="all">{t('scope.all')}</option>
        </select>
      </span>
    {/if}
    <button class="btn small deny" onclick={() => answer('deny')} disabled={busy}>{t('approval.deny')}</button>
  </div>
  {#if learn.length > 0}
    <p class="remember">
      {t('approval.willRemember')}
      {#each learn as c, i (i)}{#if i > 0}, {/if}<code>{classLabel(c)}</code>{/each}
      {#if learn.some(c => c.kind === 'command')}<span class="any">{t('approval.anyFiles')}</span>{/if}
    </p>
  {:else if fixed}
    <p class="remember">{fixed}</p>
  {/if}
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
  .asking {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font-size: var(--text-sm);
    font-weight: 520;
  }
  .asking :global(svg) {
    flex: none;
    margin-top: 2px;
    color: var(--warning);
  }
  .asking ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .asking code {
    font-family: var(--mono);
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
  .safe {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .safe .btn {
    gap: 5px;
  }
  .scope {
    height: 26px;
    padding: 0 4px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font-size: var(--text-xs);
  }
  .remember {
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .remember code {
    font-family: var(--mono);
    color: var(--text);
  }
  .deny {
    margin-left: auto;
    color: var(--danger);
  }
</style>
