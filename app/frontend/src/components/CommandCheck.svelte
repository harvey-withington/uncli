<script lang="ts">
  import type { Explanation } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import ReasonList from './ReasonList.svelte'

  // "Check a command": what the session would do if Claude ran it now
  // (run it, ask, or refuse) and why for each part, so the user can see
  // what is allowed without waiting for Claude to try. Nothing is run.
  interface Props {
    sessionId: string
  }

  let { sessionId }: Props = $props()
  const app = useApp()
  let command = $state('')
  let result = $state<Explanation | null>(null)
  let checked = $state('') // the command the result is for

  async function check() {
    const c = command.trim()
    if (!c) return
    try {
      result = await app.backend.explainCommand(sessionId, c)
      checked = c
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  const ICON = { allow: 'check', ask: 'hand', deny: 'shield-x' } as const
</script>

<h3 class="group">{t('check.title')}</h3>
<form class="check" onsubmit={e => { e.preventDefault(); void check() }}>
  <input class="input small" bind:value={command} placeholder="git status; npm test" aria-label={t('check.label')} />
  <button class="btn small" type="submit" disabled={!command.trim()}>{t('check.go')}</button>
</form>
{#if result}
  <div class="result {result.action}" role="status" aria-label={t('check.resultFor', { command: checked })}>
    <p class="verdict"><Icon name={ICON[result.action]} size={14} />{t(`check.${result.action}`)}</p>
    <ReasonList reasons={result.why} label={t('check.reasons')} />
  </div>
{/if}

<style>
  .group {
    margin: var(--space-3) 0 var(--space-1) var(--space-2);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .check {
    display: flex;
    gap: var(--space-2);
  }
  .check .input {
    flex: 1;
    min-width: 0;
    font-family: var(--mono);
  }
  .small {
    height: 28px;
    font-size: var(--text-xs);
  }
  .result {
    margin-top: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    font-size: var(--text-xs);
  }
  .verdict {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0 0 4px;
    font-weight: 600;
  }
  .allow .verdict {
    color: var(--success);
  }
  .ask .verdict {
    color: var(--warning);
  }
  .deny .verdict {
    color: var(--danger);
  }
</style>
