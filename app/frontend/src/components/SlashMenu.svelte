<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { complete, matchCommands, slashQuery } from '../lib/slash'

  // The slash-command menu above the message box: while the message is a
  // command being typed ("/com"), the session's CLI's commands that match,
  // with what they do and take. Up and Down move, Tab or Enter puts the
  // command in the box, Escape closes it until the next "/". The composer
  // passes its key presses through onkey first.
  interface Props {
    session: SessionView
    text: string
    input: HTMLTextAreaElement | undefined
  }

  let { session, text = $bindable(), input }: Props = $props()
  const app = useApp()
  let active = $state(0)
  let closedFor = $state('') // Escape: closed for this text
  const query = $derived(app.takesSlash(session) ? slashQuery(text, input?.selectionStart ?? text.length) : null)
  const list = $derived(app.commands[session.id] ?? [])
  const matches = $derived(query === null ? [] : matchCommands(list, query))
  const open = $derived(query !== null && matches.length > 0 && closedFor !== text)

  // The commands are fetched when the first "/" is typed.
  $effect(() => {
    if (query !== null) void app.loadCommands(session.id)
  })
  $effect(() => {
    void query
    active = 0
  })

  function choose(i: number) {
    const c = matches[i]
    if (!c) return
    text = complete(text, c.name)
    queueMicrotask(() => {
      input?.focus()
      input?.setSelectionRange(text.length, text.length)
    })
  }

  // onkey handles a key press in the message box; true when it was the
  // menu's.
  export function onkey(e: KeyboardEvent): boolean {
    if (!open || e.isComposing) return false
    switch (e.key) {
      case 'ArrowDown':
        active = (active + 1) % matches.length
        break
      case 'ArrowUp':
        active = (active - 1 + matches.length) % matches.length
        break
      case 'Tab':
      case 'Enter':
        if (e.shiftKey) return false
        choose(active)
        break
      case 'Escape':
        closedFor = text
        break
      default:
        return false
    }
    e.preventDefault()
    return true
  }
</script>

{#if open}
  <div class="menu" role="listbox" id="slash-menu-{session.id}" aria-label={t('slash.title')}>
    {#each matches as c, i (c.name)}
      <!-- svelte-ignore a11y_click_events_have_key_events (the message box's keys drive it) -->
      <div class="item" class:active={i === active} role="option" aria-selected={i === active} tabindex="-1" onmousedown={e => e.preventDefault()} onclick={() => choose(i)} onmouseenter={() => (active = i)}>
        <span class="name">/{c.name}</span>
        {#if c.hint}<span class="hint">{c.hint}</span>{/if}
        {#if c.description}<span class="desc">{c.description}</span>{/if}
      </div>
    {/each}
    <div class="keys">{t('slash.keys')}</div>
  </div>
{/if}

<style>
  .menu {
    position: absolute;
    left: 0;
    right: 0;
    bottom: calc(100% + 6px);
    z-index: 30;
    padding: var(--space-1);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-md);
  }
  .item {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    padding: 5px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    min-width: 0;
  }
  .item.active {
    background: var(--accent-soft);
  }
  .name {
    flex: none;
    font-family: var(--mono);
    font-size: var(--text-sm);
    color: var(--text);
  }
  .hint {
    flex: none;
    font-family: var(--mono);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .desc {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .keys {
    padding: 4px var(--space-2) 2px;
    border-top: 1px solid var(--border);
    margin-top: var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
