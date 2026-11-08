<script lang="ts">
  // New session → Where (decision 0031): this PC, or a paired server. A
  // server that isn't connected is listed, not chosen, with why. Without
  // paired servers it says where to add one.
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { LOCAL } from '@uncli/chat/lib/api'
  import { peerStatus } from '@uncli/chat/lib/servers'
  import Icon from '@uncli/chat/components/Icon.svelte'

  interface Props {
    where: string
    onchoose: (key: string) => void
    loading?: boolean // the server's profiles and providers are on their way
    error?: string // they couldn't be loaded
  }

  let { where, onchoose, loading = false, error = '' }: Props = $props()
  const app = useApp()
</script>

{#if app.backend.kind !== 'http'}
  <div class="field">
    <label class="row">
      <span class="flabel">{t('newSession.where')}</span>
      <select class="select" value={where} onchange={e => onchoose(e.currentTarget.value)} disabled={app.servers.length === 0}>
        <option value={LOCAL}>{t('peers.thisPC')}</option>
        {#each app.servers as p (p.key)}
          <option value={p.key} disabled={p.state !== 'online'}>{p.state === 'online' ? p.name : `${p.name}: ${peerStatus(p)}`}</option>
        {/each}
      </select>
      {#if loading && !error}<Icon name="loader" spin size={14} />{/if}
    </label>
    {#if app.servers.length === 0}
      <p class="hint">
        {t('newSession.whereNone')}
        <button class="link" onclick={() => { app.newSessionOpen = false; app.openSettings('peers') }}>{t('peers.openSettings')}</button>
      </p>
    {/if}
    {#if error}<p class="hint danger" role="alert">{error}</p>{/if}
  </div>
{/if}

<style>
  .field {
    margin: 0 0 var(--space-4);
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .flabel {
    font-size: var(--text-sm);
    font-weight: 600;
  }
  .select {
    flex: 1;
  }
  .hint {
    margin: var(--space-1) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .hint.danger {
    color: var(--danger);
  }
  .link {
    border: 0;
    padding: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
</style>
