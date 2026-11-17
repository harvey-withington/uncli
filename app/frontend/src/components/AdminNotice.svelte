<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'

  // UNCLI running with administrator rights ("Run as administrator", or
  // started by an installer that was), which it never needs: everything it
  // starts gets them too, an update's task is set up as the administrator,
  // and run as another account it shows that account's sessions. Above the
  // sidebar's footer on the PC and atop the session list on the phone
  // (remote: it's the PC's UNCLI). Dismissed until UNCLI starts again.
  interface Props {
    remote?: boolean
  }

  let { remote = false }: Props = $props()
  const app = useApp()
  let dismissed = $state(false)
</script>

{#if app.boot?.elevated && !dismissed}
  <div class="admin" role="status">
    <p class="title"><Icon name="triangle-alert" size={14} /><span>{t(remote ? 'admin.titleRemote' : 'admin.title')}</span></p>
    <p class="body">{t('admin.body')}</p>
    <div class="acts">
      <button class="btn ghost small" onclick={() => (dismissed = true)}>{t('admin.dismiss')}</button>
    </div>
  </div>
{/if}

<style>
  .admin {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--warning);
    border-radius: var(--radius);
    background: var(--surface-2);
    font-size: var(--text-sm);
    margin-bottom: var(--space-3);
  }
  p {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .title {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    font-weight: 600;
  }
  .title :global(svg) {
    flex: none;
    margin-top: 2px;
    color: var(--warning);
  }
  .body {
    color: var(--text-muted);
  }
  .acts {
    display: flex;
    gap: var(--space-1);
  }
</style>
