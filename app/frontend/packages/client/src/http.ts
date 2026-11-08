// The HTTP backend: UNCLI on another machine reached over its remote
// access server (decision 0021), or a host's own UNCLI (decision 0027);
// the remote backend (remote.ts) over fetch. Calls are JSON-RPC to /rpc;
// events come from /events, read with fetch so the device's token stays
// in the Authorization header (decision 0022), and resume after a dropped
// connection from the last one seen.
import { dispatchEvent, remoteBackend, RPCError } from './remote'
import { text } from './text'
import type { Backend, Connection, Handlers, Upload } from './types'

export { absolute, RemoteOnlyError, RPCError } from './remote'

export interface HttpOptions {
  base: string // the server's origin; '' for the page's own; a path (/uncli) for a host's own UNCLI on the page's server
  token: () => string | null
  // unauthorized: the server refused the token (revoked, or never paired).
  unauthorized?: () => void
  // setup: the server is a host's own UNCLI (decision 0027), which lets its
  // page install the CLIs and sign in to them; a phone can't.
  setup?: boolean
  // headers: what the host's server needs on every request to reach its
  // UNCLI, its own sign-in say (Authorization: Bearer <the host's token>).
  headers?: () => Record<string, string> | null | undefined
  fetch?: typeof fetch
}

// How long to wait before reconnecting, by attempt; a stream that says
// nothing (the server pings every 20 s) for STALL_MS is given up on. The
// first half minute retries every few seconds, so a PC that restarted (an
// update takes about 30 s) is back on the phone soon after it is.
const BACKOFF_MS = [500, 1000, 2000, 3000, 3000, 3000, 3000, 5000, 10000, 15000]
const STALL_MS = 45_000

export interface SSEFrame {
  id: string
  event: string
  data: string
}

// parseSSE splits server-sent event text into frames; rest is an
// incomplete frame to carry into the next chunk.
export function parseSSE(text: string): { frames: SSEFrame[]; rest: string } {
  const frames: SSEFrame[] = []
  const parts = text.replace(/\r\n/g, '\n').split('\n\n')
  const rest = parts.pop() ?? ''
  for (const part of parts) {
    const f: SSEFrame = { id: '', event: 'message', data: '' }
    const data: string[] = []
    for (const line of part.split('\n')) {
      if (!line || line.startsWith(':')) continue
      const i = line.indexOf(':')
      const field = i < 0 ? line : line.slice(0, i)
      const value = i < 0 ? '' : line.slice(i + 1).replace(/^ /, '')
      if (field === 'id') f.id = value
      else if (field === 'event') f.event = value
      else if (field === 'data') data.push(value)
    }
    if (data.length) {
      f.data = data.join('\n')
      frames.push(f)
    }
  }
  return { frames, rest }
}

// replyOf reads a JSON reply. Anything else (a proxy's error page, a
// server cut off mid-reply) is an RPCError saying what came back, not a
// parser's message.
async function replyOf<T>(res: Response): Promise<T> {
  if (!(res.headers.get('Content-Type') ?? '').includes('application/json')) {
    throw new RPCError(text('remote.badReply', { status: `${res.status} ${res.statusText}`.trim() }), res.status)
  }
  try {
    return (await res.json()) as T
  } catch {
    throw new RPCError(text('remote.badReply', { status: `${res.status} ${res.statusText}`.trim() }), res.status)
  }
}

export function httpBackend(opts: HttpOptions): Backend {
  const raw = opts.fetch ?? ((...a: Parameters<typeof fetch>) => fetch(...a))
  // A request that never reached the server (the network not back yet on
  // a phone just woken) says so, not the browser's "Failed to fetch".
  const f = async (...a: Parameters<typeof fetch>): Promise<Response> => {
    try {
      return await raw(...a)
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') throw e
      throw new RPCError(text('remote.unreachable'), 0)
    }
  }
  const auth = (): Record<string, string> => {
    const token = opts.token()
    return { ...opts.headers?.(), ...(token ? { Authorization: `Bearer ${token}` } : {}) }
  }
  let id = 0

  const unpaired = (): never => {
    opts.unauthorized?.()
    throw new RPCError(text('remote.unpaired'), 401)
  }

  // A call given a deadline (the state the app opens with) that hasn't
  // been answered by then is taken as not reaching the server: a PC whose
  // network is down otherwise leaves it waiting on the browser's own
  // timeout, minutes on a phone.
  async function callWithin<T>(deadlineMs: number, method: string, ...params: unknown[]): Promise<T> {
    const ctl = deadlineMs ? new AbortController() : null
    const timer = ctl ? setTimeout(() => ctl.abort(), deadlineMs) : undefined
    let res: Response
    try {
      res = await f(`${opts.base}/rpc`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...auth() },
        body: JSON.stringify({ jsonrpc: '2.0', id: ++id, method, params }),
        signal: ctl?.signal,
      })
    } catch (e) {
      if (ctl?.signal.aborted) throw new RPCError(text('remote.unreachable'), 0)
      throw e
    } finally {
      clearTimeout(timer)
    }
    if (res.status === 401) unpaired()
    const body = await replyOf<{ result?: T; error?: { code: number; message: string } }>(res)
    if (body.error) throw new RPCError(body.error.message, body.error.code)
    return body.result as T
  }

  return remoteBackend({
    base: () => opts.base,
    call: (method, ...params) => callWithin(0, method, ...params),
    callWithin,
    canSetUp: !!opts.setup,
    subscribe: h => stream(opts, raw, auth, h), // its banner says it can't reach the server itself
    async upload(file: Blob, name: string): Promise<Upload> {
      const res = await f(`${opts.base}/upload?name=${encodeURIComponent(name)}`, { method: 'POST', headers: auth(), body: file })
      if (res.status === 401) unpaired()
      const body = await replyOf<Upload & { error?: string }>(res)
      if (!res.ok) throw new RPCError(body.error ?? res.statusText, res.status)
      return body
    },
  })
}

const mounted = new Map<string, Backend>()
let hostHeaders: (() => Record<string, string> | null | undefined) | null = null

// setHostHeaders gives every mounted backend on the page what the host's
// server needs on each request: its own sign-in, for a host that signs in
// with a token rather than a cookie. Asked for on every request, so a
// renewed token is used at once.
export function setHostHeaders(h: (() => Record<string, string> | null | undefined) | null) {
  hostHeaders = h
}

// mountedBackend is a host's own UNCLI at a path of the host's server
// (decision 0027): <uncli-chat src="/uncli">, or a full address when the
// page comes from elsewhere (a desktop app's webview). The host's server
// forwards the path and adds UNCLI's key, so the page sends none; it sends
// the host's own headers (setHostHeaders). Every element with the same src
// shares one backend, and so one client.
export function mountedBackend(src: string): Backend {
  const base = src.replace(/\/+$/, '')
  let b = mounted.get(base)
  if (!b) {
    b = httpBackend({ base, token: () => null, setup: true, headers: () => hostHeaders?.() })
    mounted.set(base, b)
  }
  return b
}

// stream keeps the event stream open: it reconnects with backoff, resumes
// from the last event seen, asks the store to resync when the server says
// events were lost, and reports the connection. Coming back to the page
// (a phone waking) or the network returning reconnects at once.
function stream(opts: HttpOptions, f: typeof fetch, auth: () => Record<string, string>, h: Handlers): () => void {
  let cursor = ''
  let stopped = false
  let attempt = 0
  let lastOnline: number | null = null
  let abort: AbortController | null = null
  let wake: (() => void) | null = null
  let heard = 0 // when the stream last said anything
  const report = (c: Connection) => h.connection?.(c)

  const dispatch = (fr: SSEFrame) => {
    if (fr.id) cursor = fr.id
    let data: unknown
    try {
      data = JSON.parse(fr.data)
    } catch {
      return
    }
    switch (fr.event) {
      case 'hello': {
        const hello = data as { reload: boolean; cursor: string }
        cursor = hello.cursor
        lastOnline = Date.now()
        attempt = 0
        report({ state: 'online', lastOnline })
        if (hello.reload) h.resync?.()
        return
      }
      default: return dispatchEvent(fr.event, data, h)
    }
  }

  async function once(): Promise<'retry' | 'stop'> {
    abort = new AbortController()
    const ctl = abort
    let stall: ReturnType<typeof setTimeout> | undefined
    const arm = () => {
      heard = Date.now()
      clearTimeout(stall)
      stall = setTimeout(() => ctl.abort(), STALL_MS)
    }
    try {
      arm()
      const res = await f(`${opts.base}/events`, {
        headers: { ...auth(), ...(cursor ? { 'Last-Event-ID': cursor } : {}) },
        signal: ctl.signal,
        cache: 'no-store',
      })
      if (res.status === 401) {
        opts.unauthorized?.()
        return 'stop'
      }
      if (!res.ok || !res.body) throw new Error(res.statusText || `HTTP ${res.status}`)
      const reader = res.body.getReader()
      const dec = new TextDecoder()
      let buf = ''
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        arm()
        buf += dec.decode(value, { stream: true })
        const { frames, rest } = parseSSE(buf)
        buf = rest
        frames.forEach(dispatch)
      }
      throw new Error(text('remote.closed'))
    } catch (e) {
      if (stopped) return 'stop'
      report({ state: 'offline', lastOnline, error: e instanceof Error ? e.message : String(e) })
      return 'retry'
    } finally {
      clearTimeout(stall)
    }
  }

  // now reconnects at once: while waiting to retry, or when the stream
  // has gone quiet for longer than a ping (it died while the phone slept).
  const now = () => {
    if (wake) wake()
    else if (Date.now() - heard > 25_000) abort?.abort()
  }
  const onVisible = () => {
    if (document.visibilityState === 'visible') now()
  }
  globalThis.addEventListener?.('online', now)
  globalThis.document?.addEventListener?.('visibilitychange', onVisible)

  void (async () => {
    while (!stopped) {
      report({ state: 'connecting', lastOnline })
      if ((await once()) === 'stop' || stopped) return
      const wait = BACKOFF_MS[Math.min(attempt++, BACKOFF_MS.length - 1)]
      await new Promise<void>(resolve => {
        const t = setTimeout(resolve, wait)
        wake = () => {
          clearTimeout(t)
          resolve()
        }
      })
      wake = null
    }
  })()

  return () => {
    stopped = true
    abort?.abort()
    wake?.()
    globalThis.removeEventListener?.('online', now)
    globalThis.document?.removeEventListener?.('visibilitychange', onVisible)
  }
}
