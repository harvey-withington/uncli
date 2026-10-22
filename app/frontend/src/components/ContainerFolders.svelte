<script lang="ts">
  import type { ContainerFolder } from '../lib/api'
  import { folderNameFor, folderProblem } from '../lib/containers'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // The container editor's Folders: more Windows folders the container
  // mounts at /home/uncli/<name> (decision 0017), read-only unless set to
  // read and write. A name follows the folder until it's typed.
  interface Props {
    folders: ContainerFolder[]
    uid: string
  }

  let { folders = $bindable(), uid }: Props = $props()
  const named = new Set<number>() // rows whose name was typed

  function setSource(i: number, source: string) {
    const f = folders[i]
    if (!f) return
    f.source = source
    if (!named.has(i)) f.name = folderNameFor(source)
  }

  function remove(i: number) {
    folders = folders.filter((_, j) => j !== i)
    named.clear()
  }
</script>

<div class="folders">
  <span class="hint">{t('containerEdit.foldersNote')}</span>
  {#each folders as f, i (i)}
    {@const why = (f.source.trim() || f.name.trim()) && folderProblem(f)}
    <div class="row">
      <input
        class="input mono src"
        value={f.source}
        oninput={e => setSource(i, e.currentTarget.value)}
        placeholder="%USERPROFILE%\notes"
        aria-label={t('containerEdit.folderSource', { n: i + 1 })}
        aria-invalid={!!why}
        spellcheck="false"
      />
      <input
        class="input mono name"
        bind:value={f.name}
        oninput={() => named.add(i)}
        placeholder="notes"
        aria-label={t('containerEdit.folderName', { n: i + 1 })}
        spellcheck="false"
      />
      <select class="select access" bind:value={f.access} aria-label={t('containerEdit.folderAccess', { n: i + 1 })}>
        <option value="ro">{t('containerEdit.readOnly')}</option>
        <option value="rw">{t('containerEdit.readWrite')}</option>
      </select>
      <button type="button" class="btn small ghost icon" aria-label={t('containerEdit.removeFolder', { n: i + 1 })} title={t('containerEdit.removeFolder', { n: i + 1 })} onclick={() => remove(i)}>
        <Icon name="trash" size={14} />
      </button>
    </div>
    <span class="hint" class:err={!!why}>
      {why ? t(why) : f.name.trim() ? t('containerEdit.folderAt', { path: `/home/uncli/${f.name.trim()}` }) : ''}
    </span>
  {/each}
  <button type="button" class="btn small" onclick={() => (folders = [...folders, { source: '', name: '', access: 'ro' }])}>
    <Icon name="plus" size={13} />{t('containerEdit.addFolder')}
  </button>
</div>

<style>
  .folders {
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
  .src {
    flex: 3;
    width: 100%;
    min-width: 0;
  }
  .name {
    flex: 1;
    width: 100%;
    min-width: 0;
  }
  .access {
    flex: none;
    width: auto;
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
  .folders > .btn {
    margin-top: var(--space-1);
  }
</style>
