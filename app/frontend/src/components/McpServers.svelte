<script lang="ts">
  import { fly } from 'svelte/transition'
  import { clickOutside } from '../lib/actions'
  import type { MCPServer, SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { needOf, statusKey } from '../lib/mcp'
  import Icon from './Icon.svelte'

  // The header's MCP servers button (plug; a dot when one needs the user)
  // and its popover: every server of the session's CLI with its status and
  // what can be done with it (decision 0020). A sign-in waiting for the
  // browser can open its page again or take the address the browser ended
  // up on, for when the browser can't reach the CLI. Only for a CLI that
  // can sign its servers in, once it has reported them.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const servers = $derived(session.mcp ?? [])
  const attention = $derived(servers.some(sv => needOf(sv) !== null))
  let open = $state(false)
  const link = (sv: MCPServer) => app.mcpLinks[`${session.id}:${sv.name}`] ?? ''
  let pasting = $state('') // the server whose address is being pasted
  let pasted = $state('')

  async function usePasted(server: string) {
    if (await app.mcpPaste(session.id, server, pasted)) {
      pasting = pasted = ''
    }
  }
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
              <span class="dotted {sv.signing || sv.site ? 'signing' : sv.status}" aria-hidden="true"></span>
              <span class="who">
                <span class="name" title={sv.name}>{sv.name}</span>
                <span class="status">{t(statusKey(sv), { n: sv.tools })}</span>
              </span>
              <span class="acts">
                {#if sv.signing}
                  <button class="btn ghost small" onclick={() => app.mcpCancel(session.id, sv.name)}>{t('common.cancel')}</button>
                {:else if sv.site}
                  <button class="btn ghost small" onclick={() => app.mcpReconnect(session.id, sv.name)}>{t('mcp.reconnect')}</button>
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
              {#if (sv.signing || sv.site) && (link(sv) || sv.signing)}
                <span class="more">
                  {#if link(sv)}
                    <button class="btn ghost small" onclick={() => app.backend.openURL(link(sv))}>{t('mcp.openAgain')}</button>
                  {/if}
                  {#if sv.signing}
                    <button class="btn ghost small" onclick={() => (pasting = pasting === sv.name ? '' : sv.name)} aria-expanded={pasting === sv.name}>{t('mcp.paste')}</button>
                  {/if}
                </span>
              {/if}
              {#if pasting === sv.name && sv.signing}
                <form class="paste" onsubmit={e => { e.preventDefault(); void usePasted(sv.name) }}>
                  <!-- svelte-ignore a11y_autofocus -->
                  <input bind:value={pasted} autofocus placeholder={t('mcp.pastePlaceholder')} aria-label={t('mcp.pasteLabel', { name: sv.name })} spellcheck="false" autocomplete="off" />
                  <button class="btn small primary" disabled={!pasted.trim()}>{t('mcp.pasteUse')}</button>
                </form>
              {/if}
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
    max-height: calc(100vh - 120px);
    overflow-y: auto;
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
  /* A row: the status dot, the name over its status, the actions at the
     right; a failure underneath. */
  li {
    display: grid;
    grid-template-columns: 8px minmax(0, 1fr) auto;
    align-items: center;
    gap: 2px var(--space-3);
    padding: 7px 0;
    border-top: 1px solid var(--border);
    font-size: var(--text-sm);
  }
  li:first-child {
    border-top: 0;
  }
  .dotted {
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
  .who {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
  }
  .status {
    color: var(--text-muted);
    font-size: var(--text-xs);
  }
  .acts {
    display: inline-flex;
    gap: var(--space-1);
  }
  /* Under the name while a sign-in waits for the browser. */
  .more {
    grid-column: 2 / -1;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    margin-left: calc(-1 * var(--space-2));
  }
  .paste {
    grid-column: 2 / -1;
    display: flex;
    gap: var(--space-2);
  }
  .paste input {
    flex: 1;
    min-width: 0;
    height: 26px;
    padding: 0 var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--text-xs);
  }
  .err {
    grid-column: 2 / -1;
    margin: 0;
    color: var(--danger);
    font-size: var(--text-xs);
  }
  .hint {
    margin: var(--space-3) 0 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
</style>
