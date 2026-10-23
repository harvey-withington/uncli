<script lang="ts">
  import { fly } from 'svelte/transition'
  import { clickOutside } from '../lib/actions'
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { needOf, statusKey } from '../lib/mcp'
  import Icon from './Icon.svelte'

  // The header's MCP servers button (plug; a dot when one needs the user)
  // and its popover: every server of the session's CLI with its status and
  // what can be done with it (decision 0020). Only for a CLI that can sign
  // its servers in, once it has reported them.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const servers = $derived(session.mcp ?? [])
  const attention = $derived(servers.some(sv => needOf(sv, !!app.mcpConnector[`${session.id}:${sv.name}`]) !== null))
  let open = $state(false)
</script>

<svelte:window onkeydown={e => open && e.key === 'Escape' && (open = false)} />

{#if servers.length}
  <span class="wrap">
    <button
      class="btn ghost small icon"
      class:attention
      onclick={() => (open = !open)}
      aria-expanded={open}
      aria-label={t(attention ? 'mcp.buttonAttention' : 'mcp.button')}
      title={t(attention ? 'mcp.buttonAttention' : 'mcp.button')}
    >
      <Icon name="plug" />
      {#if attention}<span class="dot" aria-hidden="true"></span>{/if}
    </button>
    {#if open}
      <div class="pop" use:clickOutside={() => (open = false)} transition:fly={{ y: -4, duration: 140 }} role="dialog" aria-label={t('mcp.title')} tabindex="-1">
        <h3>{t('mcp.title')}</h3>
        <ul>
          {#each servers as sv (sv.name)}
            <li>
              <span class="dotted {sv.signing ? 'signing' : sv.status}" aria-hidden="true"></span>
              <span class="name" title={sv.name}>{sv.name}</span>
              <span class="status">{t(statusKey(sv), { n: sv.tools })}</span>
              <span class="acts">
                {#if sv.signing}
                  <button class="btn ghost small" onclick={() => app.mcpCancel(session.id, sv.name)}>{t('common.cancel')}</button>
                {:else if sv.status === 'needs-auth'}
                  <button class="btn small" onclick={() => app.mcpSignIn(session.id, sv.name)}>{t('mcp.signIn')}</button>
                {:else if sv.status === 'failed'}
                  <button class="btn small" onclick={() => app.mcpReconnect(session.id, sv.name)}>{t('mcp.reconnect')}</button>
                {:else if sv.status === 'connected'}
                  <button class="btn ghost small" onclick={() => app.mcpReconnect(session.id, sv.name)}>{t('mcp.reconnect')}</button>
                  <button class="btn ghost small" onclick={() => app.mcpSignOut(session.id, sv.name)}>{t('mcp.signOut')}</button>
                {/if}
              </span>
              {#if sv.error}<p class="err">{sv.error}</p>{/if}
            </li>
          {/each}
        </ul>
        <p class="hint">{t('mcp.hint', app.namesFor(session.adapter))}</p>
      </div>
    {/if}
  </span>
{/if}

<style>
  .wrap {
    position: relative;
    display: inline-flex;
  }
  .attention {
    color: var(--warning);
  }
  .dot {
    position: absolute;
    top: 3px;
    right: 3px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--warning);
  }
  .pop {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 20;
    width: 380px;
    max-width: calc(100vw - 2 * var(--space-4));
    padding: var(--space-4);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    outline: none;
  }
  h3 {
    margin: 0 0 var(--space-3);
    font-size: var(--text-sm);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
    padding: 6px 0;
    border-top: 1px solid var(--border);
    font-size: var(--text-sm);
  }
  li:first-child {
    border-top: 0;
  }
  .dotted {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-faint);
  }
  .dotted.connected {
    background: var(--success);
  }
  .dotted.needs-auth,
  .dotted.failed {
    background: var(--warning);
  }
  .dotted.signing,
  .dotted.pending {
    background: var(--accent);
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
  }
  .status {
    flex: 1;
    color: var(--text-muted);
    font-size: var(--text-xs);
    white-space: nowrap;
  }
  .acts {
    display: inline-flex;
    gap: var(--space-1);
  }
  .err {
    flex-basis: 100%;
    margin: 0 0 0 16px;
    color: var(--danger);
    font-size: var(--text-xs);
  }
  .hint {
    margin: var(--space-3) 0 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
</style>
