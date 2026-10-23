<script lang="ts" module>
  // A choice of a few, as one button that shows the current one and moves
  // to the next on each click (Shift+click: the one before), wrapping
  // round: the prompt level, a group of modifiers. A choice's tone (success,
  // warning, accent) says what it risks or costs: the button wears it.
  export interface Choice {
    id: string
    label: string
    icon?: string
    tone?: '' | 'success' | 'warning' | 'accent'
    hint?: string // what it does, in the tooltip
  }

  // step is the choice after (or before) the current one, wrapping round.
  export function step(choices: Choice[], value: string, by: 1 | -1): Choice | undefined {
    const i = Math.max(0, choices.findIndex(c => c.id === value))
    return choices[(i + by + choices.length) % choices.length]
  }
</script>

<script lang="ts">
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  interface Props {
    choices: Choice[]
    value: string
    label: string // what is being chosen ("Prompt me"), read out and in the tooltip
    showLabel?: boolean // shown before the choice too
    onchoose: (id: string) => void
  }

  let { choices, value, label, showLabel = true, onchoose }: Props = $props()
  const current = $derived(choices.find(c => c.id === value) ?? choices[0])
  const next = $derived(step(choices, value, 1))
  const prev = $derived(step(choices, value, -1))
  const title = $derived(
    [
      `${label}: ${current?.label ?? ''}`,
      current?.hint ?? '',
      t('cycle.next', { next: next?.label ?? '', prev: prev?.label ?? '' }),
    ].filter(Boolean).join('\n'),
  )

  function onclick(e: MouseEvent) {
    const to = e.shiftKey ? prev : next
    if (to && to.id !== value) onchoose(to.id)
  }
</script>

<button class="cycle {current?.tone ?? ''}" {onclick} aria-label={`${label}: ${current?.label ?? ''}`} {title}>
  {#if showLabel}<span class="what">{label}:</span>{/if}
  <!-- Every choice is laid out in the same place, the others hidden, so the
       button is as wide as its widest choice and stays put under the
       pointer as it cycles. -->
  <span class="stack" aria-hidden="true">
    {#each choices as c (c.id)}
      <span class="choice" class:shown={c.id === current?.id}>{#if c.icon}<Icon name={c.icon} size={13} />{/if}{c.label}</span>
    {/each}
  </span>
</button>
<span class="visually-hidden" aria-live="polite">{current?.label}</span>

<style>
  .cycle {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 26px;
    padding: 0 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--text-xs);
    font-weight: 560;
    white-space: nowrap;
    cursor: pointer;
    transition: background var(--fast) var(--ease), color var(--fast) var(--ease), border-color var(--fast) var(--ease);
  }
  .cycle:hover {
    border-color: var(--text-faint);
  }
  .stack {
    display: inline-grid;
  }
  .choice {
    grid-area: 1 / 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    visibility: hidden;
  }
  .choice.shown {
    visibility: visible;
  }
  .what {
    color: var(--text-muted);
    font-weight: 500;
  }
  .success {
    border-color: color-mix(in srgb, var(--success) 35%, var(--border));
    background: var(--success-soft);
    color: var(--success);
  }
  .warning {
    border-color: color-mix(in srgb, var(--warning) 40%, var(--border));
    background: var(--warning-soft);
    color: var(--warning);
  }
  .accent {
    border-color: color-mix(in srgb, var(--accent) 35%, var(--border));
    background: var(--accent-soft);
    color: var(--accent);
  }
  .success .what,
  .warning .what,
  .accent .what {
    color: inherit;
    opacity: 0.75;
  }
  .cycle:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
