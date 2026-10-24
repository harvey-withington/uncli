<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Modal from './Modal.svelte'

  // "Allow <name>?" (decision 0022): a device opened a pairing code and
  // waits for the user at the server. Deny has the focus, so Enter alone
  // never lets a device in; closing the dialog denies too.
  const app = useApp()
  const request = $derived(app.pairRequest)
  let busy = $state(false)

  async function answer(allow: boolean) {
    const r = request
    if (!r || busy) return
    busy = true
    try {
      await app.backend.answerPairing(r.id, allow)
      if (allow) showToast(t('pair.allowed', { name: r.name }))
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      busy = false
      app.pairRequest = null
    }
  }
</script>

{#if request}
  <Modal title={t('pair.askTitle', { name: request.name })} width={420} onclose={() => answer(false)}>
    <p>{t('pair.askBody')}</p>
    {#snippet footer()}
      <button class="btn" onclick={() => answer(false)} disabled={busy} data-autofocus>{t('pair.deny')}</button>
      <button class="btn primary" onclick={() => answer(true)} disabled={busy}>{t('pair.allow')}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  p {
    margin: 0;
    color: var(--text-muted);
  }
</style>
