<script lang="ts">
  import { useChat } from '../lib/chatcontext'
  import { buttonOf, kindOf } from '../lib/composercontrols'
  import ChoiceMenu from './ChoiceMenu.svelte'
  import CycleButton from './CycleButton.svelte'
  import Icon from './Icon.svelte'

  // A host's own controls at the bottom left of the message box (decision
  // 0026, lib/composercontrols.ts): buttons, toggles and drop-downs drawn
  // here, then whatever the host puts in the composer-controls slot. When
  // the box is narrow, buttons and drop-downs drop their labels to their
  // icons (still the name read out and the tooltip).
  const chat = useChat()
</script>

{#if chat.composerControls.length || chat.composerSlot}
  <div class="controls">
    {#each chat.composerControls as c (c.id)}
      {@const kind = kindOf(c)}
      {#if kind === 'toggle' && 'states' in c}
        <CycleButton choices={c.states.map(s => ({ ...s, tone: s.tone ?? '' }))} value={c.value} label={c.label ?? ''} showLabel={false} onchoose={id => c.onChange(id)} />
      {:else if kind === 'menu' && 'options' in c}
        <ChoiceMenu items={c.options} value={c.value} label={c.label} onchoose={id => c.onChange(id)} />
      {:else}
        {@const b = buttonOf(c)}
        <button class="ctl {b.item.tone ?? ''}" onclick={b.click} disabled={b.disabled} aria-label={b.item.label} title={[b.item.label, b.item.hint].filter(Boolean).join('\n')}>
          {#if b.item.icon}<Icon name={b.item.icon} size={14} />{/if}<span class="label">{b.item.label}</span>
        </button>
      {/if}
    {/each}
    {#if chat.composerSlot}<svelte:element this={'slot'} name="composer-controls" />{/if}
  </div>
{/if}

<style>
  .controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    padding: 0 var(--space-3) var(--space-2);
  }
  /* A narrow box: icons only. */
  @container composer (max-width: 240px) {
    .controls :global(.label) {
      display: none;
    }
  }
</style>
