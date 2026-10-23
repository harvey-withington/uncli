<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { Modifier, SessionView, ToolbarItem } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { arrange, choicesOf, chosenIn } from '../lib/modifiers'
  import { modelOptions } from '../lib/models'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'
  import CycleButton from './CycleButton.svelte'
  import UsagePopover from './UsagePopover.svelte'

  interface Props {
    session: SessionView
    end?: Snippet // at the right end of the last line (the prompt level)
  }

  let { session, end }: Props = $props()
  const app = useApp()
  const items = $derived(app.boot?.toolbar ?? [])
  const profile = $derived(app.boot?.profiles.find(p => p.id === session.profileId))
  const models = $derived(modelOptions(app.modelsFor(session.adapter), session.model, t('model.cliDefault', app.namesFor(session.adapter))))
  // A modifier needs its tools to exist in the profile (empty = CLI defaults).
  const modifiers = $derived((app.boot?.modifiers ?? []).filter(m => {
    const tools = profile?.tools ?? []
    return tools.length === 0 || (m.requiresTools ?? []).every(r => tools.includes(r))
  }))
  let usageOpen = $state(false)

  async function run(fn: () => Promise<unknown>) {
    try {
      await fn()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  function setModel(e: Event) {
    const value = (e.currentTarget as HTMLSelectElement).value
    run(() => app.backend.setModel(session.id, value))
  }

  function toggle(id: string) {
    const on = !session.modifiers.includes(id)
    run(async () => app.upsertSession(await app.backend.toggleModifier(session.id, id, on)))
  }

  // groupHint is what a group's choice does (modifier.hint.<group>), in the
  // session's provider's words; '' when there's none.
  function groupHint(group: string): string {
    const key = `modifier.hint.${group}`
    const hint = t(key, app.namesFor(session.adapter))
    return hint === key ? '' : hint
  }

  // choose picks one of a group (the backend turns the others off), or
  // Normal: whichever is on goes off.
  function choose(members: Modifier[], id: string) {
    const on = chosenIn(members, session.modifiers)
    if (id) {
      run(async () => app.upsertSession(await app.backend.toggleModifier(session.id, id, true)))
    } else if (on) {
      run(async () => app.upsertSession(await app.backend.toggleModifier(session.id, on, false)))
    }
  }

  function slash(item: ToolbarItem) {
    const command = item.command
    if (command) run(() => app.send(command))
  }

  async function native(item: ToolbarItem) {
    if (item.action === 'open_workdir') return run(() => app.backend.openFolder(session.workdir))
    if (item.action === 'usage') {
      usageOpen = !usageOpen
      if (usageOpen) {
        await run(async () => {
          const u = await app.backend.usage()
          if (u) app.usage = u
        })
      }
    }
  }
</script>

<div class="toolbar" role="toolbar" aria-label={t('toolbar.label')}>
  {#each items as item, i (i)}
    {#if item.kind === 'model_picker'}
      <label class="model" title={t('toolbar.model')}>
        <Icon name="sparkles" size={14} />
        <span class="visually-hidden">{t('toolbar.model')}</span>
        <select value={session.model} onchange={setModel}>
          {#each models as m (m.value)}
            <option value={m.value}>{m.label}</option>
          {/each}
        </select>
      </label>
    {:else if item.kind === 'modifiers'}
      {#each arrange(modifiers) as x (x.kind === 'group' ? `group:${x.id}` : x.modifier.id)}
        {#if x.kind === 'toggle'}
          {@const on = session.modifiers.includes(x.modifier.id)}
          <button class="btn small toggle" class:on aria-pressed={on} onclick={() => toggle(x.modifier.id)} title={x.modifier.label}>
            <Icon name={x.modifier.icon} size={13} />{x.modifier.label}
          </button>
        {:else}
          <CycleButton
            choices={choicesOf(x.members, t('modifier.normal'), groupHint(x.id))}
            value={chosenIn(x.members, session.modifiers)}
            label={choicesOf(x.members, t('modifier.normal')).map(c => c.label).join(' / ')}
            showLabel={false}
            onchoose={id => choose(x.members, id)}
          />
        {/if}
      {/each}
    {:else if item.kind === 'separator'}
      <span class="sep" aria-hidden="true"></span>
    {:else if item.kind === 'slash' && app.takesSlash(session)}
      <button class="btn small ghost" onclick={() => slash(item)} disabled={session.busy} title={item.command}>
        <Icon name={item.icon ?? 'terminal'} size={13} />{item.label}
      </button>
    {:else if item.kind === 'native'}
      <span class="native">
        <button class="btn small ghost" onclick={() => native(item)} aria-expanded={item.action === 'usage' ? usageOpen : undefined}>
          <Icon name={item.icon ?? 'sparkles'} size={13} />{item.label}
        </button>
        {#if item.action === 'usage' && usageOpen}
          <UsagePopover usage={app.usage} onclose={() => (usageOpen = false)} />
        {/if}
      </span>
    {/if}
  {/each}
  {#if end}<div class="end">{@render end()}</div>{/if}
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
  }
  .end {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-left: auto;
    padding-left: var(--space-3);
  }
  .model {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 26px;
    padding: 0 4px 0 9px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--text-muted);
  }
  .model:focus-within {
    border-color: var(--accent);
  }
  .model select {
    height: 24px;
    border: 0;
    background: transparent;
    color: var(--text);
    font-size: var(--text-xs);
    font-weight: 500;
    outline: none;
    cursor: pointer;
  }
  .toggle {
    color: var(--text-muted);
    background: transparent;
  }
  .toggle.on {
    background: var(--accent-soft);
    border-color: transparent;
    color: var(--accent);
  }
  .sep {
    width: 1px;
    height: 16px;
    margin: 0 4px;
    background: var(--border);
  }
  .native {
    position: relative;
  }
</style>
