// The shell: what UNCLI's components ask of whatever holds them (decision
// 0026). Each root has its own, so two chats on one page show their own
// toasts and confirm dialogs: the window's app, the phone's, and later
// each embedded element.
import { getContext, setContext } from 'svelte'
import { APP_CONTEXT, CHAT_CONTEXT } from './chatcontext'
import { t } from './i18n.svelte'
import type { ChatStore } from '../stores/chat.svelte'

export type ToastKind = 'info' | 'success' | 'error'

export interface Toast {
  id: number
  message: string
  kind: ToastKind
}

// Destructive actions confirm through ConfirmDialog, never window.confirm().
export interface ConfirmRequest {
  title: string
  message: string
  confirmLabel: string
  danger?: boolean
}

export class Shell {
  toasts = $state<Toast[]>([])
  confirming = $state<ConfirmRequest | null>(null)
  private resolve: ((ok: boolean) => void) | null = null
  private next = 1

  // Errors stay until dismissed, except a request that couldn't reach the
  // server: a passing network blip, which the connection banner already
  // shows and retries, fades like everything else. An error's type name
  // ("Error:", "RPCError:", "TypeError:") is left off.
  toast = (message: string, kind: ToastKind = 'info') => {
    const id = this.next++
    const text = message.replace(/^(?:[A-Z]\w*)?Error:\s*/, '')
    this.toasts.push({ id, message: text, kind })
    if (kind !== 'error') setTimeout(() => this.dismiss(id), 3500)
    else if (text === t('remote.unreachable')) setTimeout(() => this.dismiss(id), 6000)
  }

  dismiss = (id: number) => {
    this.toasts = this.toasts.filter(x => x.id !== id)
  }

  // A new request cancels one still open.
  confirm = (request: ConfirmRequest): Promise<boolean> => {
    this.resolve?.(false)
    return new Promise(resolve => {
      this.confirming = request
      this.resolve = resolve
    })
  }

  settle = (ok: boolean) => {
    const r = this.resolve
    this.confirming = null
    this.resolve = null
    r?.(ok)
  }
}

const SHELL_CONTEXT = Symbol('shell')

export function provideShell(shell: Shell) {
  setContext(SHELL_CONTEXT, shell)
}

// useShell is the nearest root's shell; under a chat's (or the app's)
// store, the store's.
export function useShell(): Shell {
  const store = getContext<ChatStore | undefined>(CHAT_CONTEXT) ?? getContext<ChatStore | undefined>(APP_CONTEXT)
  const s = getContext<Shell | undefined>(SHELL_CONTEXT) ?? store?.shell
  if (!s) throw new Error('useShell() outside a root')
  return s
}
