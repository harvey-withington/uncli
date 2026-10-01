<script lang="ts">
  import { autosize } from '../lib/actions'
  import type { SessionView } from '../lib/api'
  import { useApp } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import { showToast } from '../lib/toasts.svelte'
  import Icon from './Icon.svelte'

  interface Props {
    session: SessionView
  }

  let { session }: Props = $props()
  const app = useApp()
  let text = $state('')
  let sending = $state(false)
  let input: HTMLTextAreaElement | undefined = $state()

  // Drafts are per session and survive switching between sessions.
  const drafts = new Map<string, string>()
  let shownFor = ''
  $effect(() => {
    const id = session.id
    if (id !== shownFor) {
      if (shownFor) drafts.set(shownFor, text)
      text = drafts.get(id) ?? ''
      shownFor = id
      queueMicrotask(() => input?.focus())
    }
  })

  async function send() {
    const q = text.trim()
    if (!q || session.busy || sending) return
    sending = true
    try {
      await app.send(q)
      text = ''
      drafts.delete(session.id)
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      sending = false
      queueMicrotask(() => input?.focus())
    }
  }

  async function stop() {
    try {
      await app.backend.interrupt(session.id)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      send()
    }
  }
</script>

<div class="composer">
  <textarea
    bind:this={input}
    bind:value={text}
    use:autosize={240}
    rows="2"
    placeholder={session.busy ? t('composer.waiting') : t('composer.placeholder')}
    aria-label={t('composer.label')}
    {onkeydown}
  ></textarea>
  {#if session.busy}
    <button class="btn stop" onclick={stop} title={t('composer.stop')}>
      <Icon name="square" size={13} />{t('composer.stop')}
    </button>
  {:else}
    <button class="btn primary icon send" onclick={send} disabled={!text.trim() || sending} aria-label={t('composer.send')} title={t('composer.sendHint')}>
      <Icon name="arrow-up" />
    </button>
  {/if}
</div>

<style>
  .composer {
    display: flex;
    align-items: flex-end;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-2) var(--space-2) var(--space-4);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-sm);
    transition: border-color var(--fast) var(--ease), box-shadow var(--fast) var(--ease);
  }
  .composer:focus-within {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
  }
  textarea {
    flex: 1;
    min-height: 56px;
    padding: 7px 0;
    border: 0;
    outline: none;
    resize: none;
    background: transparent;
    font-size: var(--text-md);
    line-height: 1.5;
  }
  textarea::placeholder {
    color: var(--text-faint);
  }
  .send {
    width: 34px;
    height: 34px;
    border-radius: 10px;
  }
  .stop {
    height: 34px;
    border-radius: 10px;
  }
</style>
