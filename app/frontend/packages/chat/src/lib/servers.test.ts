import { describe, expect, it } from 'vitest'
import type { PeerState, SessionView } from './api'
import { groupSessions, onServer, serverOf } from './servers'

const RIPPED = 'ripped.tail0.ts.net:9871'
const STUDIO = 'studio.tail0.ts.net:9871'
const peer = (key: string): PeerState => ({ key, url: `https://${key}`, name: key.split('.')[0]!, device: 'd', state: 'online' })

describe('server groups', () => {
  const s = (id: string, archived = false) => ({ id, archived }) as SessionView
  const list = [s('a'), s(`${RIPPED}~r1`), s('b'), s(`${RIPPED}~r2`), s('old', true), s(`${RIPPED}~gone`, true)]

  it('groups sessions by server, each paired server in its order, empty or not, archived apart', () => {
    const g = groupSessions(list, [peer(STUDIO), peer(RIPPED)])
    expect(g.local.sessions.map(x => x.id)).toEqual(['a', 'b'])
    expect(g.local.archived.map(x => x.id)).toEqual(['old'])
    expect(g.servers.map(x => [x.peer.name, x.sessions.length, x.archived.length])).toEqual([['studio', 0, 0], ['ripped', 2, 1]])
  })

  it('moves a session among its own server’s', () => {
    expect(serverOf(`${RIPPED}~r1`)).toBe(RIPPED)
    const active = list.filter(x => !x.archived)
    expect(onServer(active, `${RIPPED}~r2`).map(x => x.id)).toEqual([`${RIPPED}~r1`, `${RIPPED}~r2`])
    expect(onServer(active, 'b').map(x => x.id)).toEqual(['a', 'b'])
  })
})
