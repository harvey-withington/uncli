<script lang="ts">
  import { fly } from 'svelte/transition'
  import { clickOutside } from '../lib/actions'
  import type { Share, SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { sharesNote, toggled } from '../lib/shares'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  // What an isolated session can use of the user's own (decision 0016):
  // a quiet note in the header that opens the same checkboxes as the
  // new-session dialog. A change applies from the next message.
  interface Props {
    session: SessionView
    compact?: boolean // the icon alone, its words as its name (the compact header)
  }

  let { session, compact = false }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const shares = $derived(session.shares ?? [])
  let open = $state(false)
  let saving = $state(false)

  async function set(share: Share, on: boolean) {
    saving = true
    try {
      app.upsertSession(await app.backend.setShares(session.id, toggled(shares, share, on)))
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      saving = false
    }
  }
</script>

{#if session.shareable?.length}
  <span class="wrap">
    <button
      class="note"
      class:none={!shares.length}
      onclick={() => (open = !open)}
      aria-expanded={open}
      aria-haspopup="dialog"
      aria-label={compact ? t(sharesNote(shares)) : undefined}
      title={compact ? t(sharesNote(shares)) : t('shares.title')}
    >
      <Icon name={shares.length ? 'plug' : 'unplug'} size={compact ? 14 : 12} />{#if !compact}{t(sharesNote(shares))}{/if}
    </button>
    {#if open}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="pop" use:clickOutside={() => (open = false)} onkeydown={e => e.key === 'Escape' && (open = false)} transition:fly={{ y: -4, duration: 140 }} role="dialog" aria-label={t('shares.title')} tabindex="-1">
        <h3>{t('shares.title')}</h3>
        {#each session.shareable as s (s)}
          <label class="check">
            <input type="checkbox" checked={shares.includes(s)} disabled={saving} onchange={e => set(s, e.currentTarget.checked)} />
            {t(`shares.${s}`)}
          </label>
        {/each}
        <p class="applies">{t('shares.applies', names)}</p>
      </div>
    {/if}
  </span>
{/if}

<style>
  .wrap {
    position: relative;
    display: inline-flex;
  }
  .note {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 0 4px;
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--text-xs);
    white-space: nowrap;
    cursor: pointer;
  }
  .note:hover {
    color: var(--text);
    background: var(--surface-2);
  }
  .note.none {
    color: var(--text-faint);
  }
  .pop {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 20;
    width: 300px;
    padding: var(--space-4);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    outline: none;
    font-family: var(--font);
    white-space: normal;
  }
  h3 {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
    color: var(--text);
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text);
  }
  .applies {
    margin: var(--space-3) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.45;
  }
</style>
