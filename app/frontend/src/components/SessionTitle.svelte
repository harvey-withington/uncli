<script lang="ts">
  import type { SessionView } from '../lib/api'
  import { hold } from '../lib/backguard'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'

  // A session header's title: click it to rename the session in place
  // (Enter or leaving the box saves, Escape cancels), as the pencil in the
  // session list does; Back cancels too. It sits inside the header's h1,
  // taking its font.

  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  let editing = $state(false)
  let draft = $state('')
  // While the box is open, Back cancels it (lib/backguard.ts).
  $effect(() => {
    if (editing) return hold(() => (editing = false))
  })

  function start() {
    draft = session.title
    editing = true
  }

  async function commit() {
    if (!editing) return
    editing = false
    const title = draft.trim()
    if (title === session.title) return
    try {
      app.upsertSession(await app.backend.rename(session.id, title))
    } catch (e) {
      showToast(String(e), 'error')
    }
  }
</script>

{#if editing}
  <!-- svelte-ignore a11y_autofocus -->
  <input
    class="input edit"
    bind:value={draft}
    autofocus
    aria-label={t('session.rename')}
    onblur={commit}
    onkeydown={e => {
      if (e.key === 'Enter') commit()
      if (e.key === 'Escape') { e.stopPropagation(); editing = false }
    }}
  />
{:else}
  <button class="show" class:untitled={!session.title} onclick={start} title={t('session.renameHint', { title: session.title || t('session.untitled') })}>
    {session.title || t('session.untitled')}
  </button>
{/if}

<style>
  .show {
    display: block;
    max-width: 100%;
    padding: 0;
    border: none;
    background: none;
    font: inherit;
    letter-spacing: inherit;
    color: inherit;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: text;
  }
  /* No padding or background: either would change the h1's width and cut
     the title short. */
  .show:hover {
    text-decoration: underline dotted var(--text-faint);
    text-underline-offset: 4px;
  }
  .untitled {
    color: var(--text-muted);
  }
  .edit {
    width: 100%;
    height: 30px;
    font: inherit;
  }
</style>
