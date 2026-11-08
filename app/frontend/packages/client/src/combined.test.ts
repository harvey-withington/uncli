import { describe, expect, it, vi } from 'vitest'
import { combinedBackend } from './combined'
import { withServer } from './identity'
import { mockBackend } from './mock'
import type { Backend, Handlers, PeerState, SessionView } from './types'

const RIPPED = 'ripped.tail0.ts.net:9871'
const STUDIO = 'studio.tail0.ts.net:9871'

const peerState = (key: string, state: PeerState['state']): PeerState =>
  ({ key, url: `https://${key}`, name: key.split('.')[0]!, device: 'd1', state })

// rig: this PC (the mock) paired with ripped (online, its own sessions)
// and studio (offline); each server's raw handlers are kept to play its
// events.
function rig(servers = [peerState(RIPPED, 'online'), peerState(STUDIO, 'offline')]) {
  const local = mockBackend()
  const raw: Record<string, Handlers> = {}
  let localH: Handlers | null = null
  const inner: Record<string, Backend> = {}
  const peers: Record<string, Backend> = {}
  const peerOf = (key: string) => {
    const b = (inner[key] ??= mockBackend({ server: key }))
    b.subscribe = h => { raw[key] = h; return () => { delete raw[key] } }
    return (peers[key] ??= withServer({ ...b, kind: 'http' }, key))
  }
  const base: Backend = {
    ...local,
    remoteServers: async () => servers,
    peer: peerOf,
    subscribe: h => { localH = h; return () => {} },
    cliStatus: vi.fn(local.cliStatus),
  }
  return { b: combinedBackend(base), base, raw, local: () => localH!, inner }
}

const handlers = (over: Partial<Handlers> = {}): Handlers => ({
  sessionEvent: () => {}, sessionChanged: () => {}, pageChanged: () => {}, cliProgress: () => {}, cliStatus: () => {}, ...over,
})

describe('combinedBackend', () => {
  it('lists this PC’s sessions and every online server’s', async () => {
    const { b } = rig()
    const boot = await b.bootstrap()
    const ids = boot.sessions.map(s => s.id)
    expect(ids).toContain('s-code')
    expect(ids).toContain(`${RIPPED}~r-build`)
    expect(ids.some(id => id.startsWith(STUDIO))).toBe(false)
  })

  it('sends a session’s calls to its server', async () => {
    const { b } = rig()
    await b.bootstrap()
    const pages = await b.pages(`${RIPPED}~r-build`)
    expect(pages[0]?.sessionId).toBe(`${RIPPED}~r-build`)
    expect(pages[0]?.question).toMatch(/nightly build/)
    const renamed = await b.rename(`${RIPPED}~r-notes`, 'Release notes')
    expect(renamed.id).toBe(`${RIPPED}~r-notes`)
    expect((await b.pages('s-code'))[0]?.sessionId).toBe('s-code')
  })

  it('keeps the machine’s state this PC’s', async () => {
    const { b, base } = rig()
    await b.cliStatus(false)
    expect(base.cliStatus).toHaveBeenCalled()
  })

  it('searches and pins across servers', async () => {
    const { b } = rig()
    await b.bootstrap()
    const found = await b.search({ text: 'build' })
    expect(found.hits.some(h => h.sessionId === `${RIPPED}~r-build`)).toBe(true)
    const pins = await b.pinned()
    expect(pins.map(p => p.sessionId)).toContain('s-cowork')
  })

  it('passes on a server’s session events, qualified, and none of its machine’s', async () => {
    const { b, raw, inner } = rig()
    const seen: SessionView[] = []
    const cli = vi.fn()
    b.subscribe(handlers({ sessionChanged: v => seen.push(v), cliStatus: cli }))
    await vi.waitFor(() => expect(raw[RIPPED]).toBeDefined())
    const v = (await inner[RIPPED]!.bootstrap()).sessions[0]!
    raw[RIPPED]!.sessionChanged(v)
    raw[RIPPED]!.cliStatus({ installed: false, version: '', pinned: '', custom: false, loggedIn: false })
    expect(seen.map(s => s.id)).toEqual([`${RIPPED}~${v.id}`])
    expect(cli).not.toHaveBeenCalled()
  })

  it('follows servers added and removed', async () => {
    const { b, raw, local } = rig([peerState(RIPPED, 'online')])
    const resync = vi.fn()
    const changed = vi.fn()
    b.subscribe(handlers({ resync, peersChanged: changed }))
    await vi.waitFor(() => expect(raw[RIPPED]).toBeDefined())
    local().peersChanged?.([peerState(RIPPED, 'online'), peerState(STUDIO, 'connecting')])
    expect(raw[STUDIO]).toBeDefined()
    expect(resync).not.toHaveBeenCalled()
    local().peersChanged?.([peerState(STUDIO, 'online')])
    expect(raw[RIPPED]).toBeUndefined()
    expect(resync).toHaveBeenCalledTimes(1) // ripped's sessions leave the list
    expect(changed).toHaveBeenCalledTimes(2)
  })
})
