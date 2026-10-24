<script lang="ts">
  import { onMount } from 'svelte'
  import type { FolderListing } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'

  // Picks a folder on the server by browsing it (Backend.listFolder), for
  // clients that can't open the server's folder dialog: a remote client
  // starting a session there (decision 0021). Folders only; the top lists
  // the home folder and the drives.

  interface Props {
    title: string
    start?: string // a folder to open at; '' for the top
    onpick: (path: string) => void
    onclose: () => void
  }

  let { title, start = '', onpick, onclose }: Props = $props()
  const app = useApp()
  let listing = $state<FolderListing | null>(null)
  let loading = $state(false)

  async function open(path: string) {
    loading = true
    try {
      listing = await app.backend.listFolder(path)
    } catch (e) {
      // A folder that's gone (or can't be read) falls back to the top.
      if (path !== '') return open('')
      showToast(String(e), 'error')
    } finally {
      loading = false
    }
  }

  onMount(() => void open(start))
</script>

<Modal {title} width={480} {onclose}>
  <div class="where">
    <button class="btn ghost small icon" onclick={() => open(listing?.parent ?? '')} disabled={!listing?.path || loading} aria-label={t('folders.up')} title={t('folders.up')}>
      <Icon name="arrow-up" size={16} />
    </button>
    <span class="path" title={listing?.path}>{listing?.path || t('folders.top')}</span>
    {#if loading}<Icon name="loader" spin size={14} label={t('app.loading')} />{/if}
  </div>
  {#if listing}
    {#if listing.folders.length === 0}
      <p class="empty">{t('folders.none')}</p>
    {:else}
      <ul aria-label={t('folders.list')}>
        {#each listing.folders as f (f.path)}
          <li><button class="row" onclick={() => open(f.path)}><Icon name="folder" size={16} /><span>{f.name}</span><Icon name="chevron-right" size={14} /></button></li>
        {/each}
      </ul>
    {/if}
  {/if}
  {#snippet footer()}
    <button class="btn" onclick={onclose}>{t('common.cancel')}</button>
    <button class="btn primary" onclick={() => listing?.path && onpick(listing.path)} disabled={!listing?.path}>{t('folders.choose')}</button>
  {/snippet}
</Modal>

<style>
  .where {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
  }
  .where .btn {
    min-width: 44px;
    min-height: 44px;
  }
  .path {
    flex: 1;
    min-width: 0;
    font-family: var(--mono);
    font-size: var(--text-sm);
    overflow-wrap: anywhere;
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: min(50vh, 420px);
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }
  li + li {
    border-top: 1px solid var(--border);
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    min-height: 44px;
    padding: 0 var(--space-3);
    border: none;
    background: none;
    color: var(--text);
    font-size: var(--text-sm);
    text-align: left;
  }
  .row span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .row:hover {
    background: var(--surface-2);
  }
  .empty {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }
</style>
