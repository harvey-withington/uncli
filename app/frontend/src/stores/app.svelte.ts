// App state: sessions, their pages, the page each session is showing, and
// the live text of answers still streaming. All state is keyed by id.
import type {
  ArtifactFile, AttachmentRef, Backend, ContainersInfo, PinnedPage, Bootstrap, SearchHit, SearchResult, CLIStatus, Page, Progress, SessionEventMsg, SessionView, TextDelta,
  ThinkingData, UsageLimit,
} from '../lib/api'
import { autoSummaryFor, loadLayout, outlineOf, saveLayout, summaryBlocks, summaryEntries, type OutlineEntry, type OutlineLayout } from '../lib/outline'
import { loadHeaderCompact, loadPanelLayout, loadQuestionCompact, saveHeaderCompact, loadSidebarWidth, savePanelLayout, saveQuestionCompact, saveSidebarWidth, type PanelTab } from '../lib/panels'
import { toBlocks } from '../lib/render/markdown'
import { orderAt } from '../lib/reorder'
import type { SearchRange } from '../lib/search'
import { setNames, t } from '../lib/i18n.svelte'
import { namesOf, providerOf, type ProviderNames } from '../lib/providers'
import { showToast } from '../lib/toasts.svelte'
import { applyThemeFile } from '../lib/hosttheme'
import { settingsAttention } from '../lib/attention'

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
  // WSL and the container profiles (decision 0011); loaded on demand.
  containers = $state<ContainersInfo | null>(null)
  // What in Settings needs the user, per tab (lib/attention.ts).
  attention = $derived(settingsAttention(this.containers))
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
  usageOpen = $state(false) // the usage dashboard
  settingsAt = $state('') // a section to open Settings at ('safe', 'notify'…: #settings-<section>)
  paletteOpen = $state(false) // the command palette
  keysOpen = $state(false) // the keyboard map
  importOpen = $state(false) // the import picker (saved CLI conversations)
  outline = $state<OutlineLayout>(loadLayout()) // the "On this page" panel
  // The side panel's tab ("On this page" or "Artifacts"; its open state and
  // width are outline's), the artifact each session shows there, and what is in each session's artifacts folder now.
  panel = $state(loadPanelLayout())
  artifactSel = $state<Record<string, string>>({})
  artifactLive = $state<Record<string, ArtifactFile[]>>({})
  sidebarWidth = $state(loadSidebarWidth())
  questionCompact = $state(loadQuestionCompact()) // the question header collapsed to one line
  headerCompact = $state(loadHeaderCompact()) // the session header collapsed to two lines
  // Search across sessions: the box's text and filters (the sidebar shows
  // results instead of the session list while there is text), a counter
  // that focuses the box (Ctrl+K), and a page to scroll to a match on.
  searchText = $state('')
  searchFilters = $state<{ profiles: string[]; bookmarked: boolean; thisSession: boolean; range: SearchRange }>({
    profiles: [], bookmarked: false, thisSession: false, range: 'any',
  })
  searchFocus = $state(0)
  search = $state<{ result: SearchResult | null; active: number }>({ result: null, active: 0 })
  reveal = $state<{ pageId: string; terms: string[] } | null>(null)
  // Pinned pages across sessions (the sidebar's Pinned group), and whether
  // the sidebar shows the archived sessions.
  pins = $state<PinnedPage[]>([])
  showArchived = $state(false)
  summarising = $state<Record<string, boolean>>({}) // page id → a summary is being made
  preferHeadings = $state<Record<string, boolean>>({}) // page id → show its headings, not its summary
  private autoSummarised = new Set<string>() // tried once, so a failure isn't retried in a loop
  private off: (() => void) | null = null

  constructor(backend: Backend) {
    this.backend = backend
  }

  // The session list shows active sessions; archived ones are listed apart.
  activeSessions = $derived.by(() => this.sessions.filter(s => !s.archived))
  archivedSessions = $derived.by(() => this.sessions.filter(s => s.archived))
  current = $derived.by(() => this.sessions.find(s => s.id === this.currentId) ?? null)
  currentPages = $derived.by(() => (this.currentId ? this.pages[this.currentId] ?? [] : []))
  currentIndex = $derived.by(() => (this.currentId ? this.index[this.currentId] ?? 0 : 0))
  currentPage = $derived.by(() => this.currentPages[this.currentIndex] ?? null)
  cliReady = $derived.by(() => !!this.cli?.installed && !!this.cli?.loggedIn)

  // loadThemeFile applies the user's theme.yaml (decision 0010); announce
  // says what happened, for the palette's Reload theme file.
  async loadThemeFile(announce: boolean) {
    try {
      const file = await this.backend.theme()
      const warnings = applyThemeFile(file.found ? { light: file.light ?? undefined, dark: file.dark ?? undefined } : null)
      if (warnings.length) showToast(t('theme.file.warnings', { n: warnings.length, first: warnings[0] ?? '' }), 'error')
      else if (announce) showToast(file.found ? t('theme.file.loaded') : t('theme.file.none', { path: file.path }))
    } catch (e) {
      applyThemeFile(null)
      showToast(String(e), 'error')
    }
  }

  async init() {
    this.off = this.backend.subscribe({
      sessionEvent: m => this.onEvent(m),
      sessionChanged: v => this.upsertSession(v),
      pageChanged: p => this.upsertPage(p),
      cliProgress: p => { this.progress = p },
      cliStatus: s => { this.cli = s },
      containersChanged: c => { this.containers = c },
      notifyOpen: id => { void this.openFromNotification(id) },
    })
    const [boot, cli] = await Promise.all([this.backend.bootstrap(), this.backend.cliStatus(false)])
    this.boot = boot
    setNames(namesOf(providerOf(boot)))
    this.cli = cli
    this.sessions = sortSessions(boot.sessions)
    void this.refreshPins()
    void this.loadThemeFile(false)
    // Containers' labels, and whether one needs the user (the Settings dot).
    if (boot.platform === 'windows') void this.loadContainers()
    this.ready = true
    const first = this.activeSessions[0]
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
    if (this.outline.open) void this.refreshArtifacts(id)
    await this.backend.focus(id)
  }

  // openFromNotification shows the session a notification was about, on
  // its latest page (where the answer or the approval card is).
  async openFromNotification(id: string) {
    if (!id || !this.sessions.some(s => s.id === id)) return
    await this.select(id)
    const n = this.pages[id]?.length ?? 0
    if (n > 0) this.index[id] = n - 1
  }

  // openSettings opens Settings, at a section if one is given.
  openSettings(section = '') {
    this.settingsAt = section
    this.settingsOpen = true
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

  // togglePanel opens or closes the side panel, on the tab it had.
  togglePanel() {
    this.outline.open = !this.outline.open
    saveLayout(this.outline)
    if (this.outline.open && this.currentId) void this.refreshArtifacts(this.currentId)
  }

  // showTab opens the side panel on a tab (O and A); pressed again on the
  // tab already showing, it closes the panel.
  showTab(tab: PanelTab) {
    if (this.outline.open && this.tabFor(this.currentId) === tab) {
      this.togglePanel()
      return
    }
    this.setTab(tab)
    if (!this.outline.open) this.togglePanel()
  }

  setTab(tab: PanelTab) {
    this.panel.tab = tab
    savePanelLayout(this.panel)
  }

  // tabFor is the tab a session shows: Artifacts only for a session type
  // that keeps them.
  tabFor(sessionId: string | null): PanelTab {
    return this.panel.tab === 'artifacts' && this.hasArtifacts(sessionId) ? 'artifacts' : 'outline'
  }

  // hasArtifacts: the session's type keeps artifacts.
  hasArtifacts(sessionId: string | null): boolean {
    const s = this.sessions.find(x => x.id === sessionId)
    return !!s && !!this.boot?.profiles.find(p => p.id === s.profileId)?.artifacts
  }

  // openArtifact shows an artifact (a path in the artifacts folder) on the
  // Artifacts tab.
  openArtifact(sessionId: string, path: string) {
    this.artifactSel[sessionId] = path
    this.setTab('artifacts')
    if (!this.outline.open) this.togglePanel()
    else void this.refreshArtifacts(sessionId)
  }

  // refreshArtifacts reads what is in a session's artifacts folder now.
  async refreshArtifacts(sessionId: string) {
    if (!this.hasArtifacts(sessionId)) return
    try {
      this.artifactLive[sessionId] = await this.backend.artifactFiles(sessionId)
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  toggleHeaderCompact() {
    this.headerCompact = !this.headerCompact
    saveHeaderCompact(this.headerCompact)
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
    const order = orderAt(this.activeSessions, id, to)
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

  // openHit shows a search result: its session, its page, and (via reveal)
  // the first block with a match, scrolled into view.
  async openHit(hit: SearchHit, terms: string[]) {
    if (this.currentId !== hit.sessionId || !this.pages[hit.sessionId]) await this.select(hit.sessionId)
    let list = this.pages[hit.sessionId] ?? []
    if (!list.some(p => p.id === hit.pageId)) {
      list = this.pages[hit.sessionId] = await this.backend.pages(hit.sessionId)
    }
    const i = list.findIndex(p => p.id === hit.pageId)
    if (i >= 0) this.index[hit.sessionId] = i
    this.reveal = { pageId: hit.pageId, terms }
  }

  goTo(i: number) {
    if (this.currentId) this.index[this.currentId] = i
  }

  // namesFor is what text about a session calls its AI provider (t()'s
  // {agent}, {cli}, {account}); no adapter: the first provider.
  namesFor(adapter?: string): ProviderNames {
    return namesOf(providerOf(this.boot, adapter))
  }

  async loadContainers() {
    try {
      this.containers = await this.backend.containers()
    } catch {
      // not available here: nothing to offer
    }
  }

  // container: a container profile to run in, '' for this machine.
  async createSession(profileId: string, workdir: string, model: string, container = '') {
    const { session: v, lastNewSession } = await this.backend.createSession(profileId, workdir, model, container)
    if (this.boot) this.boot.lastNewSession = lastNewSession
    this.upsertSession(v)
    this.pages[v.id] = []
    this.index[v.id] = 0
    await this.select(v.id)
    return v
  }

  async send(text: string, attachments: AttachmentRef[] = []) {
    const id = this.currentId
    if (!id) return
    await this.backend.send(id, text, attachments)
  }

  async toggleBookmark() {
    const p = this.currentPage
    if (!p) return
    const updated = await this.backend.setBookmark(p.sessionId, p.id, !p.bookmarked)
    this.upsertPage(updated)
  }

  // togglePin pins or unpins the page shown (P).
  async togglePin() {
    const p = this.currentPage
    if (!p) return
    await this.setPinned(p.sessionId, p.id, !p.pinned)
  }

  async setPinned(sessionId: string, pageId: string, on: boolean) {
    this.upsertPage(await this.backend.setPinned(sessionId, pageId, on))
    await this.refreshPins()
  }

  async refreshPins() {
    try {
      this.pins = await this.backend.pinned()
    } catch (e) {
      showToast(String(e), 'error')
    }
  }

  // openPin shows a pinned page in its session.
  async openPin(pin: PinnedPage) {
    if (this.currentId !== pin.sessionId || !this.pages[pin.sessionId]) await this.select(pin.sessionId)
    let list = this.pages[pin.sessionId] ?? []
    if (!list.some(p => p.id === pin.pageId)) list = this.pages[pin.sessionId] = await this.backend.pages(pin.sessionId)
    const i = list.findIndex(p => p.id === pin.pageId)
    if (i >= 0) this.index[pin.sessionId] = i
  }

  // importTranscript imports a saved CLI conversation and opens it on its
  // latest page.
  async importTranscript(id: string, profileId: string) {
    const r = await this.backend.importTranscript(id, profileId)
    this.upsertSession(r.session)
    delete this.pages[r.session.id] // load them fresh
    await this.select(r.session.id)
    this.index[r.session.id] = Math.max(0, (this.pages[r.session.id]?.length ?? 1) - 1)
    showToast(t(r.folderGone ? 'import.doneGone' : 'import.done', { n: r.pagesLoaded }), r.folderGone ? 'info' : 'success')
    return r
  }

  // archive archives a session (stopping its CLI) or restores it.
  async archive(id: string, on: boolean) {
    this.upsertSession(await this.backend.archive(id, on))
    void this.refreshPins()
  }

  async remove(id: string) {
    await this.backend.remove(id)
    this.sessions = this.sessions.filter(s => s.id !== id)
    this.pins = this.pins.filter(p => p.sessionId !== id)
    delete this.pages[id]
    delete this.index[id]
    delete this.live[id]
    if (this.currentId === id) {
      this.currentId = null
      const next = this.activeSessions[0]
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
        if (this.outline.open && sessionId === this.currentId) void this.refreshArtifacts(sessionId)
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
