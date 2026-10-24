<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import type { Pairing } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { countdown } from '../lib/remote'
  import { showToast } from '../lib/toasts.svelte'
  import CopyButton from './CopyButton.svelte'
  import Icon from './Icon.svelte'
  import Modal from './Modal.svelte'

  // Pair a device (decision 0022): a QR code and a link carrying a code
  // that works once, for a few minutes. When the device asks, the window's
  // "Allow <name>?" takes over and this closes.

  interface Props {
    onclose: () => void
  }

  let { onclose }: Props = $props()
  const app = useApp()
  let pairing = $state<Pairing | null>(null)
  let now = $state(Date.now())
  let timer: ReturnType<typeof setInterval> | undefined
  const left = $derived(pairing ? countdown(pairing.expires, now) : null)

  async function start() {
    try {
      pairing = await app.backend.startPairing()
      now = Date.now()
    } catch (e) {
      showToast(String(e), 'error')
      onclose()
    }
  }

  onMount(() => {
    void start()
    timer = setInterval(() => (now = Date.now()), 1000)
  })
  onDestroy(() => clearInterval(timer))

  // The code is spent once a device asks: Allow takes over.
  $effect(() => {
    if (app.pairRequest) onclose()
  })

  function close() {
    void app.backend.cancelPairing().catch(() => {})
    onclose()
  }
</script>

<Modal title={t('pair.title')} width={440} onclose={close}>
  {#if !pairing}
    <p class="center"><Icon name="loader" spin size={18} label={t('app.loading')} /></p>
  {:else}
    <p class="lead">{t('pair.body')}</p>
    <div class="qr" class:expired={!left}>
      <img src={pairing.qr} alt={t('pair.qrAlt')} width="220" height="220" />
    </div>
    <div class="link">
      <code>{pairing.url}</code>
      <CopyButton text={pairing.url} label={t('pair.copyLink')} />
    </div>
    <p class="expiry" role="status">
      {#if left}{t('pair.expiresIn', { time: left })}{:else}{t('pair.expired')}{/if}
    </p>
  {/if}
  {#snippet footer()}
    {#if pairing && !left}<button class="btn" onclick={start}>{t('pair.newCode')}</button>{/if}
    <button class="btn primary" onclick={close}>{t('pair.done')}</button>
  {/snippet}
</Modal>

<style>
  .lead {
    margin: 0 0 var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .center {
    display: grid;
    place-items: center;
    min-height: 200px;
  }
  .qr {
    display: grid;
    place-items: center;
    padding: var(--space-3);
    margin: 0 auto;
    width: max-content;
    border-radius: var(--radius);
    /* A QR code needs a light ground to scan, in either theme. */
    background: var(--qr-ground);
  }
  .qr img {
    display: block;
    image-rendering: pixelated;
  }
  .qr.expired img {
    opacity: 0.15;
  }
  .link {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .link code {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    font-size: var(--text-xs);
    user-select: all;
  }
  .expiry {
    margin: var(--space-2) 0 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
</style>
