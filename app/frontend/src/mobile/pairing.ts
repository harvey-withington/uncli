// The phone's side of pairing (decision 0022): the code arrives in the
// link's fragment (#pair=CODE), the phone asks with its name and waits for
// Allow on the server, then keeps its own token. The token stays in this
// origin's storage and only ever goes in an Authorization header.

const TOKEN_KEY = 'uncli-device-token'
const NAME_KEY = 'uncli-device-name'

export interface Paired {
  token: string
  device: { id: string; name: string }
}

export function loadToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

export function saveToken(token: string | null) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch {
    // not kept: the device pairs again next time
  }
}

export function loadDeviceName(): string {
  try {
    return localStorage.getItem(NAME_KEY) ?? ''
  } catch {
    return ''
  }
}

export function saveDeviceName(name: string) {
  try {
    localStorage.setItem(NAME_KEY, name)
  } catch {
    // only a convenience
  }
}

// pairCodeFromHash reads the code from a link's fragment.
export function pairCodeFromHash(hash: string): string {
  const m = /(?:^#|&)pair=([A-Za-z0-9]{4,20})(?:&|$)/.exec(hash)
  return m?.[1]?.toUpperCase() ?? ''
}

// normaliseCode is a typed pairing code as the server knows it: letters
// and digits only, upper case; '' until it's a whole code.
export function normaliseCode(typed: string): string {
  const c = typed.toUpperCase().replace(/[^A-Z0-9]/g, '')
  return c.length === 10 ? c : ''
}

// sessionFromHash reads the session a link opens (#s=<id>), from a
// notification.
export function sessionFromHash(hash: string): string {
  const m = /(?:^#|&)s=([^&]+)/.exec(hash)
  return m?.[1] ? decodeURIComponent(m[1]) : ''
}

// guessDeviceName is a starting name for the device, from its browser.
export function guessDeviceName(ua: string): string {
  const android = /Android[^;)]*;\s*([^;)]+?)(?:\s+Build\/[^;)]*)?\)/.exec(ua)
  const model = android?.[1]?.trim()
  if (model && model !== 'K') return model
  if (/Android/.test(ua)) return 'Android phone'
  if (/iPhone/.test(ua)) return 'iPhone'
  if (/iPad/.test(ua)) return 'iPad'
  if (/Windows/.test(ua)) return 'Windows PC'
  if (/Mac OS X/.test(ua)) return 'Mac'
  return 'Phone'
}

export class PairError extends Error {
  constructor(message: string, readonly kind: 'code' | 'denied' | 'gone' | 'network') {
    super(message)
    this.name = 'PairError'
  }
}

// pair asks to pair with code under name, then waits for the server's
// answer: the token when allowed, a PairError otherwise. waiting is told
// once the server's window is asking.
export async function pair(f: typeof fetch, base: string, code: string, name: string, waiting: () => void, signal?: AbortSignal): Promise<Paired> {
  const post = async (path: string, body: unknown) => {
    try {
      return await f(`${base}${path}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal })
    } catch (e) {
      throw new PairError(e instanceof Error ? e.message : String(e), 'network')
    }
  }
  const res = await post('/pair/request', { code, name })
  const req = (await res.json()) as { id?: string; claim?: string; error?: string }
  if (!res.ok || !req.id || !req.claim) throw new PairError(req.error ?? res.statusText, 'code')
  waiting()
  for (;;) {
    const r = await post('/pair/claim', { id: req.id, claim: req.claim })
    if (r.status === 202) continue
    const body = (await r.json()) as Paired & { error?: string }
    if (r.ok && body.token) return { token: body.token, device: body.device }
    throw new PairError(body.error ?? r.statusText, r.status === 403 ? 'denied' : 'gone')
  }
}
