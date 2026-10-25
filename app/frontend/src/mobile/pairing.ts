// The phone's side of pairing (decision 0022): the code arrives in the
// link's fragment (#pair=CODE), the phone asks with its name and waits for
// Allow on the server, then keeps its own token. The token stays in this
// origin's storage and only ever goes in an Authorization header.
import { t } from '../lib/i18n.svelte'

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
  if (/Android/.test(ua)) return t('pair.guess.android')
  if (/iPhone/.test(ua)) return t('pair.guess.iphone')
  if (/iPad/.test(ua)) return t('pair.guess.ipad')
  if (/Windows/.test(ua)) return t('pair.guess.windows')
  if (/Mac OS X/.test(ua)) return t('pair.guess.mac')
  return t('pair.guess.phone')
}

export class PairError extends Error {
  constructor(message: string, readonly kind: 'code' | 'denied' | 'gone' | 'network') {
    super(message)
    this.name = 'PairError'
  }
}

// A request waiting for Allow is kept, so the phone carries on waiting
// for it after the page was reloaded or put away (switching to another
// app to tell someone at the PC, say). The server keeps it 5 minutes.
const PENDING_KEY = 'uncli-pair-pending'
const PENDING_LIFE = 5 * 60_000

export interface PendingPair {
  id: string
  claim: string
  at: number // ms
}

export function loadPending(now = Date.now()): PendingPair | null {
  try {
    const p = JSON.parse(localStorage.getItem(PENDING_KEY) ?? 'null') as PendingPair | null
    if (p && typeof p.id === 'string' && typeof p.claim === 'string' && now - p.at < PENDING_LIFE) return p
  } catch {
    // none kept
  }
  clearPending()
  return null
}

function savePending(p: PendingPair) {
  try {
    localStorage.setItem(PENDING_KEY, JSON.stringify(p))
  } catch {
    // not kept: a reload while waiting means pairing again
  }
}

function clearPending() {
  try {
    localStorage.removeItem(PENDING_KEY)
  } catch {
    // nothing kept
  }
}

async function post(f: typeof fetch, url: string, body: unknown, signal?: AbortSignal): Promise<Response> {
  try {
    return await f(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal })
  } catch (e) {
    throw new PairError(e instanceof Error ? e.message : String(e), 'network')
  }
}

// requestPair asks to pair with code under name; the request is kept until
// it's answered.
export async function requestPair(f: typeof fetch, base: string, code: string, name: string, now = Date.now()): Promise<PendingPair> {
  const res = await post(f, `${base}/pair/request`, { code, name })
  const req = (await res.json()) as { id?: string; claim?: string; error?: string }
  if (!res.ok || !req.id || !req.claim) throw new PairError(req.error ?? res.statusText, 'code')
  const p = { id: req.id, claim: req.claim, at: now }
  savePending(p)
  return p
}

// claimPair waits for the server's answer to a request: the token when
// allowed, a PairError when refused or gone. A dropped connection is
// retried while the request can still be answered.
export async function claimPair(f: typeof fetch, base: string, p: PendingPair, opts: { signal?: AbortSignal; retryMs?: number } = {}): Promise<Paired> {
  for (;;) {
    let r: Response
    try {
      r = await post(f, `${base}/pair/claim`, { id: p.id, claim: p.claim }, opts.signal)
    } catch (e) {
      if (opts.signal?.aborted || Date.now() - p.at > PENDING_LIFE) {
        clearPending()
        throw e
      }
      await new Promise(res => setTimeout(res, opts.retryMs ?? 2000))
      continue
    }
    if (r.status === 202) continue
    clearPending()
    const body = (await r.json()) as Paired & { error?: string }
    if (r.ok && body.token) return { token: body.token, device: body.device }
    throw new PairError(body.error ?? r.statusText, r.status === 403 ? 'denied' : 'gone')
  }
}

// pair asks and waits; waiting is told once the server's window is asking.
export async function pair(f: typeof fetch, base: string, code: string, name: string, waiting: () => void, signal?: AbortSignal): Promise<Paired> {
  const p = await requestPair(f, base, code, name)
  waiting()
  return claimPair(f, base, p, { signal })
}
