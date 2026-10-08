<script lang="ts">
  import type { Approval, Scope, SessionView } from '../lib/api'
  import { cardNotes, classLabel, describeApproval, fixedReason, isShell, loadScope, modeOf, saveScope } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import CommandView from './CommandView.svelte'
  import Icon from './Icon.svelte'

  // A tool use waiting for the user's answer: what Claude wants to do (a
  // command coloured part by part, CommandView), a line on why when the
  // colours can't say it (what an unsafe part could do, the decision
  // model's reason), and Allow once / This is safe / Deny. "This is safe"
  // is a toggle: on, the classes of the parts that prompted are on the safe
  // list (in the scope beside it, where the user last left it) and the card
  // waits for Allow once or Deny; off, the list is as it was. Under Always
  // there is no "This is safe" (the user asked to be prompted), and a part
  // whose risk comes from what it touches can only be allowed once, which
  // a line says.
  interface Props {
    session: SessionView
    approval: Approval
  }

  let { session, approval }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const view = $derived(describeApproval(approval))
  const notes = $derived(cardNotes(approval))
  const always = $derived(modeOf(session) === 'always')
  const learn = $derived(always ? [] : approval.learn)
  const fixed = $derived(always || learn.length > 0 ? undefined : fixedReason(approval))
  const marked = $derived(!!approval.marked)
  let scope = $state<Scope>(loadScope())
  let busy = $state(false)

  async function act(f: () => Promise<void>) {
    if (busy) return
    busy = true
    try {
      await f()
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
  const answer = (decision: 'allow' | 'deny') => act(() => app.backend.answerApproval(session.id, approval.requestId, decision))
  function toggleSafe() {
    if (!marked) saveScope(scope)
    void act(() => app.backend.markSafe(session.id, approval.requestId, !marked, scope))
  }
</script>

<div class="card" role="alertdialog" aria-label={view.title} aria-describedby="approval-{approval.requestId}">
  <div class="head">
    <span class="icon"><Icon name="shield" size={16} /></span>
    <span class="title">{view.title}</span>
    <span class="hint">{t('approval.waiting', names)}</span>
  </div>
  {#if notes.length > 0}
    <p class="asking" aria-label={t('asking.title', names)}><Icon name="triangle-alert" size={13} /><span>{notes.join(' ')}</span></p>
  {/if}
  <div class="what" id="approval-{approval.requestId}">
    {#if view.target}
      <CommandView command={view.target} why={approval.why} shell={isShell(approval.tool)} note={view.note} />
    {:else if view.note}<span class="note">{view.note}</span>{/if}
    {#if view.body}
      {#if view.bodyLabel}<span class="label">{view.bodyLabel}</span>{/if}
      <pre class="body" class:diff={view.lang === 'diff'}>{view.body}</pre>
    {/if}
  </div>
  <div class="actions">
    <button class="btn primary small" onclick={() => answer('allow')} disabled={busy}>{t('approval.allowOnce')}</button>
    {#if learn.length > 0}
      <span class="safe">
        <button class="btn small" class:on={marked} aria-pressed={marked} onclick={toggleSafe} disabled={busy}>
          <Icon name="shield-check" size={13} />{t('approval.safe')}
        </button>
        <select class="scope" value={marked ? approval.markedScope ?? scope : scope} onchange={e => (scope = e.currentTarget.value as Scope)} aria-label={t('scope.label')} disabled={busy || marked}>
          <option value="project">{t('scope.project')}</option>
          <option value="all">{t('scope.all')}</option>
        </select>
      </span>
    {/if}
    <button class="btn small deny" onclick={() => answer('deny')} disabled={busy}>{t('approval.deny')}</button>
  </div>
  {#if learn.length > 0}
    <p class="remember">
      {t(marked ? 'approval.markedSafe' : 'approval.willRemember')}
      {#each learn as c, i (i)}{#if i > 0}{', '}{/if}<code>{classLabel(c)}</code>{/each}
      {#if learn.some(c => c.kind === 'command')}<span class="any">{t('approval.anyFiles')}</span>{/if}
      {#if marked}<span class="undo">{t('approval.undoSafe')}</span>{/if}
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
    align-items: baseline;
    gap: var(--space-2);
    margin: var(--space-2) 0 0;
    font-size: var(--text-sm);
    color: var(--text);
  }
  .asking :global(svg) {
    flex: none;
    position: relative;
    top: 2px;
    color: var(--warning);
  }
  .what {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin: var(--space-2) 0 var(--space-3);
    min-width: 0;
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
  .safe .btn.on {
    border-color: color-mix(in srgb, var(--success) 60%, var(--border));
    background: color-mix(in srgb, var(--success) 14%, var(--surface));
    color: var(--success);
  }
  .undo {
    margin-left: var(--space-2);
    color: var(--text-faint);
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
