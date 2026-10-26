// Session identity is server plus id (decision 0021). This PC's sessions
// keep their bare ids; a session on another UNCLI is known by a key that
// names its server too, "<server>~<id>". withServer gives a backend's
// sessions those keys, both ways: every id it hands the UI is qualified,
// and every id the UI hands it is turned back into the server's own. So
// the stores, links and UI state, which treat a session id as an opaque
// key, key on server and id without knowing it, and a backend combining
// several servers (phase 2) is these, side by side.
import type {
  Backend, Bootstrap, CreatedSession, Handlers, ImportedSession, Page, PinnedPage, SearchQuery, SearchResult,
  SessionEventMsg, SessionView, TranscriptEntry,
} from './types'

export const LOCAL = 'local'
const SEP = '~' // never in a host name or a session id

export interface SessionRef {
  server: string
  id: string
}

export function qualify(server: string, id: string): string {
  return server === LOCAL || !id ? id : `${server}${SEP}${id}`
}

export function parseRef(key: string): SessionRef {
  const i = key.lastIndexOf(SEP)
  return i < 0 ? { server: LOCAL, id: key } : { server: key.slice(0, i), id: key.slice(i + 1) }
}

// serverKey is how a server is named in session keys: its host and port.
export function serverKey(origin: string): string {
  try {
    return new URL(origin).host
  } catch {
    return origin
  }
}

// The methods whose first argument is a session id.
export const SESSION_FIRST = [
  'setShares', 'pages', 'send', 'interrupt', 'restart', 'setModel', 'toggleModifier', 'rename', 'setSortOrder',
  'answerApproval', 'answerAsk', 'commands', 'mcpSignIn', 'mcpCallback', 'mcpReconnect', 'mcpSignOut', 'mcpCancel',
  'setUnattended', 'setMode', 'safeList', 'setSafeEntry', 'deleteSafeEntry', 'moveSafeEntry', 'setSafeLabel',
  'markSafe', 'teach', 'previewClasses', 'knownTools', 'explainCommand', 'sessionAllowlist', 'remove',
  'setBookmark', 'setPinned', 'archive', 'focus', 'summarisePage', 'openFile', 'openPath', 'watchFiles', 'watchRead',
  'artifactFiles', 'readArtifact', 'artifactPath', 'artifactLink',
] as const satisfies readonly (keyof Backend)[]

type Q = (id: string) => string

const view = (v: SessionView, q: Q): SessionView => ({ ...v, id: q(v.id) })
const page = (p: Page, q: Q): Page => ({ ...p, sessionId: q(p.sessionId) })

// What each method's result carries that names a session.
const RESULTS: Partial<Record<keyof Backend, (v: never, q: Q) => unknown>> = {
  bootstrap: (b: Bootstrap, q: Q) => ({ ...b, sessions: b.sessions.map(s => view(s, q)) }),
  createSession: (c: CreatedSession, q: Q) => ({ ...c, session: view(c.session, q) }),
  importTranscript: (c: ImportedSession, q: Q) => ({ ...c, session: view(c.session, q) }),
  transcripts: (ts: TranscriptEntry[], q: Q) => ts.map(t => (t.sessionId ? { ...t, sessionId: q(t.sessionId) } : t)),
  setShares: view, toggleModifier: view, rename: view, setSortOrder: view, setUnattended: view, setMode: view, archive: view, restart: view,
  pages: (ps: Page[], q: Q) => ps.map(p => page(p, q)),
  setBookmark: page, setPinned: page, summarisePage: page,
  pinned: (ps: PinnedPage[], q: Q) => ps.map(p => ({ ...p, sessionId: q(p.sessionId) })),
  search: (r: SearchResult, q: Q) => ({ ...r, hits: r.hits.map(h => ({ ...h, sessionId: q(h.sessionId) })) }),
}

// withServer is backend with its sessions keyed as server's.
export function withServer(backend: Backend, server: string): Backend {
  if (server === LOCAL) return backend
  const q: Q = id => qualify(server, id)
  const own = (key: string) => {
    const r = parseRef(key)
    if (r.server !== server) throw new Error(`session ${key} isn't on ${server}`)
    return r.id
  }
  const first = new Set<string>(SESSION_FIRST)
  const out: Record<string, unknown> = { ...backend }
  for (const [name, fn] of Object.entries(backend)) {
    if (typeof fn !== 'function' || name === 'subscribe') continue
    const map = RESULTS[name as keyof Backend] as ((v: unknown, q: Q) => unknown) | undefined
    out[name] = async (...args: unknown[]) => {
      if (first.has(name) && typeof args[0] === 'string') args[0] = own(args[0])
      if (name === 'search' && (args[0] as SearchQuery)?.sessionId) args[0] = { ...(args[0] as SearchQuery), sessionId: own((args[0] as SearchQuery).sessionId!) }
      const res = await (fn as (...a: unknown[]) => Promise<unknown>).apply(backend, args)
      return map && res != null ? map(res, q) : res
    }
  }
  out.subscribe = (h: Handlers) => backend.subscribe({
    ...h,
    sessionEvent: (m: SessionEventMsg) => h.sessionEvent({ ...m, sessionId: q(m.sessionId) }),
    sessionChanged: (v: SessionView) => h.sessionChanged(view(v, q)),
    pageChanged: (p: Page) => h.pageChanged(page(p, q)),
    notifyOpen: h.notifyOpen && ((id: string) => h.notifyOpen?.(q(id))),
  })
  return out as unknown as Backend
}
