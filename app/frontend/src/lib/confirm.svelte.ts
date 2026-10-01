// Destructive actions confirm through ConfirmDialog, never window.confirm().
export interface ConfirmRequest {
  title: string
  message: string
  confirmLabel: string
  danger?: boolean
}

export const confirmState = $state<{ request: ConfirmRequest | null; resolve: ((ok: boolean) => void) | null }>({
  request: null,
  resolve: null,
})

export function confirm(request: ConfirmRequest): Promise<boolean> {
  confirmState.resolve?.(false)
  return new Promise(resolve => {
    confirmState.request = request
    confirmState.resolve = resolve
  })
}

export function settleConfirm(ok: boolean) {
  const r = confirmState.resolve
  confirmState.request = null
  confirmState.resolve = null
  r?.(ok)
}
