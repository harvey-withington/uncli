// Remote access for the browser mock: off until switched on, then on at a
// sample tailnet address. ?remote=on starts it on; ?remote=stopped starts
// it on with Tailscale stopped (loopback only). Pairing shows a sample
// code; ?pair=ask has a phone ask to pair a moment after.
import type { PairRequest, PairedDevice, Pairing, RemoteStatus } from './types'

export interface RemoteEvents {
  status(s: RemoteStatus): void
  pair(r: PairRequest): void
  devices(d: PairedDevice[]): void
}

// A stand-in QR code: a grid of squares, enough to see the layout.
const SAMPLE_QR = 'data:image/svg+xml;base64,' + btoa(
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 21 21" shape-rendering="crispEdges"><rect width="21" height="21" fill="#fff"/>' +
  Array.from({ length: 21 * 21 }, (_, i) => ((i * 7919) % 5 < 2 ? `<rect x="${i % 21}" y="${Math.floor(i / 21)}" width="1" height="1"/>` : '')).join('') +
  '</svg>')

export function mockRemote(on: RemoteEvents) {
  const params = new URLSearchParams(globalThis.location?.search ?? '')
  const scenario = params.get('remote')
  const at = (enabled: boolean, port: number): RemoteStatus => {
    if (!enabled) return { enabled: false, port, state: 'off' }
    const loopback = `http://127.0.0.1:${port}`
    if (scenario === 'stopped') return { enabled: true, port, state: 'loopback', loopback, reason: 'stopped', detail: 'Stopped' }
    return { enabled: true, port, state: 'on', loopback, url: `https://deviant.tail2ebd58.ts.net:${port}`, certExpires: '2027-01-04T09:14:26Z' }
  }
  let status = at(scenario === 'on' || scenario === 'stopped', 9871)
  const hour = 3600_000
  let devices: PairedDevice[] = scenario === 'on'
    ? [{ id: 'd1', name: 'Pixel 9', paired: new Date(Date.now() - 50 * hour).toISOString(), lastSeen: new Date(Date.now() - 2 * hour).toISOString() }]
    : []
  const waiting = new Map<string, string>()
  return {
    async remoteStatus() { return structuredClone(status) },
    async setRemoteAccess(enabled: boolean, port: number) {
      if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error('choose a port from 1 to 65535')
      status = at(enabled, port)
      on.status(structuredClone(status))
      return structuredClone(status)
    },
    async startPairing(): Promise<Pairing> {
      if (status.state !== 'on' || !status.url) throw new Error('turn Remote access on, with Tailscale connected, to pair a device')
      const code = 'K7QWM3XRP2'
      if (params.get('pair') === 'ask') {
        setTimeout(() => { waiting.set('r1', 'Harvey’s Pixel'); on.pair({ id: 'r1', name: 'Harvey’s Pixel' }) }, 800)
      }
      return { code, url: `${status.url}/m/#pair=${code}`, qr: SAMPLE_QR, expires: new Date(Date.now() + 5 * 60_000).toISOString() }
    },
    async cancelPairing() {},
    async answerPairing(id: string, allow: boolean) {
      const name = waiting.get(id)
      if (!name) throw new Error('no such pairing request')
      waiting.delete(id)
      if (allow) {
        const now = new Date().toISOString()
        devices = [{ id: `d${devices.length + 2}`, name, paired: now, lastSeen: now }, ...devices]
        on.devices(structuredClone(devices))
      }
    },
    async pairedDevices() { return structuredClone(devices) },
    async revokeDevice(id: string) {
      devices = devices.filter(d => d.id !== id)
      on.devices(structuredClone(devices))
    },
  }
}
