<script lang="ts">
  import type { Page } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { providerOf } from '../lib/providers'
  import { errorDetail, resend } from '../lib/signin'
  import { showToast } from '../lib/toasts.svelte'
  import CodeSignIn from './CodeSignIn.svelte'
  import Icon from './Icon.svelte'

  // A turn the provider refused because the CLI's sign-in failed or expired
  // (often when the connection changes while it's renewed). The session has
  // stopped its CLI, so signing in again here and sending again starts one
  // with the new sign-in, on the same conversation: no restart of the app.
  interface Props {
    page: Page
  }

  let { page }: Props = $props()
  const app = useApp()
  const session = $derived(app.sessions.find(s => s.id === page.sessionId))
  const names = $derived(app.namesFor(session?.adapter))
  const provider = $derived(providerOf(app.boot, session?.adapter))
  // Only the first provider signs in by link and code from here (Service.SignIn).
  const here = $derived(provider?.signIn === 'link' && provider.id === app.boot?.providers[0]?.id)
  const latest = $derived(app.pages[page.sessionId]?.at(-1)?.id === page.id)
  let signedIn = $state(false)
  let signingIn = $state(false) // the code is being asked for: Send again waits
  let sending = $state(false)

  async function sendAgain() {
    const { text, refs } = resend(page)
    sending = true
    try {
      await app.backend.send(page.sessionId, text, refs)
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      sending = false
    }
  }
</script>

<div class="signin-failed" role="alert">
  <p class="what"><Icon name="key-round" size={14} />{t('signinFailed.what', names)}</p>
  {#if errorDetail(page.error)}<p class="detail">{errorDetail(page.error)}</p>{/if}
  <div class="actions">
    {#if signedIn}
      <span class="ok"><Icon name="check" size={14} />{t('signinFailed.signedIn', names)}</span>
    {:else if here}
      <CodeSignIn label={t('signinFailed.signIn')} primary onsignedin={() => { signedIn = true; signingIn = false }} bind:active={signingIn} />
    {:else if provider}
      <button class="btn small primary" onclick={() => app.openSettings(`provider-${provider.id}`)}>
        <Icon name="log-in" size={13} />{t('signinFailed.signInSettings')}
      </button>
    {/if}
    {#if latest && !signingIn}
      <button class="btn small" class:primary={signedIn} onclick={sendAgain} disabled={sending || session?.busy}>
        {#if sending}<Icon name="loader" spin size={13} />{:else}<Icon name="rotate-ccw" size={13} />{/if}{t('signinFailed.sendAgain')}
      </button>
    {/if}
  </div>
</div>

<style>
  .signin-failed {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: var(--space-5) 0 0;
    padding: var(--space-3);
    border-radius: var(--radius-sm);
    background: var(--danger-soft);
    font-size: var(--text-sm);
  }
  .what {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    color: var(--danger);
    font-weight: 560;
  }
  .detail {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .ok {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 28px;
    color: var(--success, var(--text));
  }
</style>
