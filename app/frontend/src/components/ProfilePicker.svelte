<script lang="ts">
  // New session's session types, as cards: icon, name and what it's for,
  // tinted with the type's hue. One is chosen.
  import type { Profile } from '@uncli/chat/lib/api'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { tintStyle } from '@uncli/chat/lib/tint'
  import Icon from '@uncli/chat/components/Icon.svelte'

  let { profiles, value = $bindable() }: { profiles: Profile[]; value: string } = $props()
</script>

  <fieldset class="profiles">
    <legend class="visually-hidden">{t('newSession.mode')}</legend>
    {#each profiles as p (p.id)}
      <label class="profile" class:selected={p.id === value} style={tintStyle(p.hue)}>
        <input type="radio" name="profile" value={p.id} bind:group={value} class="visually-hidden" data-autofocus={p.id === value ? '' : undefined} />
        <span class="picon"><Icon name={p.icon} size={18} /></span>
        <span class="plabel">{p.label}</span>
        <span class="pdesc">{t(`profile.${p.id}.desc`)}</span>
      </label>
    {/each}
  </fieldset>

<style>
  .profiles {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-2);
    border: 0;
    margin: 0 0 var(--space-4);
    padding: 0;
  }
  @media (max-width: 600px) {
    .profiles {
      grid-template-columns: 1fr;
    }
  }
  .profile {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: border-color var(--fast) var(--ease), background var(--fast) var(--ease);
  }
  .profile:hover {
    border-color: var(--border-strong);
  }
  .profile.selected {
    border-color: var(--tint, var(--accent));
    background: var(--tint-soft, var(--accent-soft));
  }
  .profile:has(input:focus-visible) {
    outline: 2px solid var(--tint, var(--accent));
    outline-offset: 2px;
  }
  .picon {
    color: var(--tint, var(--text-muted));
    margin-bottom: var(--space-1);
  }
  .plabel {
    font-weight: 600;
  }
  .pdesc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.4;
  }
</style>
