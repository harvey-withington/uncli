import { describe, expect, it } from 'vitest'
import type { PeerState, SessionView } from './api'
import { groupSessions, onServer, serverOf } from './servers'

const RIPPED = 'ripped.tail0.ts.net:9871'
const STUDIO = 'studio.tail0.ts.net:9871'
const peer = (key: string): PeerState => ({ key, url: `https://${key}`, name: key.split('.')[0]!, device: 'd', state: 'online' })
const s = (id: string) => ({ id }) as SessionView

describe('server groups', () => {
  const list = [s('a'), s(`${RIPPED}~r1`), s('b'), s(`${RIPPED}~r2`)]

  it('groups sessions by server, each paired server in its order, empty or not', () => {
    const g = groupSessions(list, [peer(STUDIO), peer(RIPPED)])
    expect(g.local.map(x => x.id)).toEqual(['a', 'b'])
    expect(g.servers.map(x => [x.peer.name, x.sessions.length])).toEqual([['studio', 0], ['ripped', 2]])
  })

  it('moves a session among its own server’s', () => {
    expect(serverOf(`${RIPPED}~r1`)).toBe(RIPPED)
    expect(onServer(list, `${RIPPED}~r2`).map(x => x.id)).toEqual([`${RIPPED}~r1`, `${RIPPED}~r2`])
    expect(onServer(list, 'b').map(x => x.id)).toEqual(['a', 'b'])
  })
})
