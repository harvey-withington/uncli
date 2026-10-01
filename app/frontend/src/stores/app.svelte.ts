// App state: sessions, their pages, the page each session is showing, and
// the live text of answers still streaming. All state is keyed by id.
import type {
  Backend, Bootstrap, CLIStatus, Page, Progress, SessionEventMsg, SessionView, TextDelta,
  ThinkingData, UsageLimit,
} from '../lib/api'
import { loadLayout, saveLayout, type OutlineLayout } from '../lib/outline'
import { loadSidebarWidth, saveSidebarWidth } from '../lib/panels'
import { showToast } from '../lib/toasts.svelte'

export interface Live {
  seq: number
  text: string // the text block being streamed, not yet in answerMd
  thinkingTokens: number
}

export class AppStore {
  backend: Backend
  ready = $state(false)
  boot = $state<Bootstrap | null>(null)
  cli = $state<CLIStatus | null>(null)
  progress = $state<Progress | null>(null)
  sessions = $state<SessionView[]>([])
  currentId = $state<string | null>(null)
  pages = $state<Record<string, Page[]>>({})
  index = $state<Record<string, number>>({})
  live = $state<Record<string, Live>>({})
  usage = $state<UsageLimit | null>(null)
  newSessionOpen = $state(false)
  newSessionProfile = $state<string | null>(null) // profile the dialog opens on
  newSessionSeq = $state(0) // each open is a fresh dialog, even mid fade-out
  settingsOpen = $state(false)
  outline = $state<OutlineLayout>(loadLayout()) // the "On this page" panel
  sidebarWidth = $state(loadSidebarWidth())
  private off: (() => void) | null = null

  constructor(backend: Backend) {
    this.backend = backend
  }

  current = $derived.by(() => this.sessions.find(s => s.id === this.currentId) ?? null)
  currentPages = $derived.by(() => (this.currentId ? this.pages[this.currentId] ?? [] : []))
  currentIndex = $derived.by(() => (this.currentId ? this.index[this.currentId] ?? 0 : 0))
  currentPage = $derived.by(() => this.currentPages[this.currentIndex] ?? null)
  cliReady = $derived.by(() => !!this.cli?.installed && !!this.cli?.loggedIn)

  async init() {
    this.off = this.backend.subscribe({
      sessionEvent: m => this.onEvent(m),
      sessionChanged: v => this.upsertSession(v),
      pageChanged: p => this.upsertPage(p),
      cliProgress: p => { this.progress = p },
      cliStatus: s => { this.cli = s },
    })
    const [boot, cli] = await Promise.all([this.backend.bootstrap(), this.backend.cliStatus(false)])
    this.boot = boot
    this.cli = cli
    this.sessions = sortSessions(boot.sessions)
    this.ready = true
    const first = this.sessions[0]
    if (first) await this.select(first.id)
  }

  destroy() {
    this.off?.()
  }

  async select(id: string) {
    this.currentId = id
    if (!this.pages[id]) {
      try {
        const pages = await this.backend.pages(id)
        this.pages[id] = pages
        this.index[id] = Math.max(0, pages.length - 1)
      } catch (e) {
        showToast(String(e), 'error')
      }
    }
    await this.backend.focus(id)
  }

  // openNewSession opens the new-session dialog, on a given profile if one
  // was picked (the welcome cards), else on the first.
  openNewSession(profileId: string | null = null) {
    this.newSessionProfile = profileId
    this.newSessionSeq++
    this.newSessionOpen = true
  }

  setSidebarWidth(width: number) {
    this.sidebarWidth = width
    saveSidebarWidth(width)
  }

  toggleOutline() {
    this.outline.open = !this.outline.open
    saveLayout(this.outline)
  }

  setOutlineWidth(width: number) {
    this.outline.width = width
    saveLayout(this.outline)
  }

  goTo(i: number) {
    if (this.currentId) this.index[this.currentId] = i
  }

  async createSession(profileId: string, workdir: string, model: string) {
    const { session: v, lastNewSession } = await this.backend.createSession(profileId, workdir, model)
    if (this.boot) this.boot.lastNewSession = lastNewSession
    this.upsertSession(v)
    this.pages[v.id] = []
    this.index[v.id] = 0
    await this.select(v.id)
    return v
  }

  async send(text: string) {
    const id = this.currentId
    if (!id) return
    await this.backend.send(id, text)
  }

  async toggleBookmark() {
    const p = this.currentPage
    if (!p) return
    const updated = await this.backend.setBookmark(p.sessionId, p.id, !p.bookmarked)
    this.upsertPage(updated)
  }

  async remove(id: string) {
    await this.backend.remove(id)
    this.sessions = this.sessions.filter(s => s.id !== id)
    delete this.pages[id]
    delete this.index[id]
    delete this.live[id]
    if (this.currentId === id) {
      this.currentId = null
      const next = this.sessions[0]
      if (next) await this.select(next.id)
    }
  }

  upsertSession(v: SessionView) {
    const i = this.sessions.findIndex(s => s.id === v.id)
    if (i >= 0) this.sessions[i] = v
    else this.sessions = sortSessions([v, ...this.sessions])
  }

  upsertPage(p: Page) {
    const list = (this.pages[p.sessionId] ??= [])
    const i = list.findIndex(x => x.id === p.id)
    if (i >= 0) {
      const before = list[i]
      list[i] = p
      const live = this.live[p.sessionId]
      if (live && live.seq === p.seq && before && before.answerMd !== p.answerMd) live.text = ''
      return
    }
    list.push(p)
    list.sort((a, b) => a.seq - b.seq)
    // A new page (just sent) becomes the one shown.
    if (p.status === 'open') this.index[p.sessionId] = list.length - 1
  }

  private onEvent({ sessionId, event }: SessionEventMsg) {
    switch (event.kind) {
      case 'text_delta': {
        const d = event.data as TextDelta
        const cur = this.live[sessionId]
        if (cur && cur.seq === event.turnSeq) cur.text += d.text
        else this.live[sessionId] = { seq: event.turnSeq, text: d.text, thinkingTokens: 0 }
        break
      }
      case 'thinking': {
        const d = event.data as ThinkingData
        const cur = this.live[sessionId]
        if (cur && cur.seq === event.turnSeq) cur.thinkingTokens = d.estimatedTokens ?? cur.thinkingTokens
        else this.live[sessionId] = { seq: event.turnSeq, text: '', thinkingTokens: d.estimatedTokens ?? 0 }
        break
      }
      case 'text_block': {
        const cur = this.live[sessionId]
        if (cur) cur.text = ''
        break
      }
      case 'turn_result':
        delete this.live[sessionId]
        break
      case 'usage_limit':
        this.usage = event.data as UsageLimit
        break
    }
  }
}

function sortSessions(list: SessionView[]): SessionView[] {
  return [...list].sort((a, b) => b.sortOrder - a.sortOrder || b.createdAt - a.createdAt)
}

// The answer to show for a page: what's stored plus any block still streaming.
export function displayAnswer(page: Page, live: Live | undefined): string {
  if (!live || live.seq !== page.seq || !live.text) return page.answerMd
  return page.answerMd ? `${page.answerMd}\n\n${live.text}` : live.text
}
