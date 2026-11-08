<script lang="ts">
  // A group heading in the sidebar's session list: This PC, or a paired
  // server with its status (children) after the name.
  import type { Snippet } from 'svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'

  interface Props {
    icon: string
    label: string
    title?: string
    children?: Snippet
  }

  let { icon, label, title, children }: Props = $props()
</script>

<h3 {title}>
  <Icon name={icon} size={12} />
  <span class="label">{label}</span>
  {@render children?.()}
</h3>

<style>
  h3 {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 0 var(--space-1);
    padding: 0 var(--space-2);
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-faint);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
