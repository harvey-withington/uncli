<script lang="ts">
  import { untrack } from 'svelte'
  import { fade, fly } from 'svelte/transition'
  import { focusTrap } from '../lib/actions'
  import { commands, keyLabel, loadRecent, rank, saveRecent, type Command } from '../lib/commands'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  // The command palette (Ctrl+Shift+P, F1): every action that can run now,
  // filtered as you type (each word a prefix of a word, in order; or the
  // letters in order). Recent commands come first. ↑/↓ choose, Enter runs,
  // Escape closes; the input keeps focus throughout (aria-activedescendant).

  const app = useApp()
  let query = $state('')
  let active = $state(0)
  let recent = $state(loadRecent())
  // The list is built once, when the palette opens: running a command
  // closes it, so it never needs to follow state while open.
  const all = untrack(() => commands(app).filter(c => c.id !== 'palette'))
  const list = $derived(rank(all, query, recent))
  const mac = $derived(app.boot?.platform === 'darwin')
  let listEl: HTMLElement | undefined = $state()

  $effect(() => {
    void query
    active = 0
  })
  $effect(() => {
    listEl?.querySelector(`#palette-opt-${active}`)?.scrollIntoView?.({ block: 'nearest' })
  })

  function close() {
    app.paletteOpen = false
  }
  function choose(c: Command | undefined) {
    if (!c) return
    recent = saveRecent(c.id, recent)
    close()
    c.run()
  }
  function onkeydown(e: KeyboardEvent) {
    const n = list.length
    if (e.key === 'Escape') {
      e.stopPropagation()
      close()
    } else if (e.key === 'ArrowDown' && n) {
      e.preventDefault()
      active = (active + 1) % n
    } else if (e.key === 'ArrowUp' && n) {
      e.preventDefault()
      active = (active - 1 + n) % n
    } else if (e.key === 'Enter') {
      e.preventDefault()
      choose(list[active])
    }
  }
</script>

<div class="scrim" transition:fade={{ duration: 100 }} onpointerdown={e => e.target === e.currentTarget && close()} role="presentation">
  <div class="palette" role="dialog" aria-modal="true" aria-label={t('palette.title')} tabindex="-1" use:focusTrap transition:fly={{ y: -6, duration: 140 }}>
    <div class="field">
      <Icon name="search" size={16} />
      <!-- svelte-ignore a11y_autofocus -->
      <input
        bind:value={query}
        {onkeydown}
        autofocus
        role="combobox"
        aria-expanded="true"
        aria-controls="palette-list"
        aria-activedescendant={list.length ? `palette-opt-${active}` : undefined}
        aria-label={t('palette.title')}
        placeholder={t('palette.placeholder')}
        spellcheck="false"
        autocomplete="off"
      />
    </div>
    {#if list.length === 0}
      <p class="none">{t('palette.none')}</p>
    {:else}
      <ul id="palette-list" role="listbox" aria-label={t('palette.title')} bind:this={listEl}>
        {#each list as c, i (c.id)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <li
            id="palette-opt-{i}"
            role="option"
            aria-selected={i === active}
            class:active={i === active}
            onpointermove={() => (active = i)}
            onclick={() => choose(c)}
          >
            <span class="group">{t(`cmd.group.${c.group}`)}</span>
            <span class="label">{c.label}</span>
            {#if c.checked}<span class="checked" aria-label={t('palette.current')}><Icon name="check" size={13} /></span>{/if}
            {#if c.keys?.[0]}
              {@const k = c.keys[0]}
              <span class="keys">{#each keyLabel(k, mac) as part, j (j)}<kbd>{part}</kbd>{/each}</span>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
    <p class="hint">{t('palette.hint')}</p>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
    background: var(--scrim);
  }
  .palette {
    width: min(620px, calc(100vw - 32px));
    display: flex;
    flex-direction: column;
    max-height: 64vh;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-lg);
    overflow: hidden;
    outline: none;
  }
  .field {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
    color: var(--text-faint);
  }
  input {
    flex: 1;
    border: 0;
    background: none;
    outline: none;
    color: var(--text);
    font-size: var(--text-md);
  }
  ul {
    position: relative;
    margin: 0;
    padding: var(--space-1);
    list-style: none;
    overflow-y: auto;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: 7px var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: var(--text-sm);
  }
  li.active {
    background: var(--accent-soft);
  }
  .group {
    flex: none;
    width: 76px;
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
  }
  li.active .label {
    color: var(--accent);
  }
  .checked {
    display: inline-flex;
    color: var(--success);
  }
  .keys {
    display: inline-flex;
    gap: 3px;
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
  .none {
    margin: 0;
    padding: var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .hint {
    margin: 0;
    padding: var(--space-2) var(--space-4);
    border-top: 1px solid var(--border);
    font-size: var(--text-xs);
    color: var(--text-faint);
  }
</style>
