import { describe, expect, it, vi } from 'vitest'
import { PairError, claimPair, guessDeviceName, loadPending, normaliseCode, pair, pairCodeFromHash, requestPair, sessionFromHash } from './pairing'

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status })

describe('pairing links', () => {
  it('reads the code and the session from the fragment', () => {
    expect(pairCodeFromHash('#pair=k7qwm3xrp2')).toBe('K7QWM3XRP2')
    expect(pairCodeFromHash('#x=1&pair=AB12CD34EF')).toBe('AB12CD34EF')
    expect(pairCodeFromHash('#pair=<script>')).toBe('')
    expect(pairCodeFromHash('')).toBe('')
    expect(sessionFromHash('#s=deviant.ts.net%3A9871~abc')).toBe('deviant.ts.net:9871~abc')
  })

  it('takes a typed code however it was written', () => {
    expect(normaliseCode('bc3yj-64ac7')).toBe('BC3YJ64AC7')
    expect(normaliseCode(' BC3Y J64A C7 ')).toBe('BC3YJ64AC7')
    expect(normaliseCode('BC3YJ')).toBe('')
  })

  it('guesses a name from the browser', () => {
    expect(guessDeviceName('Mozilla/5.0 (Linux; Android 15; Pixel 9 Build/AP4A) AppleWebKit')).toBe('Pixel 9')
    expect(guessDeviceName('Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36')).toBe('Android phone')
    expect(guessDeviceName('Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)')).toBe('iPhone')
  })
})

describe('pair', () => {
  it('asks, waits through the server’s long polls, and gets the token', async () => {
    const replies = [json({ id: 'r1', claim: 'c1' }), json({ error: 'waiting' }, 202), json({ token: 't'.repeat(64), device: { id: 'd1', name: 'Pixel 9' } })]
    const f = vi.fn(async () => replies.shift()!)
    const waiting = vi.fn()
    const got = await pair(f as never, '', 'CODE', 'Pixel 9', waiting)
    expect(got.token).toHaveLength(64)
    expect(waiting).toHaveBeenCalledOnce()
    const bodies = f.mock.calls.map(c => JSON.parse((c as unknown as [string, RequestInit])[1].body as string))
    expect(bodies).toEqual([{ code: 'CODE', name: 'Pixel 9' }, { id: 'r1', claim: 'c1' }, { id: 'r1', claim: 'c1' }])
  })

  it('keeps waiting through a dropped connection, and remembers the request', async () => {
    const p = await requestPair((async () => json({ id: 'r1', claim: 'c1' })) as never, '', 'CODE', 'Pixel')
    expect(loadPending()).toMatchObject({ id: 'r1', claim: 'c1' })
    let n = 0
    const f = vi.fn(async () => {
      n++
      if (n === 1) throw new TypeError('Failed to fetch') // the phone was put away
      return json({ token: 't'.repeat(64), device: { id: 'd1', name: 'Pixel' } })
    })
    const got = await claimPair(f as never, '', p, { retryMs: 1 })
    expect(got.device.name).toBe('Pixel')
    expect(loadPending()).toBeNull()
    // An old request isn't resumed.
    localStorage.setItem('uncli-pair-pending', JSON.stringify({ id: 'x', claim: 'y', at: Date.now() - 6 * 60_000 }))
    expect(loadPending()).toBeNull()
  })

  it('says why it failed', async () => {
    await expect(pair((async () => json({ error: 'bad code' }, 403)) as never, '', 'X', 'n', () => {})).rejects.toMatchObject({ kind: 'code', message: 'bad code' })
    const denied = [json({ id: 'r', claim: 'c' }), json({ error: 'no' }, 403)]
    await expect(pair((async () => denied.shift()!) as never, '', 'X', 'n', () => {})).rejects.toMatchObject({ kind: 'denied' })
    await expect(pair((async () => { throw new TypeError('offline') }) as never, '', 'X', 'n', () => {})).rejects.toBeInstanceOf(PairError)
  })
})
