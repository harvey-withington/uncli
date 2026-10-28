import { t } from './i18n.svelte'

export type ToastKind = 'info' | 'success' | 'error'

export interface Toast {
  id: number
  message: string
  kind: ToastKind
}

let next = 1

export const toasts = $state<{ list: Toast[] }>({ list: [] })

// Errors stay until dismissed, except a request that couldn't reach the
// server: a passing network blip, which the connection banner already
// shows and retries, fades like everything else. An error's type name
// ("Error:", "RPCError:", "TypeError:") is left off.
export function showToast(message: string, kind: ToastKind = 'info') {
  const id = next++
  const text = message.replace(/^(?:[A-Z]\w*)?Error:\s*/, '')
  toasts.list.push({ id, message: text, kind })
  if (kind !== 'error') setTimeout(() => dismissToast(id), 3500)
  else if (text === t('remote.unreachable')) setTimeout(() => dismissToast(id), 6000)
}

export function dismissToast(id: number) {
  toasts.list = toasts.list.filter(t => t.id !== id)
}
