<script lang="ts">
  import { useChat } from '../lib/context'
  import { t } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { useShell } from '../lib/shell.svelte'

  const shell = useShell()

  interface Props {
    text: string
    label?: string
  }

  let { text, label }: Props = $props()
  const chat = useChat()
  let copied = $state(false)
  let timer: ReturnType<typeof setTimeout> | undefined

  async function copy(e: MouseEvent) {
    e.stopPropagation()
    try {
      await chat.backend.copyText(text)
      copied = true
      clearTimeout(timer)
      timer = setTimeout(() => (copied = false), 1400)
    } catch (err) {
      shell.toast(t('copy.failed', { error: String(err) }), 'error')
    }
  }
</script>

<button class="copy btn small icon ghost" class:copied onclick={copy} aria-label={label ?? t('copy.copy')} title={copied ? t('copy.copied') : (label ?? t('copy.copy'))}>
  <Icon name={copied ? 'check' : 'copy'} size={14} />
</button>

<style>
  .copy {
    background: var(--surface);
    border-color: var(--border);
    box-shadow: var(--shadow-sm);
  }
  .copied {
    color: var(--success);
  }
</style>
