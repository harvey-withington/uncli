import { afterEach, describe, expect, it, vi } from 'vitest'
import { RPCError } from './api/http'
import { dismissToast, showToast, toasts } from './toasts.svelte'

afterEach(() => {
  for (const t of [...toasts.list]) dismissToast(t.id)
  vi.useRealTimers()
})

describe('toasts', () => {
  it('leave off the error’s type name', () => {
    showToast(String(new RPCError('Couldn’t reach the server. Check the connection and try again.', 0)), 'error')
    showToast(String(new TypeError('Failed to fetch')), 'error')
    showToast(String(new Error('no such session')), 'error')
    expect(toasts.list.map(t => t.message)).toEqual(['Couldn’t reach the server. Check the connection and try again.', 'Failed to fetch', 'no such session'])
  })

  it('keep an error until dismissed, but let a passing network blip fade', () => {
    vi.useFakeTimers()
    showToast('Error: no such session', 'error')
    showToast(String(new RPCError('Couldn’t reach the server. Check the connection and try again.', 0)), 'error')
    showToast('Copied')
    vi.advanceTimersByTime(4000)
    expect(toasts.list.map(t => t.message)).toEqual(['no such session', 'Couldn’t reach the server. Check the connection and try again.'])
    vi.advanceTimersByTime(3000)
    expect(toasts.list.map(t => t.message)).toEqual(['no such session'])
  })
})
