<script lang="ts">
  import { cleanLocal, localProblems } from '../lib/containers'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'

  // The container editor's own copies: paths in a session's folder, such
  // as node_modules, that the container keeps on its own disk (decision
  // 0032), so its installs and Windows' don't overwrite each other.
  interface Props {
    paths: string[]
    uid: string
  }

  let { paths = $bindable(), uid }: Props = $props()
  const whys = $derived(localProblems(paths))
</script>

<div class="local">
  <span class="hint" id="{uid}-local-note">{t('containerEdit.localNote')}</span>
  {#each paths as p, i (i)}
    {@const why = whys[i]}
    <div class="row">
      <input
        class="input mono"
        bind:value={paths[i]}
        placeholder={i ? 'apps/web/node_modules' : 'node_modules'}
        aria-label={t('containerEdit.localPath', { n: i + 1 })}
        aria-invalid={!!why}
        aria-describedby="{uid}-local-note"
        spellcheck="false"
      />
      <button type="button" class="btn small ghost icon" aria-label={t('containerEdit.removeLocal', { n: i + 1 })} title={t('containerEdit.removeLocal', { n: i + 1 })} onclick={() => (paths = paths.filter((_, j) => j !== i))}>
        <Icon name="trash" size={14} />
      </button>
    </div>
    {#if why || p.trim()}
      <span class="hint" class:err={!!why}>{why ? t(why) : t('containerEdit.localKept', { path: cleanLocal(p) })}</span>
    {/if}
  {/each}
  <button type="button" class="btn small" onclick={() => (paths = [...paths, ''])}>
    <Icon name="plus" size={13} />{t('containerEdit.addLocal')}
  </button>
</div>

<style>
  .local {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-1);
    padding-top: 6px;
  }
  .row {
    display: flex;
    gap: var(--space-1);
    align-self: stretch;
  }
  .row .input {
    flex: 1;
    width: 100%;
    min-width: 0;
  }
  .mono {
    font-family: var(--mono);
    font-size: var(--text-xs);
  }
  .hint {
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .hint.err {
    color: var(--danger);
  }
  .local > .btn {
    margin-top: var(--space-1);
  }
</style>
