// Remote servers for the browser mock (decision 0031). ?peers=on starts
// paired with two servers: ripped, online with sessions of its own, and
// studio, offline. Adding one by its pairing link waits a moment for
// "Allow" and pairs.
import { withServer } from '../identity'
import { parsePairLink } from '../pairlink'
import type { Backend, PeerState } from '../types'

export function mockPeers(changed: (l: PeerState[]) => void, server: (key: string) => Backend) {
  const params = new URLSearchParams(globalThis.location?.search ?? '')
  const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString()
  let list: PeerState[] = params.get('peers') === 'on'
    ? [
      { key: 'ripped.tail2ebd58.ts.net:9871', url: 'https://ripped.tail2ebd58.ts.net:9871', name: 'ripped', device: 'd7', state: 'online', lastOnline: ago(0), app: '0.4.0', protocol: 1 },
      { key: 'studio.tail2ebd58.ts.net:9871', url: 'https://studio.tail2ebd58.ts.net:9871', name: 'studio', device: 'd2', state: 'offline', error: "can't reach studio: connection timed out", lastOnline: ago(180), app: '0.4.0', protocol: 1 },
    ]
    : []
  const backends: Record<string, Backend> = {}
  let adding: ((e: Error) => void) | null = null
  const tell = () => changed(structuredClone(list))
  return {
    async remoteServers() { return structuredClone(list) },
    addRemoteServer(link: string, _name: string): Promise<PeerState> {
      const base = parsePairLink(link)?.base
      if (!base) return Promise.reject(new Error('that isn’t a pairing link'))
      const key = new URL(base).host
      return new Promise((resolve, reject) => {
        adding = reject
        setTimeout(() => {
          if (adding !== reject) return
          adding = null
          const p: PeerState = { key, url: base, name: key.split('.')[0] ?? key, device: 'd9', state: 'online', lastOnline: new Date().toISOString(), app: '0.4.0', protocol: 1 }
          list = [...list.filter(s => s.key !== key), p]
          tell()
          resolve(structuredClone(p))
        }, 1500)
      })
    },
    async cancelAddRemoteServer() {
      adding?.(new Error('pairing cancelled'))
      adding = null
    },
    async removeRemoteServer(key: string) {
      list = list.filter(s => s.key !== key)
      tell()
    },
    async thisPCName() { return 'DEVIANT' },
    peer(key: string): Backend {
      return (backends[key] ??= withServer({ ...server(key), kind: 'http' }, key))
    },
  }
}
