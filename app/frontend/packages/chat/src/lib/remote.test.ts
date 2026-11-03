import { describe, expect, it } from 'vitest'
import { countdown, remoteLine, validPort } from './remote'

describe('remoteLine', () => {
  it('says off, on, or why not', () => {
    expect(remoteLine({ enabled: false, port: 9871, state: 'off' })).toMatchObject({ key: 'remote.state.off', tone: 'muted' })
    const on = remoteLine({ enabled: true, port: 9871, state: 'on', url: 'https://a.b.ts.net:9871', loopback: 'http://127.0.0.1:9871' })
    expect(on).toMatchObject({ key: 'remote.state.on', tone: 'ok', params: { url: 'https://a.b.ts.net:9871' } })
    expect(remoteLine({ enabled: true, port: 1, state: 'loopback', reason: 'https-off' })).toMatchObject({ key: 'remote.reason.https-off', tone: 'warn' })
    expect(remoteLine({ enabled: true, port: 1, state: 'error', reason: 'listen', detail: 'in use' })).toMatchObject({ key: 'remote.reason.listen', tone: 'danger', params: { detail: 'in use' } })
  })
})

describe('validPort', () => {
  it('takes 1 to 65535 only', () => {
    expect(validPort('9871')).toBe(9871)
    expect(validPort(' 80 ')).toBe(80)
    expect(validPort('0')).toBeNull()
    expect(validPort('65536')).toBeNull()
    expect(validPort('98a')).toBeNull()
    expect(validPort('')).toBeNull()
  })
})

describe('countdown', () => {
  it('counts down to the expiry, then stops', () => {
    const now = Date.parse('2026-10-24T12:00:00Z')
    expect(countdown('2026-10-24T12:04:32Z', now)).toBe('4:32')
    expect(countdown('2026-10-24T12:00:05Z', now)).toBe('0:05')
    expect(countdown('2026-10-24T12:00:00Z', now)).toBeNull()
    expect(countdown('not a date', now)).toBeNull()
  })
})
