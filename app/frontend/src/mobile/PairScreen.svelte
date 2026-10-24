<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import { t } from '../lib/i18n.svelte'
  import { onMount } from 'svelte'
  import { PairError, claimPair, guessDeviceName, loadDeviceName, loadPending, normaliseCode, requestPair, saveDeviceName, type Paired } from './pairing'

  // Pairing on the phone (decision 0022). With a code from the server's QR
  // code: the device's name, then "Waiting for Allow on <server>…". With
  // none: how to get one. revoked says the server no longer knows this
  // device.

  interface Props {
    code: string
    server: string
    revoked?: boolean
    onpaired: (p: Paired) => void
    oncode: (code: string) => void // a code typed in (the link lost it, or it was read out)
  }

  let { code, server, revoked = false, onpaired, oncode }: Props = $props()
  let typed = $state('')
  const typedCode = $derived(normaliseCode(typed))
  let name = $state(loadDeviceName() || guessDeviceName(navigator.userAgent))
  let step = $state<'name' | 'waiting' | 'failed'>('name')
  let problem = $state('')

  function failed(err: unknown) {
    step = 'failed'
    const kind = err instanceof PairError ? err.kind : 'network'
    problem = t(`mpair.failed.${kind}`, { detail: err instanceof Error ? err.message : String(err) })
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    const n = name.trim()
    if (!n) return
    saveDeviceName(n)
    problem = ''
    try {
      const p = await requestPair(fetch, '', code, n)
      step = 'waiting'
      onpaired(await claimPair(fetch, '', p))
    } catch (err) {
      failed(err)
    }
  }

  // A request still waiting for Allow (the page was reloaded or put away):
  // carry on waiting for it.
  onMount(() => {
    const p = loadPending()
    if (!p) return
    step = 'waiting'
    claimPair(fetch, '', p).then(onpaired, failed)
  })
</script>

<main class="pair">
  <img class="mark" src="./icon-192.png" alt="" width="64" height="64" />
  <h1>{t('mpair.title')}</h1>
  {#if step === 'waiting'}
    <p class="waiting" role="status"><Icon name="loader" spin size={18} />{t('mpair.waiting', { server })}</p>
  {:else if !code && step === 'name'}
    {#if revoked}<p class="warn" role="alert"><Icon name="triangle-alert" size={16} />{t('mpair.revoked')}</p>{/if}
    <p>{t('mpair.howTo', { server })}</p>
    <ol>
      <li>{t('mpair.step1')}</li>
      <li>{t('mpair.step2')}</li>
      <li>{t('mpair.step3')}</li>
    </ol>
    <form onsubmit={e => { e.preventDefault(); if (typedCode) oncode(typedCode) }}>
      <label class="field">
        <span>{t('mpair.typeCode')}</span>
        <input class="input" bind:value={typed} autocomplete="off" autocapitalize="characters" spellcheck="false" placeholder="ABCDE-FGHJK" />
      </label>
      <button class="btn big" type="submit" disabled={!typedCode}>{t('mpair.useCode')}</button>
    </form>
  {:else if step === 'name'}
    <p>{t('mpair.nameBody', { server })}</p>
    <form onsubmit={submit}>
      <label class="field">
        <span>{t('mpair.name')}</span>
        <input class="input" bind:value={name} maxlength="40" autocomplete="off" required />
      </label>
      <button class="btn primary big" type="submit" disabled={!name.trim()}>{t('mpair.pair')}</button>
    </form>
  {:else}
    <p class="warn" role="alert"><Icon name="triangle-alert" size={16} />{problem}</p>
    <p>{t('mpair.again')}</p>
    <button class="btn big" onclick={() => { step = 'name'; oncode('') }}>{t('mpair.newCode')}</button>
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
