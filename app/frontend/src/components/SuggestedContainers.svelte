<script lang="ts">
  import type { SuggestedContainer, Suggestions } from '@uncli/chat/lib/api'
  import { useApp } from '../lib/context'
  import { thisPC, type Place } from '../stores/place.svelte'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'

  // The containers a session folder's repository suggests in .uncli
  // (decision 0028), in the new session dialog: Import for a new one, Sync
  // when the user's differs (a quieter link when it's the user who changed
  // it), and why one can't be used. One already imported and built is
  // picked for the session when nothing else was. Import and Sync open
  // ImportContainerDialog through onimport, outside the dialog's own body;
  // a change to refresh reads the suggestions again. Standalone (Settings →
  // Containers → Import from a project) lists those already imported too,
  // and says when the project suggests none.
  interface Props {
    folder: string
    place?: Place // whose folder it is: this PC's, or a paired server's (decision 0033)
    container?: string // the dialog's "Run in"
    refresh: number
    standalone?: boolean
    onimport: (s: SuggestedContainer, repo: string, name: string) => void
  }
  const app = useApp()
  let { folder, place = thisPC(app), container = $bindable(''), refresh, standalone = false, onimport }: Props = $props()
  let found = $state<Suggestions | null>(null)
  let loaded = $state(false)
  let asked = 0

  async function load(dir: string) {
    const n = ++asked
    try {
      const s = await place.backend.suggestedContainers(dir)
      if (n !== asked) return
      found = s
      const built = new Set((place.containers?.containers ?? []).filter(c => c.built).map(c => c.id))
      const mine = s.containers.find(c => c.yours && built.has(c.yours))
      if (!standalone && !container && mine?.yours) container = mine.yours
    } catch {
      if (n === asked) found = null // a folder that's gone, say: nothing to suggest
    } finally {
      if (n === asked) loaded = true
    }
  }

  $effect(() => {
    void refresh
    if (folder) void load(folder)
    else found = null
  })

  const day = (iso?: string) => (iso ? new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' }) : '')
  const shown = $derived((found?.containers ?? []).filter(c => standalone || c.status !== 'imported'))
  const labelOf = (id?: string) => place.containers?.containers.find(c => c.id === id)?.label ?? id ?? ''

  const open = (s: SuggestedContainer) => found?.repo && onimport(s, found.repo, found.name ?? '')
</script>

{#if standalone && loaded && !found?.problem && !shown.length}
  <p class="line">{t('suggested.none')}</p>
{/if}
{#if found?.problem || shown.length}<div class="list">
{#if found?.problem}
  <p class="line warn"><Icon name="triangle-alert" size={13} />{t(`suggested.problem.${found.problem}`, { detail: found.detail ?? '' })}</p>
{/if}
{#each shown as s (s.profile.id)}
  {@const label = s.profile.label || s.profile.id}
  <p class="line" class:warn={s.status === 'unusable'}>
    <Icon name={s.status === 'unusable' ? 'triangle-alert' : s.status === 'imported' ? 'check' : 'layers'} size={13} />
    <span class="text">
      {#if s.status === 'new'}{t('suggested.new', { label })}
      {:else if s.status === 'differs' && s.by === 'project'}{t('suggested.changedByProject', { label, day: day(s.changed) })}
      {:else if s.status === 'differs'}{t('suggested.changedByYou', { label })}
      {:else if s.status === 'imported'}{t('suggested.imported', { yours: labelOf(s.yours) })}
      {:else if s.problem === 'base'}{t('suggested.badBase', { label })}
      {:else}{t('suggested.invalid', { label, detail: s.detail ?? '' })}{/if}
    </span>
    {#if s.status === 'new'}
      <button class="btn small" onclick={() => open(s)}>{t('suggested.import')}</button>
    {:else if s.status === 'differs'}
      <button class="btn small" class:ghost={s.by === 'you'} onclick={() => open(s)}>{t('suggested.sync')}</button>
    {/if}
  </p>
{/each}
</div>{/if}

<style>
  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin-bottom: var(--space-4);
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .line.warn {
    color: var(--warning);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  .line .btn {
    flex: none;
  }
</style>
