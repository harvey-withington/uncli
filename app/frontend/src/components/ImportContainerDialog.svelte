<script lang="ts">
  import type { ContainerProfile, SuggestedContainer } from '@uncli/chat/lib/api'
  import { untrack } from 'svelte'
  import { flaggedIn, importStart, reachesOf } from '../lib/suggestions'
  import { useApp } from '../lib/context'
  import { thisPC, type Place } from '../stores/place.svelte'
  import { t } from '@uncli/chat/lib/i18n.svelte'
  import ContainerEditor from './ContainerEditor.svelte'
  import Icon from '@uncli/chat/components/Icon.svelte'
  import Modal from '@uncli/chat/components/Modal.svelte'

  // Imports a container a project suggests (decision 0028), or syncs the
  // user's with it: the container editor on the project's version, saved
  // as the user leaves it. Most suggestions are a team's ordinary setup, so
  // the summary is calm when nothing reaches outside the container, and
  // lists what does when something does, as the user changes the form.
  interface Props {
    suggestion: SuggestedContainer
    repo: string // the repository's root
    name: string // its folder's name
    place?: Place // the PC it's imported on: this one, or a paired server (decision 0033)
    onclose: () => void
  }
  const app = useApp()
  let { suggestion, repo, name, place = thisPC(app), onclose }: Props = $props()
  const start = untrack(() => importStart(suggestion))
  const yours = $derived(suggestion.yours ? place.containers?.containers.find(c => c.id === suggestion.yours) : undefined)
  const taken = $derived((place.containers?.containers ?? []).map(c => c.id).filter(id => id !== suggestion.yours))
  const bases = $derived(place.containers?.bases ?? [])
  const syncing = untrack(() => !!suggestion.yours)

  const save = (p: ContainerProfile) => place.backend.importContainer(p, repo, suggestion.profile.id)
</script>

<Modal title={t(syncing ? 'suggested.syncTitle' : 'suggested.importTitle', { label: suggestion.profile.label })} width={640} {onclose}>
  <ContainerEditor {start} container={yours} {bases} {taken} {onclose} onsave={save} flagged={suggestion.steps}
    backend={place.backend} reload={() => place.reloadContainers()}>
    {#snippet top(draft: ContainerProfile)}
      {@const reaches = reachesOf(draft)}
      {@const steps = flaggedIn(draft.setup, suggestion.steps)}
      <div class="summary" class:out={reaches.length || steps.length}>
        <p class="from">{t(syncing ? 'suggested.syncFrom' : 'suggested.from', { name })}</p>
        {#if !reaches.length && !steps.length}
          <p class="verdict"><Icon name="shield-check" size={15} />{t('suggested.inside')}</p>
        {:else}
          <p class="verdict"><Icon name="triangle-alert" size={15} />{t('suggested.outside')}</p>
          <ul>
            {#each reaches as r (r)}<li>{t(`suggested.reach.${r}`)}</li>{/each}
            {#if steps.length}<li>{t(steps.length === 1 ? 'suggested.reach.steps.one' : 'suggested.reach.steps.other', { n: steps.length })}</li>{/if}
          </ul>
          <p class="note">{t('suggested.outsideNote')}</p>
        {/if}
      </div>
    {/snippet}
  </ContainerEditor>
</Modal>

<style>
  .summary {
    margin-bottom: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    font-size: var(--text-sm);
  }
  .summary.out {
    border-color: var(--warning);
  }
  p {
    margin: 0;
  }
  .from {
    color: var(--text-muted);
    margin-bottom: var(--space-1);
  }
  .verdict {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--success);
  }
  .out .verdict {
    color: var(--warning);
    font-weight: 600;
  }
  ul {
    margin: var(--space-1) 0 0;
    padding-left: var(--space-5);
  }
  .note {
    margin-top: var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
</style>
