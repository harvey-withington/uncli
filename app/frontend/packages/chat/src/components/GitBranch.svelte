<script lang="ts">
  // The git branch a session's folder is on (or the commit, when detached),
  // after its folder in the header: which branch the agent works on, without
  // opening an editor. Nothing when the folder isn't in a repository.
  import type { GitHead } from '../lib/api'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  let { git }: { git?: GitHead } = $props()
  const label = $derived(git ? (git.branch || t('git.detached', { commit: git.commit.slice(0, 7) })) : '')
</script>

{#if git && label}
  <span class="branch" title={t('git.title', { branch: label })}>
    <Icon name="git-branch" size={12} />
    <span class="name">{label}</span>
  </span>
{/if}

<style>
  .branch {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    flex: none;
    max-width: 40%;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--mono);
  }
</style>
