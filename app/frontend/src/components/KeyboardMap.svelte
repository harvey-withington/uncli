<script lang="ts">
  import { GROUPS, keyLabel, LOCAL_KEYS, SHORTCUTS } from '../lib/commands'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import Modal from './Modal.svelte'

  // Every shortcut, grouped: the registry's table (lib/commands.ts, which
  // the key handler runs from, so the map can't drift from what keys do),
  // and the few that belong to one place, such as the message box. Opened
  // with ? or from the command palette.

  const app = useApp()
  const mac = $derived(app.boot?.platform === 'darwin')
  const groups = GROUPS.map(g => ({ g, list: SHORTCUTS.filter(c => c.group === g) })).filter(x => x.list.length)
</script>

<Modal title={t('keys.title')} width={620} onclose={() => (app.keysOpen = false)}>
  <div class="cols">
    {#each groups as { g, list } (g)}
      <section>
        <h3>{t(`cmd.group.${g}`)}</h3>
        <dl>
          {#each list as c (c.id)}
            <div class="row">
              <dt>{t(c.label)}</dt>
              <dd>
                {#each c.keys ?? [] as k, i (i)}
                  {#if i > 0}<span class="or">{t('keys.or')}</span>{/if}
                  {#each keyLabel(k, mac) as part, j (j)}<kbd>{part}</kbd>{/each}
                {/each}
              </dd>
            </div>
          {/each}
        </dl>
      </section>
    {/each}
    <section>
      <h3>{t('keys.elsewhere')}</h3>
      <dl>
        {#each LOCAL_KEYS as k (k.keys + k.where)}
          <div class="row">
            <dt>{t(k.does)} <span class="where">{t(k.where)}</span></dt>
            <dd><kbd>{k.keys}</kbd></dd>
          </div>
        {/each}
      </dl>
    </section>
  </div>
  <p class="note">{t('keys.note')}</p>
</Modal>

<style>
  .cols {
    columns: 2;
    column-gap: var(--space-6);
  }
  section {
    break-inside: avoid;
    margin-bottom: var(--space-4);
  }
  h3 {
    margin: 0 0 var(--space-2);
    font-size: var(--text-xs);
    font-weight: 600;
    color: var(--text-muted);
  }
  dl {
    margin: 0;
  }
  .row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
    padding: 3px 0;
    font-size: var(--text-sm);
  }
  dt {
    min-width: 0;
  }
  .where {
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  dd {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: 3px;
    margin: 0;
  }
  .or {
    margin: 0 2px;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  kbd {
    min-width: 20px;
    padding: 1px 5px;
    border: 1px solid var(--border);
    border-bottom-width: 2px;
    border-radius: 4px;
    background: var(--surface-2);
    color: var(--text-muted);
    font-family: var(--font);
    font-size: 11px;
    text-align: center;
  }
  .note {
    margin: var(--space-2) 0 0;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
