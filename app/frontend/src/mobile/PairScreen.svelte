<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import { t } from '../lib/i18n.svelte'
  import { PairError, guessDeviceName, loadDeviceName, pair, saveDeviceName, type Paired } from './pairing'

  // Pairing on the phone (decision 0022). With a code from the server's QR
  // code: the device's name, then "Waiting for Allow on <server>…". With
  // none: how to get one. revoked says the server no longer knows this
  // device.

  interface Props {
    code: string
    server: string
    revoked?: boolean
    onpaired: (p: Paired) => void
  }

  let { code, server, revoked = false, onpaired }: Props = $props()
  let name = $state(loadDeviceName() || guessDeviceName(navigator.userAgent))
  let step = $state<'name' | 'waiting' | 'failed'>('name')
  let problem = $state('')

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    const n = name.trim()
    if (!n) return
    saveDeviceName(n)
    problem = ''
    try {
      const p = await pair(fetch, '', code, n, () => (step = 'waiting'))
      onpaired(p)
    } catch (err) {
      step = 'failed'
      const kind = err instanceof PairError ? err.kind : 'network'
      problem = t(`mpair.failed.${kind}`, { detail: err instanceof Error ? err.message : String(err) })
    }
  }
</script>

<main class="pair">
  <img class="mark" src="./icon-192.png" alt="" width="64" height="64" />
  <h1>{t('mpair.title')}</h1>
  {#if !code}
    {#if revoked}<p class="warn" role="alert"><Icon name="triangle-alert" size={16} />{t('mpair.revoked')}</p>{/if}
    <p>{t('mpair.howTo', { server })}</p>
    <ol>
      <li>{t('mpair.step1')}</li>
      <li>{t('mpair.step2')}</li>
      <li>{t('mpair.step3')}</li>
    </ol>
  {:else if step === 'name'}
    <p>{t('mpair.nameBody', { server })}</p>
    <form onsubmit={submit}>
      <label class="field">
        <span>{t('mpair.name')}</span>
        <input class="input" bind:value={name} maxlength="40" autocomplete="off" required />
      </label>
      <button class="btn primary big" type="submit" disabled={!name.trim()}>{t('mpair.pair')}</button>
    </form>
  {:else if step === 'waiting'}
    <p class="waiting" role="status"><Icon name="loader" spin size={18} />{t('mpair.waiting', { server })}</p>
  {:else}
    <p class="warn" role="alert"><Icon name="triangle-alert" size={16} />{problem}</p>
    <p>{t('mpair.again')}</p>
  {/if}
</main>

<style>
  .pair {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    max-width: 440px;
    margin: 0 auto;
    padding: var(--space-6) var(--space-4);
    font-size: var(--text-md);
  }
  .mark {
    align-self: flex-start;
  }
  h1 {
    margin: 0;
    font-size: var(--text-xl);
  }
  p,
  ol {
    margin: 0;
    color: var(--text-muted);
  }
  ol {
    padding-left: var(--space-5);
  }
  li + li {
    margin-top: var(--space-2);
  }
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--text-sm);
  }
  .field .input {
    min-height: 44px;
    font-size: 16px; /* no zoom on focus */
  }
  .big {
    min-height: 44px;
    justify-content: center;
  }
  .waiting,
  .warn {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .warn {
    color: var(--danger);
  }
</style>
