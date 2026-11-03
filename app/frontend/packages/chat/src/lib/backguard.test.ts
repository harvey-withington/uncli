import { describe, expect, it, vi } from 'vitest'
import { hold, take, watch } from './backguard'

describe('Back guard', () => {
  it('cancels the held edit once, then lets Back go back', () => {
    const cancel = vi.fn()
    hold(cancel)
    expect(take()).toBe(true)
    expect(cancel).toHaveBeenCalledOnce()
    expect(take()).toBe(false)
  })

  it('lets Back go back once the edit is released', () => {
    const cancel = vi.fn()
    const release = hold(cancel)
    release()
    expect(take()).toBe(false)
    expect(cancel).not.toHaveBeenCalled()
  })

  it('tells the watcher when Back is held and let go, by release or by Back', () => {
    const seen: boolean[] = []
    const stop = watch(h => seen.push(h))
    hold(() => {})()
    hold(() => {})
    take()
    stop()
    hold(() => {})()
    expect(seen).toEqual([true, false, true, false])
  })

  it('ignores an older edit releasing after a newer one took over', () => {
    const older = hold(() => {})
    const newer = vi.fn()
    hold(newer)
    older()
    expect(take()).toBe(true)
    expect(newer).toHaveBeenCalledOnce()
  })
})
