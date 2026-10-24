// App state: sessions, their pages, the page each session is showing, and
// the live text of answers still streaming. All state is keyed by id.
import type {
  ArtifactFile, AttachmentRef, Backend, ContainersInfo, PinnedPage, Bootstrap, SearchHit, SearchResult, CLIStatus, Page, Progress, SessionEventMsg, SessionView, TextDelta,
  CommandInfo, ModelInfo, RemoteStatus, Share, ThinkingData, UsageLimit,
} from '../lib/api'
import { autoSummaryFor, loadLayout, outlineOf, saveLayout, summaryBlocks, summaryEntries, type OutlineEntry, type OutlineLayout } from '../lib/outline'
import { loadHeaderCompact, loadPanelLayout, loadQuestionCompact, saveHeaderCompact, loadSidebarWidth, savePanelLayout, saveQuestionCompact, saveSidebarWidth, type PanelTab } from '../lib/panels'
import { loadLastSession, saveLastSession } from '../lib/keep'
import { loadWatched, pick, saveWatched, type Watched } from '../lib/watched'
import { connectedNow } from '../lib/mcp'
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
  // Every other provider's CLI status, by provider id (the first's is cli),
  // loaded once at start and kept up to date.
  providerStatus = $state<Record<string, CLIStatus>>({})
  progress = $state<Progress | null>(null)
  // WSL and the container profiles (decision 0011); loaded on demand.
  containers = $state<ContainersInfo | null>(null)
  // Remote access (decision 0021), for the server's own window.
  remote = $state<RemoteStatus | null>(null)
  // What in Settings needs the user, per tab (lib/attention.ts).
  attention = $derived(settingsAttention(this.containers))
  sessions = $state<SessionView[]>([])
  currentId = $state<string | null>(null)
  pages = $state<Record<string, Page[]>>({})
  index = $state<Record<string, number>>({})
  live = $state<Record<string, Live>>({})
  commands = $state<Record<string, CommandInfo[]>>({}) // session id → the slash commands its CLI takes
  commandsPending = $state<Record<string, boolean>>({}) // session id → its CLI is starting, to report them
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
  // The side panel's tab ("On this page", "Artifacts" or "File"; its open state and
  // width are outline's), the artifact each session shows there, and what is in each session's artifacts folder now.
  panel = $state(loadPanelLayout())
  artifactSel = $state<Record<string, string>>({})
  artifactLive = $state<Record<string, ArtifactFile[]>>({})
  // The File tab's choices per session (lib/watched.ts), loaded as they're
  // first changed, and a request for its switcher (the File tab takes it).
  watched = $state<Record<string, Watched>>({})
  // MCP sign-in: the servers the user signed in to or reconnected (by
  // "session:server", for the toast when they connect), the last sign-in
  // link of each, and connectors waiting to be reconnected.
  mcpActed = new Set<string>()
  mcpLinks = $state<Record<string, string>>({})
  fileSwitcher = $state(false)
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
      providerStatus: s => { if (s.provider) this.providerStatus[s.provider] = s },
      containersChanged: c => { this.containers = c },
      notifyOpen: id => { void this.openFromNotification(id) },
      remoteStatus: s => { this.remote = s },
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
    void this.loadProviders()
    this.ready = true
    // The session open last time, else the first.
    const last = loadLastSession()
    const first = this.activeSessions.find(s => s.id === last) ?? this.activeSessions[0]
    if (first) await this.select(first.id)
  }

  destroy() {
    this.off?.()
  }

  async select(id: string) {
    this.currentId = id
    saveLastSession(id)
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
    void this.loadCommands(id)
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
    if (this.panel.tab === 'file') return 'file'
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

  // MCP sign-in (decision 0020). mcpSignIn opens the server's sign-in in
  // the browser; a connector of the provider's own is signed in on its
  // site, and the backend reconnects it until it connects (MCPServer.site).
  // A server the user acted on gets a toast when it connects.
  mcpSignIn = async (sessionId: string, server: string) => {
    const key = `${sessionId}:${server}`
    try {
      this.mcpActed.add(key)
      const r = await this.backend.mcpSignIn(sessionId, server)
      if (r.url) {
        this.mcpLinks[key] = r.url
        await this.backend.openURL(r.url)
      }
    } catch (e) {
      this.mcpActed.delete(key)
      showToast(String(e), 'error')
    }
  }

  mcpReconnect = async (sessionId: string, server: string) => {
    const key = `${sessionId}:${server}`
    this.mcpActed.add(key)
    await this.backend.mcpReconnect(sessionId, server).catch(e => showToast(String(e), 'error'))
  }

  mcpSignOut = async (sessionId: string, server: string) => {
    await this.backend.mcpSignOut(sessionId, server).catch(e => showToast(String(e), 'error'))
  }

  mcpCancel = async (sessionId: string, server: string) => {
    const key = `${sessionId}:${server}`
    this.mcpActed.delete(key)
    await this.backend.mcpCancel(sessionId, server).catch(e => showToast(String(e), 'error'))
  }

  // mcpPaste hands on the address the browser ended up on.
  mcpPaste = async (sessionId: string, server: string, url: string): Promise<boolean> => {
    try {
      await this.backend.mcpCallback(sessionId, server, url.trim())
      return true
    } catch (e) {
      showToast(String(e), 'error')
      return false
    }
  }

  // watchedFor is a session's File tab choices.
  watchedFor(sessionId: string): Watched {
    return this.watched[sessionId] ?? loadWatched(sessionId)
  }

  setWatched(sessionId: string, w: Watched) {
    this.watched[sessionId] = w
    saveWatched(sessionId, w)
  }

  // watchFile shows a file from the session folder (relative to it) on the
  // File tab, where it updates as it changes.
  watchFile(sessionId: string, path: string) {
    this.setWatched(sessionId, pick(this.watchedFor(sessionId), path))
    this.setTab('file')
    if (!this.outline.open) this.togglePanel()
  }

  // closeFile puts the File tab back as it starts (no file; the recent and
  // starred ones stay) and shows On this page.
  closeFile(sessionId: string) {
    this.setWatched(sessionId, { ...this.watchedFor(sessionId), current: '' })
    this.setTab('outline')
  }

  // chooseFile opens the File tab with its switcher open.
  chooseFile() {
    this.setTab('file')
    if (!this.outline.open) this.togglePanel()
    this.fileSwitcher = true
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

  // The other providers' CLIs: installed, signed in, and their models.
  async loadProviders(fresh = false) {
    const others = (this.boot?.providers ?? []).slice(1)
    await Promise.all(others.map(async p => {
      try {
        this.providerStatus[p.id] = await this.backend.providerStatus(p.id, fresh)
      } catch {
        // its status stays unknown: it's offered as not set up
      }
    }))
  }

  // statusOf is a provider's CLI status; the first's is cli.
  statusOf(provider?: string): CLIStatus | null {
    const first = this.boot?.providers[0]?.id
    if (!provider || provider === first) return this.cli
    return this.providerStatus[provider] ?? null
  }

  // modelsFor lists the models a provider's sessions can use: the first's
  // as its CLI reported them, the others' from their sign-in check.
  modelsFor(provider?: string): ModelInfo[] | null {
    const first = this.boot?.providers[0]?.id
    if (!provider || provider === first) return this.boot?.models?.length ? this.boot.models : null
    return this.providerStatus[provider]?.models ?? []
  }

  async loadContainers() {
    try {
      this.containers = await this.backend.containers()
    } catch {
      // not available here: nothing to offer
    }
  }

  // container: a container profile to run in, '' for this machine;
  // provider: an AI provider's id, '' for the first.
  // shares: what it can use of the user's own; null for its type's defaults.
  async createSession(profileId: string, workdir: string, model: string, container = '', provider = '', shares: Share[] | null = null) {
    const { session: v, lastNewSession } = await this.backend.createSession(profileId, workdir, model, container, provider, shares)
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

  // takesSlash: the session's CLI takes slash commands.
  takesSlash(session: SessionView): boolean {
    const caps = providerOf(this.boot, session.adapter)?.capabilities ?? this.boot?.capabilities
    return !!caps?.slashPassthrough
  }

  // loadCommands fetches the slash commands a session's CLI takes.
  // None known yet means the backend has started the CLI to report them:
  // they come with a 'commands' event (pending until then, or 20 s).
  async loadCommands(id: string) {
    try {
      const list = await this.backend.commands(id)
      this.commands[id] = list
      const s = this.sessions.find(x => x.id === id)
      if (list.length === 0 && s && this.takesSlash(s)) {
        if (!this.commandsPending[id]) {
          this.commandsPending[id] = true
          setTimeout(() => (this.commandsPending[id] = false), 20000)
        }
      } else {
        this.commandsPending[id] = false
      }
    } catch {
      this.commandsPending[id] = false // none offered; typing a command still works
    }
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
    for (const sv of connectedNow(v.id, this.sessions[i]?.mcp, v.mcp, this.mcpActed)) {
      this.mcpActed.delete(`${v.id}:${sv.name}`)
      showToast(t(sv.tools === 1 ? 'mcp.connectedOne' : 'mcp.connected', { name: sv.name, n: sv.tools }), 'success')
    }
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
      case 'commands':
        void this.loadCommands(sessionId)
        break
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
