// App state: sessions, their pages, the page each session is showing, and
// the live text of answers still streaming. All state is keyed by id.
import type {
  Backend, Bootstrap, CLIStatus, Page, Progress, SessionEventMsg, SessionView, TextDelta,
  ThinkingData, UsageLimit,
} from '../lib/api'
import { autoSummaryFor, loadLayout, outlineOf, saveLayout, summaryBlocks, summaryEntries, type OutlineEntry, type OutlineLayout } from '../lib/outline'
import { loadQuestionCompact, loadSidebarWidth, saveQuestionCompact, saveSidebarWidth } from '../lib/panels'
import { toBlocks } from '../lib/render/markdown'
import { orderAt } from '../lib/reorder'
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
  newSessionFolder = $state<string | null>(null) // folder it opens with (a dropped folder)
  newSessionSeq = $state(0) // each open is a fresh dialog, even mid fade-out
  settingsOpen = $state(false)
  outline = $state<OutlineLayout>(loadLayout()) // the "On this page" panel
  sidebarWidth = $state(loadSidebarWidth())
  questionCompact = $state(loadQuestionCompact()) // the question header collapsed to one line
  summarising = $state<Record<string, boolean>>({}) // page id → a summary is being made
  preferHeadings = $state<Record<string, boolean>>({}) // page id → show its headings, not its summary
  private autoSummarised = new Set<string>() // tried once, so a failure isn't retried in a loop
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
  openNewSession(profileId: string | null = null, folder: string | null = null) {
    this.newSessionProfile = profileId
    this.newSessionFolder = folder
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

  toggleQuestionCompact() {
    this.questionCompact = !this.questionCompact
    saveQuestionCompact(this.questionCompact)
  }

  setOutlineWidth(width: number) {
    this.outline.width = width
    saveLayout(this.outline)
  }

  // moveSession puts a session at a position in the list as shown (drag
  // and drop, or Alt+arrows), saving only its new sortOrder.
  async moveSession(id: string, to: number) {
    const order = orderAt(this.sessions, id, to)
    if (order === null) return
    const before = this.sessions
    this.sessions = sortSessions(this.sessions.map(s => (s.id === id ? { ...s, sortOrder: order } : s)))
    try {
      this.upsertSession(await this.backend.setSortOrder(id, order))
    } catch (e) {
      this.sessions = before
      showToast(String(e), 'error')
    }
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
    if (i < 0) {
      this.sessions = sortSessions([v, ...this.sessions])
    } else if (this.sessions[i]?.sortOrder !== v.sortOrder) {
      this.sessions = sortSessions(this.sessions.map(s => (s.id === v.id ? v : s)))
    } else {
      this.sessions[i] = v
    }
  }

  // outlineFor is what the outline shows for a page: the summary when there
  // is one (unless the user switched to headings), else the headings. The
  // answer's margin marks the same entries.
  outlineFor(page: Page) {
    const blocks = toBlocks(displayAnswer(page, this.live[page.sessionId]))
    const headings = outlineOf(blocks)
    const summary = page.outline ? summaryEntries(page.outline.sections, blocks) : []
    const showingSummary = summary.length > 0 && !(this.preferHeadings[page.id] && headings.length > 0)
    const entries: OutlineEntry[] = showingSummary ? summary : headings
    return { blocks, headings, summary, showingSummary, entries }
  }

  // summarise asks the quick-task model for a page's table of contents.
  async summarise(page: Page) {
    if (this.summarising[page.id] || page.status === 'open') return
    delete this.preferHeadings[page.id] // show the summary when it arrives
    this.summarising[page.id] = true
    try {
      this.upsertPage(await this.backend.summarisePage(page.sessionId, page.id, summaryBlocks(toBlocks(page.answerMd))))
    } catch (e) {
      showToast(String(e), 'error')
    } finally {
      delete this.summarising[page.id]
    }
  }

  // autoSummarise summarises a finished page if the user's preference says
  // this answer should be. Once per page per run.
  autoSummarise(page: Page) {
    if (page.status !== 'done' || page.outline || this.autoSummarised.has(page.id)) return
    const blocks = toBlocks(page.answerMd)
    if (autoSummaryFor(page.answerMd, blocks.length, this.boot?.preferences.autoSummary ?? 'off') !== 'summarise') return
    this.autoSummarised.add(page.id)
    void this.summarise(page)
  }

  upsertPage(p: Page) {
    const list = (this.pages[p.sessionId] ??= [])
    const i = list.findIndex(x => x.id === p.id)
    if (i >= 0) {
      const before = list[i]
      list[i] = p
      const live = this.live[p.sessionId]
      if (live && live.seq === p.seq && before && before.answerMd !== p.answerMd) live.text = ''
      // As soon as an answer finishes, wherever the user is looking.
      if (before?.status !== 'done' && p.status === 'done') this.autoSummarise(p)
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
