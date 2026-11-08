// The sidebar's groups (decision 0031): this PC's sessions, then each
// paired server's under its name, in the order the servers were added.
import { LOCAL, parseRef, type PeerState, type SessionView } from './api'
import { t } from './i18n.svelte'

export function serverOf(sessionId: string): string {
  return parseRef(sessionId).server
}

// onServer is the sessions of list on the server sessionId is on: a
// session moves among those, each server keeping its own order.
export function onServer<T extends { id: string }>(list: readonly T[], sessionId: string): T[] {
  const server = serverOf(sessionId)
  return list.filter(s => serverOf(s.id) === server)
}

export interface ServerGroup {
  peer: PeerState
  sessions: SessionView[]
}

export function groupSessions(sessions: readonly SessionView[], servers: readonly PeerState[]): { local: SessionView[]; servers: ServerGroup[] } {
  const by = new Map<string, SessionView[]>()
  for (const s of sessions) {
    const k = serverOf(s.id)
    by.set(k, [...(by.get(k) ?? []), s])
  }
  return { local: by.get(LOCAL) ?? [], servers: servers.map(peer => ({ peer, sessions: by.get(peer.key) ?? [] })) }
}

// peerStatus is a server's state in words, for its heading's tooltip and
// Settings: online, connecting, or why not.
export function peerStatus(p: PeerState): string {
  switch (p.state) {
    case 'online': return t('peers.online')
    case 'connecting': return t('peers.connecting')
    case 'unpaired': return t('peers.unpaired')
    // The connection's own error already says it can't reach the server.
    default: return p.error ? p.error.charAt(0).toUpperCase() + p.error.slice(1) : t('peers.offline')
  }
}
