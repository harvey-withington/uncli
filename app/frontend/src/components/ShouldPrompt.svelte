<script lang="ts">
  import type { SafeClass, Scope } from '../lib/api'
  import { classLabel, loadScope, saveScope } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // On a part of a trace row that ran because UNCLI judged it safe: "This
  // should prompt" marks that part's class unsafe on the safe list, for this
  // project or all projects (the scope the user last used), so learning
  // goes both ways.
  interface Props {
    sessionId: string
    classes: SafeClass[]
  }

  let { sessionId, classes }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(app.sessions.find(s => s.id === sessionId)?.adapter))
  let scope = $state<Scope>(loadScope())
  let busy = $state(false)
  let done = $state(false)

  async function teach() {
    if (busy) return
    busy = true
    try {
      saveScope(scope)
      await app.backend.teach(sessionId, classes, 'unsafe', scope)
      done = true
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
    }
  }
</script>

<div class="teach">
  {#if done}
    <span class="done"><Icon name="check" size={12} />{t('teach.done', { ...names, what: classes.map(classLabel).join(', ') })}</span>
  {:else}
    <button class="btn ghost small" onclick={teach} disabled={busy} title={t('teach.hint', { ...names, what: classes.map(classLabel).join(', ') })}>
      <Icon name="hand" size={12} />{t('teach.prompt')}
    </button>
    <select class="scope" bind:value={scope} aria-label={t('scope.label')} disabled={busy}>
      <option value="project">{t('scope.project')}</option>
      <option value="all">{t('scope.all')}</option>
    </select>
  {/if}
</div>

<style>
  .teach {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin-left: auto;
    font-family: var(--font);
  }
  .teach .btn {
    gap: 4px;
    height: 22px;
  }
  .scope {
    height: 22px;
    font-family: var(--font);
    padding: 0 4px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font-size: var(--text-xs);
  }
  .done {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--text-muted);
  }
</style>
