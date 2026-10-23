<script lang="ts">
  import type { MCPServer, SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { needOf, type MCPNeed } from '../lib/mcp'
  import Icon from './Icon.svelte'

  // Lines under the session's folder for MCP servers that need the user
  // (decision 0020): one to sign in to, a sign-in waiting for the browser
  // (open the link again, paste the address the browser ended up on, or
  // cancel), a connector signed in on the provider's site waiting to be
  // reconnected, and one that couldn't connect. Nothing when all's well.
  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  const names = $derived(app.namesFor(session.adapter))
  const key = (sv: MCPServer) => `${session.id}:${sv.name}`
  const needs = $derived(
    (session.mcp ?? [])
      .map(sv => ({ sv, need: needOf(sv) }))
      .filter((x): x is { sv: MCPServer; need: Exclude<MCPNeed, null> } => x.need !== null),
  )
  let pasting = $state('') // the server whose address is being pasted
  let pasted = $state('')

  const ICON: Record<Exclude<MCPNeed, null>, string> = { signing: 'loader', 'sign-in': 'key-round', connector: 'loader', failed: 'unplug' }

  async function usePasted(server: string) {
    if (await app.mcpPaste(session.id, server, pasted)) {
      pasting = pasted = ''
    }
  }
</script>

{#if needs.length}
  <ul class="mcp" aria-label={t('mcp.title')}>
    {#each needs as { sv, need } (sv.name)}
      <li class={need}>
        <Icon name={ICON[need]} size={14} spin={need === 'signing' || need === 'connector'} />
        <span class="text">{t(`mcp.need.${need}`, { ...names, name: sv.name })}{#if sv.error}<span class="err"> {sv.error}</span>{/if}</span>
        <span class="acts">
          {#if need === 'sign-in'}
            <button class="btn small" onclick={() => app.mcpSignIn(session.id, sv.name)}>{t('mcp.signIn')}</button>
          {:else if need === 'signing'}
            {#if app.mcpLinks[key(sv)]}
              <button class="btn ghost small" onclick={() => app.backend.openURL(app.mcpLinks[key(sv)] ?? '')}>{t('mcp.openAgain')}</button>
            {/if}
            <button class="btn ghost small" onclick={() => (pasting = pasting === sv.name ? '' : sv.name)} aria-expanded={pasting === sv.name}>{t('mcp.paste')}</button>
            <button class="btn ghost small" onclick={() => app.mcpCancel(session.id, sv.name)}>{t('common.cancel')}</button>
          {:else if need === 'connector'}
            {#if app.mcpLinks[key(sv)]}
              <button class="btn ghost small" onclick={() => app.backend.openURL(app.mcpLinks[key(sv)] ?? '')}>{t('mcp.openAgain')}</button>
            {/if}
            <button class="btn ghost small" onclick={() => app.mcpReconnect(session.id, sv.name)}>{t('mcp.reconnect')}</button>
            <button class="btn ghost small" onclick={() => app.mcpCancel(session.id, sv.name)}>{t('common.cancel')}</button>
          {:else}
            <button class="btn small" onclick={() => app.mcpReconnect(session.id, sv.name)}>{t('mcp.reconnect')}</button>
          {/if}
        </span>
        {#if pasting === sv.name && need === 'signing'}
          <form class="paste" onsubmit={e => { e.preventDefault(); void usePasted(sv.name) }}>
            <!-- svelte-ignore a11y_autofocus -->
            <input bind:value={pasted} autofocus placeholder={t('mcp.pastePlaceholder')} aria-label={t('mcp.pasteLabel', { name: sv.name })} spellcheck="false" autocomplete="off" />
            <button class="btn small primary" disabled={!pasted.trim()}>{t('mcp.pasteUse')}</button>
          </form>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .mcp {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 2px var(--space-2);
    font-size: var(--text-xs);
    color: var(--warning);
  }
  li.signing,
  li.connector {
    color: var(--accent);
  }
  .text {
    min-width: 0;
  }
  .err {
    color: var(--danger);
  }
  .acts {
    display: inline-flex;
    gap: var(--space-1);
  }
  .paste {
    display: flex;
    flex-basis: 100%;
    gap: var(--space-2);
    padding-left: 22px;
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
</style>
