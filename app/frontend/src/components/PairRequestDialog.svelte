<script lang="ts">
  import type { PairRequest } from '../lib/api/types'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Modal from './Modal.svelte'

  // "Allow <name>?" (decision 0022): a device opened a pairing code and
  // waits for the user at the server. The name is only what the device
  // says, so the dialog also says where it calls from, as Tailscale knows
  // it. Deny has the focus, so Enter alone never lets a device in; closing
  // the dialog denies too, and it closes itself once the request expires.
  const app = useApp()
  const request = $derived(app.pairRequest)
  let busy = $state(false)

  function from(r: PairRequest): string {
    if (r.local) return t('pair.fromLocal')
    if (r.machine && r.user) return t('pair.fromPeer', { machine: r.machine, user: r.user })
    if (r.machine) return t('pair.fromMachine', { machine: r.machine })
    return t('pair.fromUnknown')
  }

  $effect(() => {
    const r = request
    if (!r) return
    const left = new Date(r.expires).getTime() - Date.now()
    const timer = setTimeout(() => {
      if (app.pairRequest?.id !== r.id) return
      app.pairRequest = null
      showToast(t('pair.askExpired', { name: r.name }))
    }, Math.max(0, left))
    return () => clearTimeout(timer)
  })

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
    <p class="from" class:unknown={!request.local && !request.machine}>{from(request)}</p>
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
  .from {
    margin-bottom: var(--space-2);
    color: var(--text);
    font-weight: 600;
  }
  .from.unknown {
    color: var(--warning);
  }
</style>
