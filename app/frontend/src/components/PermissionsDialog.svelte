<script lang="ts">
  import type { RuleAction, ToolRule } from '../lib/api'
  import { GIT_CLASSES, ruleLabel } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'

  // The tool rules of the current session's project (its folder): each
  // allows, asks about or denies a tool, a command or a kind of git
  // command. The most specific rule wins (a command, then a git class,
  // then a program, then the whole tool); anything without a rule asks.
  const app = useApp()
  const session = $derived(app.current)
  let rules = $state<ToolRule[]>([])
  let sessionRules = $state<ToolRule[]>([])
  let loading = $state(true)

  // The "add a rule" row.
  type What = (typeof GIT_CLASSES)[number] | 'command' | 'tool'
  let what = $state<What>('git:read')
  let text = $state('')
  let action = $state<RuleAction>('allow')
  const ACTIONS: RuleAction[] = ['allow', 'ask', 'deny']

  $effect(() => {
    const id = session?.id
    if (!id) return
    loading = true
    Promise.all([app.backend.toolRules(id), app.backend.sessionToolRules(id)]).then(
      ([r, sr]) => { rules = r; sessionRules = sr; loading = false },
      e => { showToast(String(e), 'error'); loading = false },
    )
  })

  async function save(r: ToolRule) {
    if (!session) return
    try {
      rules = await app.backend.setToolRule(session.id, r)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function remove(r: ToolRule) {
    if (!session) return
    try {
      rules = await app.backend.deleteToolRule(session.id, r)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function dropSession(r: ToolRule) {
    if (!session) return
    try {
      sessionRules = await app.backend.deleteSessionToolRule(session.id, r)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  async function promote(r: ToolRule) {
    if (!session) return
    try {
      await app.backend.promoteSessionToolRule(session.id, r)
      ;[rules, sessionRules] = await Promise.all([app.backend.toolRules(session.id), app.backend.sessionToolRules(session.id)])
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  function add() {
    const v = text.trim()
    if (what === 'command') {
      if (v) save({ tool: 'Bash', prefix: v, action })
    } else if (what === 'tool') {
      if (v) save({ tool: v, action })
    } else {
      save({ tool: 'Bash', prefix: what, action })
    }
    text = ''
  }

  const needsText = $derived(what === 'command' || what === 'tool')
</script>

<Modal title={t('rules.title')} width={560} onclose={() => (app.permissionsOpen = false)}>
  <p class="muted">{t('rules.body', { folder: session?.workdir ?? '' })}</p>

  {#if sessionRules.length > 0}
    <h3 class="group">{t('rules.thisSession')}</h3>
    <ul class="rules" aria-label={t('rules.thisSession')}>
      {#each sessionRules as r (r.tool + '|' + (r.prefix ?? ''))}
        <li>
          <span class="what" title={r.prefix ?? r.tool}>{ruleLabel(r)}</span>
          <span class="tag">{t('rules.action.allow')}</span>
          <button class="btn ghost small" onclick={() => promote(r)}>{t('rules.makePermanent')}</button>
          <button class="btn ghost small icon" onclick={() => dropSession(r)} aria-label={t('rules.remove', { what: ruleLabel(r) })} title={t('rules.remove', { what: ruleLabel(r) })}>
            <Icon name="trash" size={13} />
          </button>
        </li>
      {/each}
    </ul>
    <h3 class="group">{t('rules.thisProject')}</h3>
  {/if}
  {#if loading}
    <p class="muted"><Icon name="loader" spin size={14} /></p>
  {:else if rules.length === 0}
    <p class="empty">{t('rules.none')}</p>
  {:else}
    <ul class="rules" aria-label={t('rules.list')}>
      {#each rules as r (r.tool + '|' + (r.prefix ?? ''))}
        <li>
          <span class="what" title={r.prefix ?? r.tool}>{ruleLabel(r)}</span>
          <select class="select small" value={r.action} aria-label={t('rules.actionFor', { what: ruleLabel(r) })} onchange={e => save({ ...r, action: e.currentTarget.value as RuleAction })}>
            {#each ACTIONS as a (a)}<option value={a}>{t(`rules.action.${a}`)}</option>{/each}
          </select>
          <button class="btn ghost small icon" onclick={() => remove(r)} aria-label={t('rules.remove', { what: ruleLabel(r) })} title={t('rules.remove', { what: ruleLabel(r) })}>
            <Icon name="trash" size={13} />
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  <form class="add" onsubmit={e => { e.preventDefault(); add() }}>
    <select class="select small" bind:value={what} aria-label={t('rules.addWhat')}>
      {#each GIT_CLASSES as c (c)}<option value={c}>{ruleLabel({ tool: 'Bash', prefix: c })}</option>{/each}
      <option value="command">{t('rules.aCommand')}</option>
      <option value="tool">{t('rules.aTool')}</option>
    </select>
    {#if needsText}
      <input class="input small" bind:value={text} placeholder={what === 'command' ? 'git push' : 'Write'} aria-label={what === 'command' ? t('rules.commandLabel') : t('rules.toolLabel')} />
    {/if}
    <select class="select small" bind:value={action} aria-label={t('rules.addAction')}>
      {#each ACTIONS as a (a)}<option value={a}>{t(`rules.action.${a}`)}</option>{/each}
    </select>
    <button class="btn small" type="submit" disabled={needsText && !text.trim()}><Icon name="plus" size={13} />{t('rules.add')}</button>
  </form>
  <p class="hint">{t('rules.hint')}</p>

  {#snippet footer()}
    <button class="btn" onclick={() => (app.permissionsOpen = false)}>{t('common.close')}</button>
  {/snippet}
</Modal>

<style>
  .muted {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
    overflow-wrap: anywhere;
  }
  .empty {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-faint);
  }
  .rules {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0 0 var(--space-3);
    padding: 0;
    list-style: none;
  }
  .rules li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 4px var(--space-2);
    border-radius: var(--radius-sm);
  }
  .rules li:hover {
    background: var(--surface-2);
  }
  .what {
    flex: 1;
    min-width: 0;
    font-size: var(--text-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .group {
    margin: 0 0 var(--space-1) var(--space-2);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .tag {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
    padding-top: var(--space-3);
    border-top: 1px solid var(--border);
  }
  .add .input {
    flex: 1;
    min-width: 120px;
    font-family: var(--mono);
  }
  .small {
    height: 28px;
    font-size: var(--text-xs);
  }
  .hint {
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
