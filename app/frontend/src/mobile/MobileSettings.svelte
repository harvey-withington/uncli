<script lang="ts">
  import AppearanceSection from '../components/AppearanceSection.svelte'
  import Modal from '../components/Modal.svelte'
  import UpdateSection from '../components/UpdateSection.svelte'
  import { confirm } from '../lib/confirm.svelte'
  import { t } from '../lib/i18n.svelte'
  import type { Snippet } from 'svelte'

  // The phone's Settings: this device and the server it's paired with,
  // its look, what it's told about (notifications, from the slot), the
  // PC's updates (Check now, Update now: decision 0025), and
  // Forget this device, which drops its token here (the server keeps it
  // listed until it's revoked there).

  interface Props {
    server: string
    deviceName: string
    version: string
    onforget: () => void
    onclose: () => void
    notifications?: Snippet
  }

  let { server, deviceName, version, onforget, onclose, notifications }: Props = $props()

  async function forget() {
    const ok = await confirm({ title: t('mobile.forgetTitle'), message: t('mobile.forgetBody', { server }), confirmLabel: t('mobile.forget'), danger: true })
    if (ok) onforget()
  }
</script>

<Modal title={t('mobile.settings')} width={480} {onclose}>
  <section>
    <h3>{t('mobile.thisDevice')}</h3>
    <dl>
      <dt>{t('mobile.deviceName')}</dt>
      <dd>{deviceName || '–'}</dd>
      <dt>{t('mobile.server')}</dt>
      <dd>{server}</dd>
      <dt>{t('mobile.serverVersion')}</dt>
      <dd>{version || '–'}</dd>
    </dl>
  </section>
  {@render notifications?.()}
  <UpdateSection />
  <AppearanceSection />
  <section>
    <h3>{t('mobile.pairing')}</h3>
    <p class="muted">{t('mobile.forgetHint')}</p>
    <button class="btn danger-outline" onclick={forget}>{t('mobile.forget')}</button>
  </section>
</Modal>

<style>
  section + section,
  :global(section) + section {
    margin-top: var(--space-5);
    padding-top: var(--space-4);
    border-top: 1px solid var(--border);
  }
  h3 {
    margin: 0 0 var(--space-2);
    font-size: var(--text-md);
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
    font-size: var(--text-sm);
  }
  dt {
    color: var(--text-muted);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .muted {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .danger-outline {
    min-height: 44px;
    color: var(--danger);
    border-color: var(--danger);
  }
</style>
