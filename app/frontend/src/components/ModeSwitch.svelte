<script lang="ts">
  import type { SessionMode, SessionView } from '../lib/api'
  import { MODES, modeOf } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import CycleButton from './CycleButton.svelte'

  // When the CLI stops to prompt the user (Always / When unsafe / Never),
  // applied by the backend to the next tool use and to any already waiting.
  // One button that cycles through them, coloured by risk: Always (the
  // safest) green, Never a warning.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const mode = $derived(modeOf(session))
  const TONE = { always: 'success', unsafe: '', never: 'warning' } as const
  const choices = $derived(MODES.map(m => ({ id: m.id, label: t(`mode.${m.id}`), icon: m.icon, tone: TONE[m.id], hint: t(`mode.${m.id}.hint`, names) })))

  async function choose(m: string) {
    try {
      app.upsertSession(await app.backend.setMode(session.id, m as SessionMode))
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

<CycleButton {choices} value={mode} label={t('mode.short')} onchoose={choose} />
