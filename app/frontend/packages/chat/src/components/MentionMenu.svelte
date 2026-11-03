<script lang="ts">
  import { useChat } from '../lib/chatcontext'
  import { t } from '../lib/i18n.svelte'
  import { insertMention, mentionQuery } from '../lib/mentions'
  import type { Mention } from '../stores/chat.svelte'

  // The @mention menu above the message box, for an embedding host that
  // offers mentions (decision 0026): while an "@" and a word are typed, what
  // the host says matches. Up and Down move, Tab or Enter puts the host's
  // text in the box, Escape closes it until the text changes. The composer
  // passes its key presses through onkey, as it does the slash menu's.
  interface Props {
    text: string
    input: HTMLTextAreaElement | undefined
  }

  let { text = $bindable(), input }: Props = $props()
  const chat = useChat()
  let active = $state(0)
  let closedFor = $state('') // Escape: closed for this text
  let matches = $state<Mention[]>([])
  const at = $derived(chat.complete ? mentionQuery(text, input?.selectionStart ?? text.length) : null)
  const open = $derived(at !== null && matches.length > 0 && closedFor !== text)

  // The host is asked as the word changes; an answer to an older word is
  // let go.
  let asked = 0
  $effect(() => {
    const q = at?.query
    const complete = chat.complete
    active = 0
    if (q === undefined || !complete) {
      matches = []
      return
    }
    const n = ++asked
    Promise.resolve(complete(q)).then(
      list => { if (n === asked) matches = (list ?? []).slice(0, 8) },
      () => { if (n === asked) matches = [] },
    )
  })

  function choose(i: number) {
    const m = matches[i]
    if (!m || !at) return
    const done = insertMention(text, at, m.insert)
    text = done.text
    matches = []
    queueMicrotask(() => {
      input?.focus()
      input?.setSelectionRange(done.caret, done.caret)
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
  <div class="menu" role="listbox" aria-label={t('mention.title')}>
    {#each matches as m, i (m.insert)}
      <!-- svelte-ignore a11y_click_events_have_key_events (the message box's keys drive it) -->
      <div class="item" class:active={i === active} role="option" aria-selected={i === active} tabindex="-1" onmousedown={e => e.preventDefault()} onclick={() => choose(i)} onmouseenter={() => (active = i)}>
        <span class="label">{m.label}</span>
        {#if m.detail}<span class="detail">{m.detail}</span>{/if}
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
  .label {
    flex: none;
    max-width: 60%;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: var(--text-sm);
    color: var(--text);
  }
  .detail {
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
