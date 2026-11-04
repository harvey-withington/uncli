<script lang="ts">
  import { clickOutside } from '../lib/actions'
  import type { ControlItem } from '../lib/composercontrols'
  import Icon from './Icon.svelte'

  // One of several, as a button showing the current one (icon, label, its
  // tone) that opens a menu of them all: icon, label and hint each. It
  // opens upward, as it sits at the bottom of the message box. Up and Down
  // move, Enter or Space picks, Escape or a click elsewhere closes, and
  // the focus goes back to the button.
  interface Props {
    items: ControlItem[]
    value: string | undefined
    label?: string // what's being chosen ("Mode"), read out with the current one
    onchoose: (id: string) => void
  }

  let { items, value, label, onchoose }: Props = $props()
  const current = $derived(items.find(i => i.id === value) ?? items[0])
  let open = $state(false)
  let active = $state(0)
  let button = $state<HTMLButtonElement>()
  let menu = $state<HTMLElement>()
  const name = $derived(label ? `${label}: ${current?.label ?? ''}` : (current?.label ?? ''))

  function show() {
    open = true
    active = Math.max(0, items.findIndex(i => i.id === current?.id))
    queueMicrotask(() => (menu?.querySelectorAll<HTMLElement>('[role=menuitemradio]')[active])?.focus())
  }

  function close(refocus = true) {
    open = false
    if (refocus) button?.focus()
  }

  function pick(id: string) {
    close()
    if (id !== value) onchoose(id)
  }

  function onkeydown(e: KeyboardEvent) {
    const n = items.length
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      active = (active + (e.key === 'ArrowDown' ? 1 : -1) + n) % n
      menu?.querySelectorAll<HTMLElement>('[role=menuitemradio]')[active]?.focus()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      close()
    } else if (e.key === 'Tab') {
      close(false)
    }
  }
</script>

<span class="wrap" use:clickOutside={() => open && close(false)}>
  <button
    bind:this={button}
    class="ctl {current?.tone ?? ''}"
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={name}
    title={[name, current?.hint].filter(Boolean).join('\n')}
    onclick={() => (open ? close() : show())}
  >
    {#if current?.icon}<Icon name={current.icon} size={14} />{/if}<span class="label">{current?.label}</span><Icon name="chevron-down" size={12} />
  </button>
  {#if open}
    <div class="menu" role="menu" aria-label={label} bind:this={menu} tabindex="-1" {onkeydown}>
      {#each items as item, i (item.id)}
        <button role="menuitemradio" aria-checked={item.id === current?.id} class="item {item.tone ?? ''}" tabindex={i === active ? 0 : -1} onclick={() => pick(item.id)}>
          {#if item.icon}<Icon name={item.icon} size={15} />{/if}
          <span class="text"><span class="name">{item.label}</span>{#if item.hint}<span class="hint">{item.hint}</span>{/if}</span>
          {#if item.id === current?.id}<Icon name="check" size={14} />{/if}
        </button>
      {/each}
    </div>
  {/if}
</span>

<style>
  .wrap {
    position: relative;
    display: inline-flex;
  }
  .menu {
    position: absolute;
    bottom: calc(100% + 6px);
    left: 0;
    z-index: 30;
    min-width: 220px;
    max-width: min(320px, 80vw);
    padding: var(--space-1);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-md);
  }
  .item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .item:hover,
  .item:focus-visible {
    background: var(--surface-2);
    outline: none;
  }
  .item.success { color: var(--success); }
  .item.warning { color: var(--warning); }
  .item.danger { color: var(--danger); }
  .item.accent { color: var(--accent); }
  .item.muted { color: var(--text-muted); }
  .text {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }
  .name {
    font-size: var(--text-sm);
    font-weight: 560;
  }
  .hint {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
</style>
