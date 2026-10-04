<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { modeOf } from '../lib/approvals'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // One line under the session's folder for each way the session departs
  // from the usual: when Claude prompts, and whether the user is away.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const mode = $derived(modeOf(session))
  const lines = $derived(
    [
      mode === 'always' && { icon: 'hand', text: t('mode.always.on'), tone: 'info' },
      mode === 'never' && { icon: 'zap', text: t('mode.never.on'), tone: 'warn' },
      session.unattended && { icon: 'coffee', text: t('unattended.on'), tone: 'warn' },
    ].filter(l => !!l),
  )
</script>

{#each lines as l (l.icon)}
  <p class="status {l.tone}" role="status"><Icon name={l.icon} size={14} />{l.text}</p>
{/each}

<style>
  .status {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0 0 0 38px;
    font-size: var(--text-xs);
  }
  .info {
    color: var(--accent);
  }
  .warn {
    color: var(--warning);
  }
</style>
