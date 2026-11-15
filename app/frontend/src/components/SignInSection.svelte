<script lang="ts">
  import { useApp } from '../lib/context'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import { namesOf, providerOf } from '@uncli/chat/lib/api'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import CodeSignIn from '@uncli/chat/components/CodeSignIn.svelte'

  // The phone's Settings → the PC's sign-in (decision 0034): the account
  // the first provider's CLI is signed in with there, and Sign in again,
  // by link and code, for when it expires while you're away from the PC.
  // The rest of AI Providers (versions, installs, other providers) stays
  // at the PC.
  const app = useApp()
  const provider = $derived(providerOf(app.boot))
  const names = $derived(namesOf(provider))
  const status = $derived(app.cli)
</script>

{#if provider?.signIn === 'link'}
  <section id="settings-signin">
    <h3>{t('mobile.signIn', names)}</h3>
    <p class="muted">
      {#if status?.loggedIn}<Icon name="check" size={14} />{/if}
      {status?.email ? t('settings.signedIn', { email: status.email, plan: status.subscription ?? '' }) : t('settings.notSignedIn', names)}
    </p>
    <p class="muted small">{t('settings.signInAgain.hint', names)}</p>
    <CodeSignIn label={t(status?.loggedIn ? 'settings.signInAgain.link' : 'setup.signin.button')} primary={!status?.loggedIn} />
  </section>
{/if}

<style>
  h3 {
    margin: 0 0 var(--space-2);
    font-size: var(--text-md);
  }
  .muted {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    margin: 0 0 var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .small {
    font-size: var(--text-xs);
  }
</style>
