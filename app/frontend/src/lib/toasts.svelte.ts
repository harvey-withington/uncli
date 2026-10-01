export type ToastKind = 'info' | 'success' | 'error'

export interface Toast {
  id: number
  message: string
  kind: ToastKind
}

let next = 1

export const toasts = $state<{ list: Toast[] }>({ list: [] })

// Errors stay until dismissed; everything else fades after a few seconds.
export function showToast(message: string, kind: ToastKind = 'info') {
  const id = next++
  toasts.list.push({ id, message: message.replace(/^Error:\s*/, ''), kind })
  if (kind !== 'error') setTimeout(() => dismissToast(id), 3500)
}

export function dismissToast(id: number) {
  toasts.list = toasts.list.filter(t => t.id !== id)
}
