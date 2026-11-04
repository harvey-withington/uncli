// ChatClient: everything a UI needs from one UNCLI server, without a UI
// (decision 0026). One per backend: it holds the single event
// subscription, what the server says (its sessions, CLI status, pins…),
// each session's pages and live text, and fetches state afresh when a
// remote client missed events. Plain TypeScript: subscribe() and
// snapshot() let any stack observe it; actions return promises that
// reject on failure, and the caller decides what to show.
import { namesOf, providerOf, type ProviderNames } from '../providers'
import { orderAt } from '../reorder'
import type { Backend, CLIStatus, ModelInfo, Page, SessionEventMsg, SessionView, Share, TextDelta, ThinkingData, UsageLimit } from '../types'
import { SessionStore } from './session'
import {
  EMPTY_SESSION, INITIAL_STATE, connectedNow, sortSessions,
  type ClientState, type Notice, type SessionData,
} from './state'

// How long to wait before fetching state afresh again, by failed attempt.
const RESYNC_RETRY_MS = [1000, 3000, 8000]

type Listener = (state: ClientState) => void

export class ChatClient {
  readonly backend: Backend
  private s: ClientState = INITIAL_STATE
  private listeners = new Set<Listener>()
  private noticeListeners = new Set<(n: Notice) => void>()
  private stores = new Map<string, SessionStore>()
  private off: (() => void) | null = null
  private started: Promise<void> | null = null
  private held: (() => void)[] | null = null // changes arriving during a resync
  private resyncAgain = false
  private resyncTries = 0 // failed in a row, retried after a pause
  private resyncTimer: ReturnType<typeof setTimeout> | undefined
  private stale = false // a resync gave up: try again when the connection is back
  // MCP servers the user signed in to or reconnected, by "session:server":
  // a notice says when they connect.
  readonly mcpActed = new Set<string>()

  constructor(backend: Backend) {
    this.backend = backend
  }

  // snapshot is the state now. It's replaced, never changed, on every
  // change, so the same object means nothing changed.
  snapshot(): ClientState {
    return this.s
  }

  // subscribe calls fn after every change; the return stops it.
  subscribe(fn: Listener): () => void {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }

  // onNotice calls fn for each notice; the return stops it.
  onNotice(fn: (n: Notice) => void): () => void {
    this.noticeListeners.add(fn)
    return () => this.noticeListeners.delete(fn)
  }

  // session is the store for one session, made on first use.
  session(id: string): SessionStore {
    let st = this.stores.get(id)
    if (!st) {
      st = new SessionStore(this, id)
      this.stores.set(id, st)
    }
    return st
  }

  // patch changes server state the caller learned itself (a status a
  // settings screen fetched, say). Session data changes through sessions.
  patch(p: Partial<Omit<ClientState, 'data' | 'sessions' | 'ready'>>) {
    this.set(p)
  }

  // dataOf is a session's data (empty until anything is known).
  dataOf(id: string): SessionData {
    return this.s.data[id] ?? EMPTY_SESSION
  }

  viewOf(id: string): SessionView | null {
    return this.s.sessions.find(s => s.id === id) ?? null
  }

  // ---- lifecycle

  // init subscribes to the server's events and fetches the first state.
  // Several UIs sharing the client all call it; it happens once (again
  // only if the first state couldn't be fetched).
  init(): Promise<void> {
    this.started ??= this.start().catch(e => {
      this.started = null
      throw e
    })
    return this.started
  }

  private async start() {
    this.off ??= this.backend.subscribe({
      sessionEvent: m => this.onEvent(m),
      sessionChanged: v => this.later(() => { this.pinsFollowSession(v); this.upsertSession(v) }),
      pageChanged: p => this.later(() => { this.pinsFollowPage(p); this.upsertPage(p) }),
      cliProgress: p => this.set({ progress: p }),
      cliStatus: s => this.set({ cli: s }),
      providerStatus: s => { if (s.provider) this.set({ providerStatus: { ...this.s.providerStatus, [s.provider]: s } }) },
      containersChanged: c => this.set({ containers: c }),
      notifyOpen: id => this.notice({ kind: 'notifyOpen', sessionId: id }),
      remoteStatus: s => this.set({ remote: s }),
      pairRequest: r => this.set({ pairRequest: r }),
      devicesChanged: d => this.set({ devices: d }),
      updateStatus: u => this.set({ update: u }),
      resync: () => { void this.resync() },
      connection: c => {
        this.set({ connection: c })
        if (c.state === 'online' && this.stale) void this.resync()
        // Back after a drop, the server may be a new one (it just updated):
        // what it says about updates now, not what the old one last said.
        if (c.state === 'online') void this.backend.updateStatus().then(u => this.set({ update: u }), () => {})
      },
    })
    await this.load()
  }

  // load fetches the first state. A remote client that couldn't reach its
  // server calls it again (the events keep reconnecting meanwhile).
  async load() {
    const [boot, cli] = await Promise.all([this.backend.bootstrap(), this.backend.cliStatus(false)])
    this.set({ boot, cli, sessions: sortSessions(boot.sessions) })
    void this.refreshPins().catch(e => this.notice({ kind: 'error', error: e }))
    // Containers' labels, and whether one needs the user.
    if (boot.platform === 'windows') void this.loadContainers()
    void this.loadProviders()
    // Remote access, for the server's own window; later changes arrive as events.
    if (this.backend.kind !== 'http') void this.backend.remoteStatus().then(s => { if (!this.s.remote) this.set({ remote: s }) }, () => {})
    void this.backend.updateStatus().then(u => { if (!this.s.update) this.set({ update: u }) }, () => {})
    this.set({ ready: true })
  }

  destroy() {
    this.off?.()
    this.off = null
    this.started = null
    clearTimeout(this.resyncTimer)
  }

  // ---- what the server's settings say

  // namesFor is what text about a session calls its AI provider; no
  // adapter: the first provider.
  namesFor(adapter?: string): ProviderNames {
    return namesOf(providerOf(this.s.boot, adapter))
  }

  // statusOf is a provider's CLI status; the first's is cli.
  statusOf(provider?: string): CLIStatus | null {
    const first = this.s.boot?.providers[0]?.id
    if (!provider || provider === first) return this.s.cli
    return this.s.providerStatus[provider] ?? null
  }

  // modelsFor lists the models a provider's sessions can use: the first's
  // as its CLI reported them, the others' from their sign-in check.
  modelsFor(provider?: string): ModelInfo[] | null {
    const first = this.s.boot?.providers[0]?.id
    if (!provider || provider === first) return this.s.boot?.models?.length ? this.s.boot.models : null
    return this.s.providerStatus[provider]?.models ?? []
  }

  // takesSlash: the session's CLI takes slash commands.
  takesSlash(session: SessionView): boolean {
    const caps = providerOf(this.s.boot, session.adapter)?.capabilities ?? this.s.boot?.capabilities
    return !!caps?.slashPassthrough
  }

  // hasArtifacts: the session's type keeps artifacts.
  hasArtifacts(sessionId: string | null): boolean {
    const s = sessionId ? this.viewOf(sessionId) : null
    return !!s && !!this.s.boot?.profiles.find(p => p.id === s.profileId)?.artifacts
  }

  // The other providers' CLIs: installed, signed in, and their models.
  async loadProviders(fresh = false) {
    const others = (this.s.boot?.providers ?? []).slice(1)
    await Promise.all(others.map(async p => {
      try {
        const st = await this.backend.providerStatus(p.id, fresh)
        this.set({ providerStatus: { ...this.s.providerStatus, [p.id]: st } })
      } catch {
        // its status stays unknown: it's offered as not set up
      }
    }))
  }

  async loadContainers() {
    try {
      this.set({ containers: await this.backend.containers() })
    } catch {
      // not available here: nothing to offer
    }
  }

  // ---- sessions

  // container: a container profile to run in, '' for this machine;
  // provider: an AI provider's id, '' for the first; shares: what it can
  // use of the user's own, null for its type's defaults.
  // meta: an embedding host's word on what the session is for (CreateOptions).
  async createSession(profileId: string, workdir: string, model: string, container = '', provider = '', shares: Share[] | null = null, meta = '') {
    const { session: v, lastNewSession } = meta
      ? await this.backend.createSession(profileId, workdir, model, container, provider, shares, { meta })
      : await this.backend.createSession(profileId, workdir, model, container, provider, shares)
    if (this.s.boot) this.set({ boot: { ...this.s.boot, lastNewSession } })
    this.upsertSession(v)
    this.setData(v.id, { pages: [], index: 0 })
    return v
  }

  // importTranscript imports a saved CLI conversation; its pages load fresh.
  async importTranscript(id: string, profileId: string) {
    const r = await this.backend.importTranscript(id, profileId)
    this.upsertSession(r.session)
    this.setData(r.session.id, { pages: null })
    return r
  }

  // moveSession puts a session at a position in the active list as shown,
  // saving only its new sortOrder. It moves at once, and back if saving fails.
  async moveSession(id: string, to: number) {
    const order = orderAt(this.s.sessions.filter(s => !s.archived), id, to)
    if (order === null) return
    const before = this.s.sessions
    this.set({ sessions: sortSessions(before.map(s => (s.id === id ? { ...s, sortOrder: order } : s))) })
    try {
      this.upsertSession(await this.backend.setSortOrder(id, order))
    } catch (e) {
      this.set({ sessions: before })
      throw e
    }
  }

  // archive archives a session (stopping its CLI) or restores it.
  async archive(id: string, on: boolean) {
    this.upsertSession(await this.backend.archive(id, on))
    void this.refreshPins().catch(e => this.notice({ kind: 'error', error: e }))
  }

  async remove(id: string) {
    await this.backend.remove(id)
    const data = { ...this.s.data }
    delete data[id]
    this.set({ sessions: this.s.sessions.filter(s => s.id !== id), pins: this.s.pins.filter(p => p.sessionId !== id), data })
    this.stores.get(id)?.changed()
  }

  // upsertSession takes a session's new view (from an event or a reply).
  upsertSession(v: SessionView) {
    const i = this.s.sessions.findIndex(s => s.id === v.id)
    const before = this.s.sessions[i]
    for (const sv of connectedNow(v.id, before?.mcp, v.mcp, this.mcpActed)) {
      this.mcpActed.delete(`${v.id}:${sv.name}`)
      this.notice({ kind: 'mcpConnected', sessionId: v.id, server: sv.name, tools: sv.tools })
    }
    let sessions: SessionView[]
    if (i < 0) sessions = sortSessions([v, ...this.s.sessions])
    else if (before?.sortOrder !== v.sortOrder) sessions = sortSessions(this.s.sessions.map(s => (s.id === v.id ? v : s)))
    else sessions = this.s.sessions.map((s, j) => (j === i ? v : s))
    this.set({ sessions })
    this.stores.get(v.id)?.changed()
  }

  // upsertPage takes a page's new state. A page still streaming loses its
  // live text once the text is stored; a new open page (just sent) becomes
  // the one shown; an answer finishing is a notice.
  upsertPage(p: Page) {
    const d = this.dataOf(p.sessionId)
    const list = [...(d.pages ?? [])]
    const i = list.findIndex(x => x.id === p.id)
    if (i >= 0) {
      const before = list[i]
      list[i] = p
      let live = d.live
      if (live && live.seq === p.seq && before && before.answerMd !== p.answerMd) live = { ...live, text: '' }
      this.setData(p.sessionId, { pages: list, live })
      if (before?.status !== 'done' && p.status === 'done') this.notice({ kind: 'pageDone', page: p })
      return
    }
    list.push(p)
    list.sort((a, b) => a.seq - b.seq)
    this.setData(p.sessionId, p.status === 'open' ? { pages: list, index: list.length - 1 } : { pages: list })
  }

  // ---- pins

  async refreshPins() {
    this.set({ pins: await this.backend.pinned() })
  }

  // The pinned pages follow a page pinned, or a pinned page's session
  // renamed or archived, from another client or another part of the UI.
  private pinsFollowPage(p: Page) {
    if (p.pinned !== this.s.pins.some(x => x.pageId === p.id)) void this.refreshPins().catch(e => this.notice({ kind: 'error', error: e }))
  }

  private pinsFollowSession(v: SessionView) {
    const pin = this.s.pins.find(x => x.sessionId === v.id)
    if (pin && (pin.sessionTitle !== v.title || pin.archived !== v.archived)) void this.refreshPins().catch(e => this.notice({ kind: 'error', error: e }))
  }

  // ---- resync

  // later applies a session or page change now, or, while a resync is
  // fetching state, after it: the snapshot would otherwise replace a
  // change that arrived while it was on its way. Replayed in order, the
  // newest wins either way.
  private later(apply: () => void) {
    if (this.held) this.held.push(apply)
    else apply()
  }

  // resync fetches state afresh: a remote client missed events its server
  // no longer holds (decision 0021). The sessions, the pages already
  // loaded and the pins come back as they are now; text that was streaming
  // shows again from its page. Changes arriving meanwhile are applied
  // after the snapshot. One that fails (a phone's network not back yet as
  // it wakes) is tried again after a pause, then when the connection is.
  async resync() {
    if (!this.s.ready) return
    if (this.held) {
      this.resyncAgain = true // asked again while fetching: fetch once more after
      return
    }
    clearTimeout(this.resyncTimer)
    this.stale = false
    this.held = []
    try {
      const boot = await this.backend.bootstrap()
      const loaded = Object.keys(this.s.data).filter(id => this.s.data[id]?.pages && boot.sessions.some(s => s.id === id))
      const fresh = await Promise.all(loaded.map(id => this.backend.pages(id)))
      const data: Record<string, SessionData> = {}
      for (const [id, d] of Object.entries(this.s.data)) {
        if (!boot.sessions.some(s => s.id === id)) continue
        const at = loaded.indexOf(id)
        if (at < 0) {
          data[id] = { ...d, live: null }
          continue
        }
        const list = fresh[at] ?? []
        data[id] = { ...d, pages: list, live: null, index: Math.min(d.index, Math.max(0, list.length - 1)) }
      }
      this.set({ boot, sessions: sortSessions(boot.sessions), data })
      for (const st of this.stores.values()) st.changed()
      void this.refreshPins().catch(e => this.notice({ kind: 'error', error: e }))
      this.resyncTries = 0
      this.notice({ kind: 'resynced' })
    } catch (e) {
      if (this.resyncTries < RESYNC_RETRY_MS.length) {
        this.resyncTimer = setTimeout(() => void this.resync(), RESYNC_RETRY_MS[this.resyncTries++])
      } else {
        this.resyncTries = 0
        this.stale = true
        this.notice({ kind: 'error', error: e })
      }
    } finally {
      const held = this.held ?? []
      this.held = null
      held.forEach(apply => apply())
      if (this.resyncAgain) {
        this.resyncAgain = false
        void this.resync()
      }
    }
  }

  // ---- events

  private onEvent({ sessionId, event }: SessionEventMsg) {
    const live = this.dataOf(sessionId).live
    switch (event.kind) {
      case 'text_delta': {
        const d = event.data as TextDelta
        this.setData(sessionId, {
          live: live && live.seq === event.turnSeq ? { ...live, text: live.text + d.text } : { seq: event.turnSeq, text: d.text, thinkingTokens: 0 },
        })
        break
      }
      case 'thinking': {
        const d = event.data as ThinkingData
        this.setData(sessionId, {
          live: live && live.seq === event.turnSeq
            ? { ...live, thinkingTokens: d.estimatedTokens ?? live.thinkingTokens }
            : { seq: event.turnSeq, text: '', thinkingTokens: d.estimatedTokens ?? 0 },
        })
        break
      }
      case 'text_block':
        if (live) this.setData(sessionId, { live: { ...live, text: '' } })
        break
      case 'commands':
        void this.session(sessionId).loadCommands()
        break
      case 'turn_result':
        this.setData(sessionId, { live: null })
        this.notice({ kind: 'turnEnded', sessionId })
        break
      case 'usage_limit':
        this.set({ usage: event.data as UsageLimit })
        break
    }
  }

  // ---- internal to the package (SessionStore changes its session's data through these)

  /** @internal */
  setData(id: string, patch: Partial<SessionData>) {
    this.set({ data: { ...this.s.data, [id]: { ...this.dataOf(id), ...patch } } })
    this.stores.get(id)?.changed()
  }

  /** @internal */
  notice(n: Notice) {
    for (const fn of this.noticeListeners) fn(n)
  }

  private set(p: Partial<ClientState>) {
    this.s = { ...this.s, ...p }
    for (const fn of this.listeners) fn(this.s)
  }
}

// chatClient is the one client for a backend: every element on a page
// that's given the same backend shares it.
const clients = new WeakMap<Backend, ChatClient>()

export function chatClient(backend: Backend): ChatClient {
  let c = clients.get(backend)
  if (!c) {
    c = new ChatClient(backend)
    clients.set(backend, c)
  }
  return c
}
