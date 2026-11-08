<script lang="ts">
  // New session → what an isolated session type can use of the user's own
  // (decision 0016): a checkbox for each, and what it costs.
  import type { Share } from '@uncli/chat/lib/api'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import type { ProviderNames } from '@uncli/chat/lib/api'
  import { toggled } from '@uncli/chat/lib/shares'

  let { shareable, shares = $bindable(), names }: { shareable: Share[]; shares: Share[]; names: ProviderNames } = $props()
</script>

<fieldset>
  <legend>{t('newSession.shares')}</legend>
  {#each shareable as s (s)}
    <label class="check">
      <input type="checkbox" checked={shares.includes(s)} onchange={e => (shares = toggled(shares, s, e.currentTarget.checked))} />
      {t(`shares.${s}`)}
    </label>
  {/each}
  <span class="hint">{t('shares.hint', names)}</span>
</fieldset>

<style>
  fieldset {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin: 0 0 var(--space-4);
    border: 0;
    padding: 0;
  }
  legend {
    padding: 0;
    margin-bottom: 4px;
    font-size: var(--text-sm);
    font-weight: 500;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
  }
  .hint {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
</style>
