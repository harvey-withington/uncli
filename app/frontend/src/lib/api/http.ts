// The HTTP backend: UNCLI on another machine, reached over its remote
// access server (decision 0021). Calls are JSON-RPC to /rpc with the same
// names and arguments the window's bridge takes; events come from
// /events, read with fetch so the device's token stays in the
// Authorization header (decision 0022), and resume after a dropped
// connection from the last one seen. What only works at the server's own
// screen is refused here, and the UI shows it disabled with the reason
// (lib/remoteonly.ts).
import { t } from '../i18n.svelte'
import type { Backend, Connection, Handlers, Upload } from './types'

export interface HttpOptions {
  base: string // the server's origin; '' for the page's own
  token: () => string | null
  // unauthorized: the server refused the token (revoked, or never paired).
  unauthorized?: () => void
  fetch?: typeof fetch
}

// RemoteOnlyError is a call that only the server's own window can make.
export class RemoteOnlyError extends Error {
  constructor(readonly method: string) {
    super(t('remote.onlyAtServer'))
    this.name = 'RemoteOnlyError'
  }
}

// RPCError is a call the server answered with an error; its message is
// the app's, meant for the user.
export class RPCError extends Error {
  constructor(message: string, readonly code: number) {
    super(message)
    this.name = 'RPCError'
  }
}

// How long to wait before reconnecting, by attempt; a stream that says
// nothing (the server pings every 20 s) for STALL_MS is given up on.
const BACKOFF_MS = [500, 1000, 2000, 5000, 10000, 15000]
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
    throw new RPCError(t('remote.badReply', { status: `${res.status} ${res.statusText}`.trim() }), res.status)
  }
  try {
    return (await res.json()) as T
  } catch {
    throw new RPCError(t('remote.badReply', { status: `${res.status} ${res.statusText}`.trim() }), res.status)
  }
}

export function httpBackend(opts: HttpOptions): Backend {
  const f = opts.fetch ?? ((...a: Parameters<typeof fetch>) => fetch(...a))
  const auth = (): Record<string, string> => {
    const token = opts.token()
    return token ? { Authorization: `Bearer ${token}` } : {}
  }
  let id = 0

  const unpaired = (): never => {
    opts.unauthorized?.()
    throw new RPCError(t('remote.unpaired'), 401)
  }

  async function call<T>(method: string, ...params: unknown[]): Promise<T> {
    const res = await f(`${opts.base}/rpc`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth() },
      body: JSON.stringify({ jsonrpc: '2.0', id: ++id, method, params }),
    })
    if (res.status === 401) unpaired()
    const body = await replyOf<{ result?: T; error?: { code: number; message: string } }>(res)
    if (body.error) throw new RPCError(body.error.message, body.error.code)
    return body.result as T
  }

  const only = (method: string) => () => Promise.reject(new RemoteOnlyError(method))

  return {
    kind: 'http',
    subscribe(h: Handlers) {
      return stream(opts, f, auth, h)
    },
    bootstrap: () => call('Bootstrap'),
    cliStatus: fresh => call('CLIStatus', fresh),
    providerStatus: (pid, fresh) => call('ProviderStatus', pid, fresh),
    cliChannels: () => call('CLIChannels'),
    providerChannels: pid => call('ProviderChannels', pid),
    installCLI: only('installCLI'),
    signIn: only('signIn'),
    submitLoginCode: only('submitLoginCode'),
    cancelSignIn: async () => {},
    setCLIVersion: only('setCLIVersion'),
    installProvider: only('installProvider'),
    setProviderVersion: only('setProviderVersion'),
    enablePlugin: only('enablePlugin'),
    disablePlugin: only('disablePlugin'),
    signInTerminal: only('signInTerminal'),
    revealCLI: only('revealCLI'),
    startDeviceSignIn: only('startDeviceSignIn'),
    cancelDeviceSignIn: async () => {},
    pickFolder: only('pickFolder'),
    createSession: (p, d, m, c, prov, sh) => call('CreateSession', p, d, m, c, prov, sh),
    setShares: (s, sh) => call('SetShares', s, sh),
    pages: s => call('Pages', s),
    send: (s, text, attachments = []) => call('Send', s, text, attachments),
    interrupt: s => call('Interrupt', s),
    restart: s => call('Restart', s),
    setModel: (s, m) => call('SetModel', s, m),
    toggleModifier: (s, mod, on) => call('ToggleModifier', s, mod, on),
    rename: (s, title) => call('Rename', s, title),
    setSortOrder: (s, o) => call('SetSortOrder', s, o),
    describePaths: async () => [],
    describeAttachments: async () => [],
    search: q => call('Search', q),
    answerApproval: (s, r, d, scope) => call('AnswerApproval', s, r, d, scope ?? ''),
    answerAsk: (s, r, allow, answers, note) => call('AnswerAsk', s, r, allow, answers, note),
    commands: s => call('Commands', s),
    mcpSignIn: (s, server) => call('MCPSignIn', s, server),
    mcpCallback: (s, server, url) => call('MCPCallback', s, server, url),
    mcpReconnect: (s, server) => call('MCPReconnect', s, server),
    mcpSignOut: (s, server) => call('MCPSignOut', s, server),
    mcpCancel: (s, server) => call('MCPCancel', s, server),
    setUnattended: (s, on) => call('SetUnattended', s, on),
    setMode: (s, m) => call('SetMode', s, m),
    safeList: s => call('SafeList', s),
    setSafeEntry: (s, e, scope) => call('SetSafeEntry', s, e, scope),
    deleteSafeEntry: (s, e) => call('DeleteSafeEntry', s, e),
    moveSafeEntry: (s, e, scope) => call('MoveSafeEntry', s, e, scope),
    setSafeLabel: (s, e, label) => call('SetSafeLabel', s, e, label),
    markSafe: (s, r, on, scope) => call('MarkSafe', s, r, on, scope),
    teach: (s, c, v, scope) => call('Teach', s, c, v, scope),
    previewClasses: (s, c) => call('PreviewClasses', s, c),
    knownTools: s => call('KnownTools', s),
    explainCommand: (s, c) => call('ExplainCommand', s, c),
    sessionAllowlist: s => call('SessionAllowlist', s),
    clipboardFiles: async () => [],
    remove: s => call('Delete', s),
    setBookmark: (s, p, on) => call('SetBookmark', s, p, on),
    setPinned: (s, p, on) => call('SetPinned', s, p, on),
    pinned: () => call('Pinned'),
    archive: (s, on) => call('Archive', s, on),
    focus: s => call('Focus', s),
    setPreferences: only('setPreferences'),
    setDecisionKey: only('setDecisionKey'),
    hasDecisionKey: () => call('HasDecisionKey'),
    testDecisionModel: only('testDecisionModel'),
    testNotification: only('testNotification'),
    summarisePage: (s, p, b) => call('SummarisePage', s, p, b),
    usage: () => call('Usage'),
    usageReport: q => call('UsageReport', q),
    containers: () => call('Containers'),
    installWSL: only('installWSL'),
    stopContainers: only('stopContainers'),
    buildContainer: only('buildContainer'),
    removeContainer: only('removeContainer'),
    saveContainer: only('saveContainer'),
    removeContainerConfig: only('removeContainerConfig'),
    startContainerSignIn: only('startContainerSignIn'),
    finishContainerSignIn: only('finishContainerSignIn'),
    cancelContainerSignIn: async () => {},
    signOutContainers: only('signOutContainers'),
    startContainerAccountSignIn: only('startContainerAccountSignIn'),
    finishContainerAccountSignIn: only('finishContainerAccountSignIn'),
    cancelContainerAccountSignIn: async () => {},
    signOutContainerAccount: only('signOutContainerAccount'),
    // The theme file is the server's; a remote client keeps its own look.
    theme: async () => ({ path: '', found: false }),
    openFolder: only('openFolder'),
    openFile: only('openFile'),
    openPath: only('openPath'),
    revealFile: only('revealFile'),
    watchFiles: (s, q) => call('WatchFiles', s, q),
    watchRead: (s, p, since) => call('WatchRead', s, p, since),
    editors: () => call('Editors'),
    transcripts: () => call('Transcripts'),
    importTranscript: (tid, p) => call('ImportTranscript', tid, p),
    artifactFiles: s => call('ArtifactFiles', s),
    readArtifact: (s, r) => call('ReadArtifact', s, r),
    artifactPath: only('artifactPath'),
    openURL: async u => {
      if (/^https?:\/\//.test(u)) window.open(u, '_blank', 'noopener,noreferrer')
    },
    copyText: text => navigator.clipboard.writeText(text),
    remoteStatus: only('remoteStatus'),
    setRemoteAccess: only('setRemoteAccess'),
    startPairing: only('startPairing'),
    cancelPairing: async () => {},
    answerPairing: only('answerPairing'),
    pairedDevices: async () => [],
    revokeDevice: only('revokeDevice'),
    listFolder: p => call('ListFolder', p),
    async upload(file: Blob, name: string): Promise<Upload> {
      const res = await f(`${opts.base}/upload?name=${encodeURIComponent(name)}`, { method: 'POST', headers: auth(), body: file })
      if (res.status === 401) unpaired()
      const body = await replyOf<Upload & { error?: string }>(res)
      if (!res.ok) throw new RPCError(body.error ?? res.statusText, res.status)
      return body
    },
    pushKey: () => call('PushKey'),
    pushState: () => call('PushState'),
    pushSubscribe: sub => call('PushSubscribe', sub),
    pushUnsubscribe: () => call('PushUnsubscribe'),
    setPushNotifications: setting => call('SetPushNotifications', setting),
    testPush: () => call('TestPush'),
    artifactLink: async (s, r) => absolute(opts.base, await call<string>('ArtifactLink', s, r)),
    uploadLink: async up => absolute(opts.base, await call<string>('UploadLink', up)),
  }
}

// absolute is a server path as a full URL, to open in the browser.
function absolute(base: string, path: string): string {
  return new URL(path, base || globalThis.location?.origin).href
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
      case 'session:event': return h.sessionEvent(data as never)
      case 'session:changed': return h.sessionChanged(data as never)
      case 'page:changed': return h.pageChanged(data as never)
      case 'cli:progress': return h.cliProgress(data as never)
      case 'cli:status': return h.cliStatus(data as never)
      case 'provider:status': return h.providerStatus?.(data as never)
      case 'containers:changed': return h.containersChanged?.(data as never)
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
      throw new Error(t('remote.closed'))
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
