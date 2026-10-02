<script lang="ts">
  import { fly } from 'svelte/transition'
  import { t } from '../lib/i18n.svelte'
  import { dismissToast, toasts } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
</script>

<div class="toasts" aria-live="polite">
  {#each toasts.list as toast (toast.id)}
    <div class="toast {toast.kind}" role={toast.kind === 'error' ? 'alert' : 'status'} transition:fly={{ y: -8, duration: 160 }}>
      {#if toast.kind === 'error'}<Icon name="circle-alert" />{/if}
      <span>{toast.message}</span>
      <button class="btn ghost small icon" onclick={() => dismissToast(toast.id)} aria-label={t('common.dismiss')}><Icon name="x" size={14} /></button>
    </div>
  {/each}
</div>

<style>
  /* Top right: the bottom of the window is the message box and its Send
     button, which a note must never cover. */
  .toasts {
    position: fixed;
    right: var(--space-5);
    top: var(--space-5);
    z-index: 60;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-width: 420px;
  }
  .toast {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-2) var(--space-3) var(--space-4);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    font-size: var(--text-sm);
  }
  .toast span {
    flex: 1;
    padding-top: 3px;
  }
  .error {
    border-color: var(--danger);
    color: var(--danger);
  }
  .error span {
    color: var(--text);
  }
</style>
