<script lang="ts">
  import type { SessionMode, SessionView } from '../lib/api'
  import { MODES, modeOf } from '../lib/approvals'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // When Claude stops to prompt the user (Always / When unsafe / Never),
  // applied by the backend to the next tool use and to any already waiting.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const mode = $derived(modeOf(session))
  let group: HTMLElement | undefined = $state()

  async function choose(m: SessionMode) {
    if (m === mode) return
    try {
      app.upsertSession(await app.backend.setMode(session.id, m))
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  // Arrow keys move the choice, as in any radio group.
  function onkeydown(e: KeyboardEvent) {
    const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const i = MODES.findIndex(m => m.id === mode)
    const next = MODES[(i + step + MODES.length) % MODES.length]
    if (!next) return
    void choose(next.id)
    group?.querySelector<HTMLElement>(`[data-mode="${next.id}"]`)?.focus()
  }
</script>

<span class="label" id="mode-label-{session.id}">{t('mode.label')}</span>
<div class="modes" role="radiogroup" aria-labelledby="mode-label-{session.id}" bind:this={group} tabindex="-1" {onkeydown}>
  {#each MODES as m (m.id)}
    <button
      class="seg {m.id}"
      class:on={mode === m.id}
      role="radio"
      aria-checked={mode === m.id}
      tabindex={mode === m.id ? 0 : -1}
      data-mode={m.id}
      title={t(`mode.${m.id}.hint`, names)}
      onclick={() => choose(m.id)}
    >
      <Icon name={m.icon} size={14} />{t(`mode.${m.id}`)}
    </button>
  {/each}
</div>

<style>
  .label {
    flex: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    white-space: nowrap;
  }
  .modes {
    display: inline-flex;
    flex: none;
    padding: 2px;
    gap: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-2);
  }
  .seg {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 24px;
    padding: 0 9px;
    border: 0;
    border-radius: calc(var(--radius) - 2px);
    background: transparent;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--text-xs);
    font-weight: 560;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .seg:hover {
    color: var(--text);
  }
  .seg.on {
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-sm);
  }
  .seg.always.on {
    color: var(--accent);
  }
  .seg.never.on {
    background: var(--warning-soft);
    color: var(--warning);
  }
</style>
