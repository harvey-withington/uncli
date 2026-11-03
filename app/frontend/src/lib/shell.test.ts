import { afterEach, describe, expect, it, vi } from 'vitest'
import { RPCError } from './api'
import { Shell } from './shell.svelte'

afterEach(() => {
  vi.useRealTimers()
})

describe('toasts', () => {
  it('leave off the error’s type name', () => {
    const shell = new Shell()
    shell.toast(String(new RPCError('Couldn’t reach the server. Check the connection and try again.', 0)), 'error')
    shell.toast(String(new TypeError('Failed to fetch')), 'error')
    shell.toast(String(new Error('no such session')), 'error')
    expect(shell.toasts.map(t => t.message)).toEqual(['Couldn’t reach the server. Check the connection and try again.', 'Failed to fetch', 'no such session'])
  })

  it('keep an error until dismissed, but let a passing network blip fade', () => {
    vi.useFakeTimers()
    const shell = new Shell()
    shell.toast('Error: no such session', 'error')
    shell.toast(String(new RPCError('Couldn’t reach the server. Check the connection and try again.', 0)), 'error')
    shell.toast('Copied')
    vi.advanceTimersByTime(4000)
    expect(shell.toasts.map(t => t.message)).toEqual(['no such session', 'Couldn’t reach the server. Check the connection and try again.'])
    vi.advanceTimersByTime(3000)
    expect(shell.toasts.map(t => t.message)).toEqual(['no such session'])
  })

  it('belong to their own root: two shells don’t share toasts', () => {
    const a = new Shell()
    const b = new Shell()
    a.toast('from a')
    expect(a.toasts.map(t => t.message)).toEqual(['from a'])
    expect(b.toasts).toEqual([])
  })
})

describe('confirm', () => {
  it('answers with what was chosen, and a new request cancels one still open', async () => {
    const shell = new Shell()
    const first = shell.confirm({ title: 'One', message: '', confirmLabel: 'OK' })
    const second = shell.confirm({ title: 'Two', message: '', confirmLabel: 'OK' })
    expect(await first).toBe(false)
    expect(shell.confirming?.title).toBe('Two')
    shell.settle(true)
    expect(await second).toBe(true)
    expect(shell.confirming).toBeNull()
  })
})
