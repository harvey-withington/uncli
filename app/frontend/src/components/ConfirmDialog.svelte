<script lang="ts">
  import { t } from '../lib/i18n.svelte'
  import { useShell } from '../lib/shell.svelte'
  import Modal from './Modal.svelte'

  // This root's confirm dialog (lib/shell).
  const shell = useShell()

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      shell.settle(true)
    }
  }
</script>

{#if shell.confirming}
  {@const r = shell.confirming}
  <Modal title={r.title} width={420} onclose={() => shell.settle(false)}>
    <p>{r.message}</p>
    {#snippet footer()}
      <button class="btn" onclick={() => shell.settle(false)}>{t('common.cancel')}</button>
      <button class="btn primary" class:danger-fill={r.danger} onclick={() => shell.settle(true)} data-autofocus {onkeydown}>{r.confirmLabel}</button>
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
