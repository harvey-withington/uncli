<script lang="ts">
  import { httpBackend } from '../lib/api/http'
  import { serverKey, withServer } from '../lib/api/identity'
  import type { Backend } from '../lib/api'
  import MobileShell from './MobileShell.svelte'
  import PairScreen from './PairScreen.svelte'
  import { loadDeviceName, loadToken, pairCodeFromHash, saveToken, type Paired } from './pairing'

  // The phone app UNCLI serves at /m/ (decision 0021). Not paired (or
  // revoked), it pairs: with the code from the server's QR code, or with
  // how to get one. Paired, it's the session list and sessions of this
  // server, through the HTTP backend, keyed as this server's.

  interface Props {
    // A backend to use instead of the server (tests and the mock preview).
    backend?: Backend
  }

  let { backend: given }: Props = $props()
  const server = serverKey(location.origin)
  let token = $state(loadToken())
  let code = $state(pairCodeFromHash(location.hash))
  let revoked = $state(false)

  function paired(p: Paired) {
    saveToken(p.token)
    history.replaceState(null, '', location.pathname)
    code = ''
    revoked = false
    token = p.token
  }

  function forget(wasRevoked: boolean) {
    saveToken(null)
    revoked = wasRevoked
    token = null
  }

  const backend = $derived.by(() => {
    if (given) return given
    if (!token) return null
    return withServer(httpBackend({ base: '', token: () => token, unauthorized: () => forget(true) }), server)
  })
</script>

{#if backend && !code}
  {#key backend}
    <MobileShell {backend} {server} deviceName={loadDeviceName()} onforget={() => forget(false)} />
  {/key}
{:else}
  <PairScreen {code} {server} {revoked} onpaired={paired} />
{/if}
