// The server's /version (decision 0021): the remote API's protocol, which
// this client was built for, and the app's version. A server on another
// protocol gets a banner; calls it doesn't know fail with its own error.

// PROTOCOL is the remote API version this client speaks (remote.Protocol
// in internal/remote/version.go).
export const PROTOCOL = 1

export interface ServerVersion {
  app: string
  protocol: number
  capabilities: string[]
  commit?: string
  commitDate?: string
}

// fetchVersion reads /version; null when the server can't be reached or
// isn't UNCLI.
export async function fetchVersion(f: typeof fetch, base = ''): Promise<ServerVersion | null> {
  try {
    const res = await f(`${base}/version`, { cache: 'no-store' })
    if (!res.ok) return null
    const v = (await res.json()) as Partial<ServerVersion>
    if (typeof v.protocol !== 'number' || typeof v.app !== 'string') return null
    return { app: v.app, protocol: v.protocol, capabilities: v.capabilities ?? [], commit: v.commit, commitDate: v.commitDate }
  } catch {
    return null
  }
}

export function mismatched(v: ServerVersion | null): boolean {
  return !!v && v.protocol !== PROTOCOL
}
