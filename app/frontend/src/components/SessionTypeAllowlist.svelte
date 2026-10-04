<script lang="ts">
  import { untrack } from 'svelte'
  import { allowLabel } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'

  // What the session's type allows without a rule of the user's
  // (allowed_tools in profiles.yaml). UNCLI applies it in Ask mode, after
  // the rules, so it was invisible until it showed up in a trace. Shown
  // read-only: it belongs to the session type, not the project.
  interface Props {
    sessionId: string
    typeLabel: string
  }

  let { sessionId, typeLabel }: Props = $props()
  const app = useApp()
  let entries = $state<string[]>([])

  $effect(() => {
    const id = sessionId
    untrack(() => {
      app.backend.sessionAllowlist(id).then(
        list => (entries = list),
        e => showToast(String(e), 'error'),
      )
    })
  })
</script>

{#if entries.length > 0}
  <h3 class="group">{t('rules.sessionType', { type: typeLabel })}</h3>
  <ul class="entries" aria-label={t('rules.sessionType', { type: typeLabel })}>
    {#each entries as e (e)}
      <li title={e}>{allowLabel(e)}</li>
    {/each}
  </ul>
  <p class="note">{t('rules.sessionTypeNote')}</p>
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
  .entries {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin: 0 0 var(--space-1);
    padding: 0 var(--space-2);
    list-style: none;
  }
  .entries li {
    padding: 2px 8px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .note {
    margin: 0 0 var(--space-3) var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
