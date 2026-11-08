// A server this PC is paired with, reached through the bridge (decision
// 0031): UNCLI's Go side holds the token, makes the calls and keeps the
// server's event stream open, so the window never fetches another
// server. Calls and events are the remote backend's (remote.ts).
import { dispatchEvent, remoteBackend, RPCError } from './remote'
import type { AttachmentRef, Backend, PeerState, Upload } from './types'

export interface Bridge {
  call<T>(method: string, ...args: unknown[]): Promise<T>
  on(name: string, cb: (data: unknown) => void): () => void
}

interface PeerEvent {
  server: string
  name: string
  data: unknown
}

// A call the server's app refused comes back through the bridge as its
// message; keep it the app's, as on the phone.
async function relay<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } catch (e) {
    throw new RPCError(e instanceof Error ? e.message : String(e), 0)
  }
}

// peerBackend is the server key's backend, its session ids its own (bare).
// base is its address, for signed links.
export function peerBackend(bridge: Bridge, key: string, base: () => string): Backend {
  const upload = (path: string) => relay(bridge.call<Upload>('PeerUpload', key, path))
  const call = <T>(method: string, ...params: unknown[]) => relay(bridge.call<T>('PeerCall', key, method, params))
  return remoteBackend({
    base,
    call,
    // UNCLI's Go side gives each call its own deadline.
    callWithin: (_ms, method, ...params) => call(method, ...params),
    subscribe(h) {
      const offs = [
        bridge.on('peer:event', d => {
          const e = d as PeerEvent
          if (e.server !== key) return
          // Any (re)connect fetches afresh: while away, the server's
          // sessions may have changed in ways the list didn't follow.
          if (e.name === 'hello') h.resync?.()
          else dispatchEvent(e.name, e.data, h)
        }),
        bridge.on('peer:status', d => {
          const s = (d as PeerState[]).find(p => p.key === key)
          if (!s) return
          const lastOnline = s.lastOnline ? Date.parse(s.lastOnline) : null
          h.connection?.({ state: s.state === 'online' || s.state === 'connecting' ? s.state : 'offline', lastOnline, error: s.error })
        }),
      ]
      return () => offs.forEach(off => off())
    },
    // The window's files are on this PC: Go reads and uploads them.
    upload: () => Promise.reject(new Error('attach files from this PC by path')),
    async attach(refs: AttachmentRef[]) {
      return Promise.all(refs.map(async r => {
        if (!r.path) return r
        const up = await upload(r.path)
        return { upload: up.id, name: up.name, mediaType: up.mediaType }
      }))
    },
  })
}
