<script lang="ts">
  import { confirmState, settleConfirm } from '../lib/confirm.svelte'
  import { t } from '../lib/i18n.svelte'
  import Modal from './Modal.svelte'

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      settleConfirm(true)
    }
  }
</script>

{#if confirmState.request}
  {@const r = confirmState.request}
  <Modal title={r.title} width={420} onclose={() => settleConfirm(false)}>
    <p>{r.message}</p>
    {#snippet footer()}
      <button class="btn" onclick={() => settleConfirm(false)}>{t('common.cancel')}</button>
      <button class="btn primary" class:danger-fill={r.danger} onclick={() => settleConfirm(true)} data-autofocus {onkeydown}>{r.confirmLabel}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  p {
    margin: 0;
    color: var(--text-muted);
  }
  .danger-fill {
    background: var(--danger);
    border-color: var(--danger);
  }
  .danger-fill:hover {
    filter: brightness(1.08);
    background: var(--danger);
    border-color: var(--danger);
  }
</style>
