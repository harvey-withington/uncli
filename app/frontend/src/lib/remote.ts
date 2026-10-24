// Remote access (decision 0021): what Settings says about its state, as a
// locale key, its values and a tone.
import type { RemoteStatus } from './api'

export type RemoteTone = 'ok' | 'warn' | 'danger' | 'muted'

export interface RemoteLine {
  key: string
  params: Record<string, string | number>
  tone: RemoteTone
}

// remoteLine is the status line: off, on at an address, on this PC only
// (and why not the tailnet), or not listening (and why).
export function remoteLine(s: RemoteStatus): RemoteLine {
  const params = { port: s.port, url: s.url ?? '', loopback: s.loopback ?? '', detail: s.detail ?? '' }
  switch (s.state) {
    case 'on':
      return { key: 'remote.state.on', params, tone: 'ok' }
    case 'loopback':
      return { key: `remote.reason.${s.reason ?? 'no-tailscale'}`, params, tone: 'warn' }
    case 'error':
      return { key: `remote.reason.${s.reason ?? 'listen'}`, params, tone: 'danger' }
  }
  return { key: 'remote.state.off', params, tone: 'muted' }
}

// validPort says whether a typed port can be used.
export function validPort(v: string): number | null {
  if (!/^\d{1,5}$/.test(v.trim())) return null
  const n = Number(v)
  return n >= 1 && n <= 65535 ? n : null
}
