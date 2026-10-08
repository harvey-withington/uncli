<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { modeOf } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // Lines under the session's folder: when the session's CLI prompts (every level has
  // one, the default in the quietest tone), whether the user is away, and
  // what runs in the background.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const mode = $derived(modeOf(session))
  // Sub-agents or shells still running after the turn that started them;
  // when one finishes, Claude carries on by itself on a page of its own.
  const background = $derived(session.background ?? [])
  const backgroundLine = $derived(
    background.length === 1
      ? t('background.one', { ...names, what: background[0]?.description || t('background.task') })
      : t('background.many', { ...names, n: background.length }),
  )
  const lines = $derived(
    [
      mode === 'always' && { icon: 'hand', text: t('mode.always.on', names), tone: 'info' },
      mode === 'unsafe' && { icon: 'shield', text: t('mode.unsafe.on', names), tone: 'muted' },
      mode === 'never' && { icon: 'zap', text: t('mode.never.on', names), tone: 'warn' },
      session.unattended && { icon: 'coffee', text: t('unattended.on', names), tone: 'warn' },
      background.length > 0 && { icon: 'loader', text: backgroundLine, tone: 'info', spin: true },
      // The CLI stopped (it exited, or its container was rebuilt): nothing is
      // lost, and the next message starts it again.
      session.state === 'exited' && !session.archived && { icon: 'play', text: t('session.stoppedHint', names), tone: 'muted' },
    ].filter(l => !!l),
  )
</script>

{#each lines as l (l.icon)}
  <p class="status {l.tone}" role="status"><Icon name={l.icon} size={14} spin={'spin' in l && !!l.spin} />{l.text}</p>
{/each}

<style>
  .status {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0 0 0 38px;
    font-size: var(--text-xs);
  }
  .muted {
    color: var(--text-muted);
  }
  .info {
    color: var(--accent);
  }
  .warn {
    color: var(--warning);
  }
</style>
