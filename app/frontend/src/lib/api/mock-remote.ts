// Remote access for the browser mock: off until switched on, then on at a
// sample tailnet address. ?remote=on starts it on; ?remote=stopped starts
// it on with Tailscale stopped (loopback only).
import type { RemoteStatus } from './types'

export function mockRemote(changed: (s: RemoteStatus) => void) {
  const scenario = new URLSearchParams(globalThis.location?.search ?? '').get('remote')
  const at = (on: boolean, port: number): RemoteStatus => {
    if (!on) return { enabled: false, port, state: 'off' }
    const loopback = `http://127.0.0.1:${port}`
    if (scenario === 'stopped') return { enabled: true, port, state: 'loopback', loopback, reason: 'stopped', detail: 'Stopped' }
    return { enabled: true, port, state: 'on', loopback, url: `https://deviant.tail2ebd58.ts.net:${port}`, certExpires: '2027-01-04T09:14:26Z' }
  }
  let status = at(scenario === 'on' || scenario === 'stopped', 9871)
  return {
    async remoteStatus() { return structuredClone(status) },
    async setRemoteAccess(on: boolean, port: number) {
      if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error('choose a port from 1 to 65535')
      status = at(on, port)
      changed(structuredClone(status))
      return structuredClone(status)
    },
  }
}
