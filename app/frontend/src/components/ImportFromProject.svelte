<script lang="ts">
  import type { SuggestedContainer } from '@uncli/chat/lib/api'
  import { atServer, onlyAtServer } from '@uncli/chat/lib/remoteonly'
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import Modal from '@uncli/chat/components/Modal.svelte'
  import { useShell } from '@uncli/chat/lib/shell.svelte'
  import ImportContainerDialog from './ImportContainerDialog.svelte'
  import SuggestedContainers from './SuggestedContainers.svelte'

  // Settings → Containers → Import from a project… (decision 0028): pick a
  // project's folder, then one of the containers its repository suggests,
  // and import or sync it as the new session dialog does
  // (ImportContainerDialog). The folder dialog is the PC's own, so this is
  // the desktop's only.
  const app = useApp()
  const shell = useShell()
  const here = $derived(atServer(app.backend))
  let folder = $state('')
  let importing = $state<{ s: SuggestedContainer; repo: string; name: string } | null>(null)
  let imported = $state(0)

  async function pick() {
    try {
      const dir = await app.backend.pickFolder(t('suggested.pickProject'))
      if (dir) folder = dir
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

<button class="btn small" onclick={pick} disabled={!here} title={here ? undefined : onlyAtServer()}>
  <Icon name="download" size={13} />{t('suggested.fromProject')}
</button>

{#if folder}
  <Modal title={t('suggested.fromProjectTitle')} width={560} onclose={() => (folder = '')}>
    <div class="field">
      <span class="flabel">{t('suggested.project')}</span>
      <div class="folder">
        <span class="path" title={folder}>{folder}</span>
        <button class="btn" onclick={pick}><Icon name="folder-open" size={14} />{t('newSession.choose')}</button>
      </div>
    </div>
    <SuggestedContainers {folder} refresh={imported} standalone onimport={(s, repo, name) => (importing = { s, repo, name })} />
    {#snippet footer()}
      <button class="btn" onclick={() => (folder = '')}>{t('common.close')}</button>
    {/snippet}
  </Modal>
{/if}
{#if importing}
  <ImportContainerDialog suggestion={importing.s} repo={importing.repo} name={importing.name} onclose={() => { importing = null; imported++ }} />
{/if}

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: var(--space-4);
  }
  .flabel {
    font-size: var(--text-sm);
    font-weight: 500;
  }
  .folder {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .path {
    flex: 1;
    min-width: 0;
    height: 32px;
    line-height: 30px;
    padding: 0 var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface-2);
    font-family: var(--mono);
    font-size: var(--text-xs);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
