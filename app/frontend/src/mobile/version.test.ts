import { describe, expect, it } from 'vitest'
import { PROTOCOL, fetchVersion, mismatched } from './version'

describe('server version', () => {
  it('reads /version and spots another protocol', async () => {
    const ok = async () => new Response(JSON.stringify({ app: '0.3.0', protocol: PROTOCOL, capabilities: ['uploads'] }))
    const v = await fetchVersion(ok as never)
    expect(v).toMatchObject({ app: '0.3.0', protocol: PROTOCOL })
    expect(mismatched(v)).toBe(false)
    expect(mismatched({ ...v!, protocol: PROTOCOL + 1 })).toBe(true)
    expect(await fetchVersion((async () => new Response('<html>')) as never)).toBeNull()
    expect(await fetchVersion((async () => { throw new TypeError('offline') }) as never)).toBeNull()
    expect(mismatched(null)).toBe(false)
  })
})
