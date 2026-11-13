<script lang="ts">
  import { LOCAL, type SuggestedContainer } from '@uncli/chat/lib/api'
  import { atServer, onlyAtServer } from '@uncli/chat/lib/remoteonly'
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import Modal from '@uncli/chat/components/Modal.svelte'
  import { useShell } from '@uncli/chat/lib/shell.svelte'
  import { ServerPlace, thisPC, type Place } from '../stores/place.svelte'
  import FolderBrowser from './FolderBrowser.svelte'
  import ImportContainerDialog from './ImportContainerDialog.svelte'
  import SuggestedContainers from './SuggestedContainers.svelte'
  import WhereField from './WhereField.svelte'

  // Settings → Containers → Import from a project… (decisions 0028, 0033):
  // where (this PC or a paired server, as in New session), a project's
  // folder there, then one of the containers its repository suggests,
  // imported or synced on that PC as the new session dialog does
  // (ImportContainerDialog). Without paired servers it starts at the
  // folder. Setup stays at a PC: the phone's screens don't offer it.
  const app = useApp()
  const shell = useShell()
  const here = $derived(atServer(app.backend))
  const local = thisPC(app)
  const remote: Record<string, ServerPlace> = {}
  let where = $state(LOCAL)
  const place = $derived<Place>(where === LOCAL ? local : remote[where]!)
  let open = $state(false)
  let folder = $state('')
  let browsing = $state(false)
  let importing = $state<{ s: SuggestedContainer; repo: string; name: string; place: Place } | null>(null)
  let imported = $state(0)

  function choose(key: string) {
    if (key !== LOCAL && !remote[key]) {
      remote[key] = new ServerPlace(key, app.backend.peer(key))
      void remote[key].load()
    }
    where = key
    folder = '' // a folder is one PC's
  }

  // start opens the dialog: on Where when there are paired servers, else
  // straight at this PC's folder dialog.
  function start() {
    where = LOCAL
    folder = ''
    if (app.servers.length) open = true
    else void pick()
  }

  async function pick() {
    if (where !== LOCAL) {
      browsing = true
      return
    }
    try {
      const dir = await app.backend.pickFolder(t('suggested.pickProject'))
      if (dir) {
        folder = dir
        open = true
      }
    } catch (e) {
      shell.toast(String(e), 'error')
    }
  }
</script>

<button class="btn small" onclick={start} disabled={!here} title={here ? undefined : onlyAtServer()}>
  <Icon name="download" size={13} />{t('suggested.fromProject')}
</button>

{#if open}
  <Modal title={t('suggested.fromProjectTitle')} width={560} onclose={() => (open = false)}>
    {#if app.servers.length}
      <WhereField {where} onchoose={choose} error={where === LOCAL ? '' : remote[where]?.error ?? ''} loading={!place.boot} />
    {/if}
    <div class="field">
      <span class="flabel">{t('suggested.project')}</span>
      <div class="folder">
        <span class="path" class:placeholder={!folder} title={folder}>{folder || t('suggested.noProject')}</span>
        <button class="btn" onclick={pick} disabled={!place.boot}><Icon name="folder-open" size={14} />{t('newSession.choose')}</button>
      </div>
    </div>
    {#if folder && place.boot}
      {#if place.boot.platform === 'windows'}
        {#key where}
          <SuggestedContainers {folder} {place} refresh={imported} standalone onimport={(s, repo, name) => (importing = { s, repo, name, place })} />
        {/key}
      {:else}
        <p class="hint">{t('suggested.noContainersThere')}</p>
      {/if}
    {/if}
    {#snippet footer()}
      <button class="btn" onclick={() => (open = false)}>{t('common.close')}</button>
    {/snippet}
  </Modal>
{/if}
{#if browsing}
  <FolderBrowser backend={place.backend} title={t('suggested.pickProject')} start={folder} onpick={dir => { browsing = false; folder = dir }} onclose={() => (browsing = false)} />
{/if}
{#if importing}
  <ImportContainerDialog suggestion={importing.s} repo={importing.repo} name={importing.name} place={importing.place} onclose={() => { importing = null; imported++ }} />
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
  .placeholder {
    color: var(--text-faint);
    font-family: var(--font);
  }
  .hint {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
</style>
