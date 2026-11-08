// This PC's sessions and its paired servers' side by side (decision 0031):
// one Backend for the window's store. A call about a session goes to the
// session's server (its id says which, identity.ts); the session list,
// pins and search cover every server that answers; everything about the
// machine (CLIs, providers, preferences, containers, usage, transcripts)
// stays this PC's. New session on a server uses that server's own backend,
// peer(key).
import { LOCAL, SESSION_FIRST, parseRef } from './identity'
import type { Backend, Bootstrap, Handlers, PeerState, PinnedPage, SearchQuery, SearchResult } from './types'

// How long the session list waits for a server: one slow to answer joins
// when its stream next connects (a resync).
const PEER_WAIT_MS = 5000

function within<T>(p: Promise<T>, ms: number): Promise<T | null> {
  return Promise.race([p.catch(() => null), new Promise<null>(resolve => setTimeout(() => resolve(null), ms))])
}

export function combinedBackend(local: Backend): Backend {
  let servers: PeerState[] = []
  const reachable = () => servers.filter(s => s.state === 'online')
  const route = (key: string): Backend => {
    const { server } = parseRef(key)
    return server === LOCAL ? local : local.peer(server)
  }
  // fromAll asks this PC and every server online; a server that fails or
  // is slow is left out.
  async function fromAll<T>(ask: (b: Backend) => Promise<T>): Promise<[T, T[]]> {
    const [own, others] = await Promise.all([ask(local), Promise.all(reachable().map(s => within(ask(local.peer(s.key)), PEER_WAIT_MS)))])
    return [own, others.filter((v): v is Awaited<T> => v !== null)]
  }

  const out: Record<string, unknown> = { ...local }
  for (const name of SESSION_FIRST) {
    out[name] = (id: string, ...rest: unknown[]) => (route(id)[name] as (...a: unknown[]) => unknown)(id, ...rest)
  }

  const combined: Partial<Backend> = {
    // This PC's notifications follow what's on screen, a server's session
    // too (app.PeerNotes); the server marks its session read.
    async focus(id: string) {
      if (parseRef(id).server === LOCAL) return local.focus(id)
      await Promise.all([local.focus(id), route(id).focus(id)])
    },
    async bootstrap(): Promise<Bootstrap> {
      servers = await local.remoteServers()
      const [boot, others] = await fromAll(b => b.bootstrap())
      return { ...boot, sessions: [...boot.sessions, ...others.flatMap(b => b.sessions)] }
    },
    async pinned(): Promise<PinnedPage[]> {
      const [own, others] = await fromAll(b => b.pinned())
      return [...own, ...others.flat()].sort((a, b) => b.startedAt - a.startedAt)
    },
    async search(q: SearchQuery): Promise<SearchResult> {
      if (q.sessionId) return route(q.sessionId).search(q)
      const [own, others] = await fromAll(b => b.search(q))
      return { ...own, hits: [...own.hits, ...others.flatMap(r => r.hits)] }
    },
    subscribe(h: Handlers) {
      // A server's sessions and pages, never its machine's state: the
      // CLIs and containers shown are this PC's.
      const sessionsOnly: Handlers = {
        sessionEvent: h.sessionEvent, sessionChanged: h.sessionChanged, pageChanged: h.pageChanged,
        cliProgress: () => {}, cliStatus: () => {}, resync: h.resync,
      }
      const subs = new Map<string, () => void>()
      let stopped = false
      const follow = (list: PeerState[]) => {
        if (stopped) return
        servers = list
        const keys = new Set(list.filter(s => s.state !== 'unpaired').map(s => s.key))
        let gone = false
        for (const [key, off] of subs) {
          if (keys.has(key)) continue
          off()
          subs.delete(key)
          gone = true
        }
        for (const key of keys) if (!subs.has(key)) subs.set(key, local.peer(key).subscribe(sessionsOnly))
        if (gone) h.resync?.() // its sessions leave the list
      }
      const offLocal = local.subscribe({ ...h, peersChanged: l => { follow(l); h.peersChanged?.(l) } })
      void local.remoteServers().then(follow, () => {})
      return () => {
        stopped = true
        offLocal()
        subs.forEach(off => off())
      }
    },
  }
  return { ...out, ...combined } as Backend
}
