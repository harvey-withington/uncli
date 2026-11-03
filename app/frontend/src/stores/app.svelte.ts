// The app's store: the window's (or the phone's) shell around a ChatClient
// (decision 0026). What the server says, each session's pages and its live
// text are the client's (@uncli/client), read here through ChatState's
// runes; this keeps what only the app has: dialogs, search, the sidebar,
// the side panel's layout, the session shown, and the user's preferences.
// Errors from the client's actions show as toasts on this root's shell.
import {
  ChatClient, displayAnswer as display,
  type AttachmentRef, type Backend, type CLIStatus, type ModelInfo, type Notice, type Page, type PinnedPage, type SearchHit,
  type SearchResult, type SessionView, type Share, type Bootstrap, type Live, type ProviderNames,
} from '../lib/api'
import { ChatState, keyed } from '../lib/chat.svelte'
import { autoSummaryFor, loadLayout, outlineOf, saveLayout, summaryBlocks, summaryEntries, type OutlineEntry, type OutlineLayout } from '../lib/outline'
import { loadHeaderCompact, loadPanelLayout, loadQuestionCompact, saveHeaderCompact, loadSidebarWidth, savePanelLayout, saveQuestionCompact, saveSidebarWidth, type PanelTab } from '../lib/panels'
import { loadLastSession, saveLastSession } from '../lib/keep'
import { loadWatched, pick, saveWatched, type Watched } from '../lib/watched'
import { toBlocks } from '../lib/render/markdown'
import type { SearchRange } from '../lib/search'
import { setNames, t } from '../lib/i18n.svelte'
import { namesOf, providerOf } from '../lib/api'
import { Shell } from '../lib/shell.svelte'
import { applyThemeFile } from '../lib/hosttheme'
import { settingsAttention } from '../lib/attention'

export type { Live }

export class AppStore {
  readonly client: ChatClient
  private chat: ChatState
  ready = $state(false)
  currentId = $state<string | null>(null)
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
  // The side panel's tab ("On this page", "Artifacts" or "File"; its open
  // state and width are outline's).
  panel = $state(loadPanelLayout())
  // The File tab's choices per session (lib/watched.ts), loaded as they're
  // first changed, and a request for its switcher (the File tab takes it).
  watched = $state<Record<string, Watched>>({})
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
  showArchived = $state(false) // the sidebar shows the archived sessions
  preferHeadings = $state<Record<string, boolean>>({}) // page id → show its headings, not its summary
  private autoSummarised = new Set<string>() // tried once, so a failure isn't retried in a loop
  private offNotices: (() => void) | null = null

  // openLast: start on the session open last time (the window); the phone
  // starts on its session list instead.
  private openLast: boolean

  // This root's toasts and confirm dialog (lib/shell): the store's own
  // unless the root passes one.
  readonly shell: Shell

  constructor(backend: Backend, opts: { openLast?: boolean; shell?: Shell } = {}) {
    this.client = new ChatClient(backend)
    this.chat = new ChatState(this.client)
    this.openLast = opts.openLast ?? true
    this.shell = opts.shell ?? new Shell()
  }

  // ---- the client's state, as runes

  get backend(): Backend { return this.client.backend }
  get boot(): Bootstrap | null { return this.chat.boot }
  get cli() { return this.chat.cli }
  set cli(v) { this.client.patch({ cli: v }) }
  get providerStatus() { return this.chat.providerStatus }
  get progress() { return this.chat.progress }
  get containers() { return this.chat.containers }
  get remote() { return this.chat.remote }
  set remote(v) { this.client.patch({ remote: v }) }
  get devices() { return this.chat.devices }
  set devices(v) { this.client.patch({ devices: v }) }
  get pairRequest() { return this.chat.pairRequest }
  set pairRequest(v) { this.client.patch({ pairRequest: v }) }
  get connection() { return this.chat.connection }
  get update() { return this.chat.update }
  get usage() { return this.chat.usage }
  set usage(v) { this.client.patch({ usage: v }) }
  get sessions(): SessionView[] { return this.chat.sessions }
  get pins(): PinnedPage[] { return this.chat.pins }

  // Each session's data, indexed by session id (and summaries by page id).
  readonly pages = keyed(id => this.chat.dataOf(id).pages ?? undefined)
  readonly index = keyed(id => this.chat.dataOf(id).index)
  readonly live = keyed(id => this.chat.dataOf(id).live ?? undefined)
  readonly commands = keyed(id => this.chat.dataOf(id).commands)
  readonly commandsPending = keyed(id => this.chat.dataOf(id).commandsPending)
  readonly artifactSel = keyed(id => this.chat.dataOf(id).artifactSel || undefined)
  readonly artifactLive = keyed(id => this.chat.dataOf(id).artifactFiles ?? undefined)
  // By "session:server", the last sign-in link of each MCP server.
  readonly mcpLinks = keyed(key => {
    const at = key.lastIndexOf(':')
    return this.chat.dataOf(key.slice(0, at)).mcpLinks[key.slice(at + 1)]
  })
  readonly summarising = keyed(pageId => this.chat.sessions.some(s => this.chat.dataOf(s.id).summarising[pageId]) || undefined)

  // patchBoot changes what the server said at start (a setting saved).
  patchBoot(p: Partial<Bootstrap>) {
    if (this.boot) this.client.patch({ boot: { ...this.boot, ...p } })
  }

  setProviderStatus(provider: string, st: CLIStatus) {
    this.client.patch({ providerStatus: { ...this.providerStatus, [provider]: st } })
  }

  // What in Settings needs the user, per tab (lib/attention.ts).
  attention = $derived(settingsAttention(this.containers))
  // The session list shows active sessions; archived ones are listed apart.
  activeSessions = $derived.by(() => this.sessions.filter(s => !s.archived))
  archivedSessions = $derived.by(() => this.sessions.filter(s => s.archived))
  current = $derived.by(() => this.sessions.find(s => s.id === this.currentId) ?? null)
  currentPages = $derived.by(() => (this.currentId ? this.chat.dataOf(this.currentId).pages ?? [] : []))
  currentIndex = $derived.by(() => (this.currentId ? this.chat.dataOf(this.currentId).index : 0))
  currentPage = $derived.by(() => this.currentPages[this.currentIndex] ?? null)
  cliReady = $derived.by(() => !!this.cli?.installed && !!this.cli?.loggedIn)

  // ---- lifecycle

  async init() {
    this.offNotices ??= this.client.onNotice(n => this.onNotice(n))
    await this.client.init()
    await this.loaded()
  }

  // load fetches the first state. A remote client that couldn't reach its
  // server calls it again (the events keep reconnecting meanwhile).
  async load() {
    await this.client.load()
    await this.loaded()
  }

  // loaded follows the first state: the provider's names for text, the
  // theme file, then (the window) the session open last time, else the first.
  private async loaded() {
    setNames(namesOf(providerOf(this.boot)))
    void this.loadThemeFile(false)
    this.ready = true
    if (!this.openLast) return
    const last = loadLastSession()
    const first = this.activeSessions.find(s => s.id === last) ?? this.activeSessions[0]
    if (first) await this.select(first.id)
  }

  destroy() {
    this.offNotices?.()
    this.client.destroy()
    this.chat.destroy()
  }

  resync() {
    return this.client.resync()
  }

  private onNotice(n: Notice) {
    switch (n.kind) {
      case 'mcpConnected':
        this.shell.toast(t(n.tools === 1 ? 'mcp.connectedOne' : 'mcp.connected', { name: n.server, n: n.tools }), 'success')
        break
      case 'pageDone':
        this.autoSummarise(n.page)
        break
      case 'turnEnded':
        if (this.outline.open && n.sessionId === this.currentId) void this.refreshArtifacts(n.sessionId)
        break
      case 'notifyOpen':
        void this.openFromNotification(n.sessionId)
        break
      case 'resynced':
        if (this.currentId && !this.sessions.some(s => s.id === this.currentId)) this.currentId = null
        break
      case 'error':
        this.shell.toast(String(n.error), 'error')
        break
    }
  }

  // ---- the theme file

  // loadThemeFile applies the user's theme.yaml (decision 0010); announce
  // says what happened, for the palette's Reload theme file.
  async loadThemeFile(announce: boolean) {
    try {
      const file = await this.backend.theme()
      const warnings = applyThemeFile(file.found ? { light: file.light ?? undefined, dark: file.dark ?? undefined } : null)
      if (warnings.length) this.shell.toast(t('theme.file.warnings', { n: warnings.length, first: warnings[0] ?? '' }), 'error')
      else if (announce) this.shell.toast(file.found ? t('theme.file.loaded') : t('theme.file.none', { path: file.path }))
    } catch (e) {
      applyThemeFile(null)
      this.shell.toast(String(e), 'error')
    }
  }

  // ---- the session shown

  async select(id: string) {
    this.currentId = id
    saveLastSession(id)
    try {
      await this.client.session(id).load()
    } catch (e) {
      this.shell.toast(String(e), 'error')
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
    this.client.session(id).showLatest()
  }

  // openHit shows a search result: its session, its page, and (via reveal)
  // the first block with a match, scrolled into view.
  async openHit(hit: SearchHit, terms: string[]) {
    if (this.currentId !== hit.sessionId || !this.pages[hit.sessionId]) await this.select(hit.sessionId)
    await this.client.session(hit.sessionId).show(hit.pageId)
    this.reveal = { pageId: hit.pageId, terms }
  }

  // openPin shows a pinned page in its session.
  async openPin(pin: PinnedPage) {
    if (this.currentId !== pin.sessionId || !this.pages[pin.sessionId]) await this.select(pin.sessionId)
    await this.client.session(pin.sessionId).show(pin.pageId)
  }

  goTo(i: number) {
    if (this.currentId) this.client.session(this.currentId).goTo(i)
  }

  async send(text: string, attachments: AttachmentRef[] = []) {
    if (this.currentId) await this.client.session(this.currentId).send(text, attachments)
  }

  async toggleBookmark() {
    if (this.currentId) await this.client.session(this.currentId).toggleBookmark()
  }

  // togglePin pins or unpins the page shown (P).
  async togglePin() {
    if (this.currentId) await this.client.session(this.currentId).togglePin()
  }

  async setPinned(sessionId: string, pageId: string, on: boolean) {
    await this.client.session(sessionId).setPinned(pageId, on)
  }

  // ---- dialogs and layout

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
    this.client.session(sessionId).selectArtifact(path)
    this.setTab('artifacts')
    if (!this.outline.open) this.togglePanel()
    else void this.refreshArtifacts(sessionId)
  }

  // refreshArtifacts reads what is in a session's artifacts folder now.
  async refreshArtifacts(sessionId: string) {
    try {
      await this.client.session(sessionId).refreshArtifacts()
    } catch (e) {
      this.shell.toast(String(e), 'error')
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

  // ---- MCP sign-in (decision 0020), with errors as toasts

  mcpSignIn = (sessionId: string, server: string) => this.toasting(this.client.session(sessionId).mcpSignIn(server))
  mcpReconnect = (sessionId: string, server: string) => this.toasting(this.client.session(sessionId).mcpReconnect(server))
  mcpSignOut = (sessionId: string, server: string) => this.toasting(this.client.session(sessionId).mcpSignOut(server))
  mcpCancel = (sessionId: string, server: string) => this.toasting(this.client.session(sessionId).mcpCancel(server))

  // mcpPaste hands on the address the browser ended up on.
  mcpPaste = async (sessionId: string, server: string, url: string): Promise<boolean> => {
    try {
      await this.client.session(sessionId).mcpPaste(server, url)
      return true
    } catch (e) {
      this.shell.toast(String(e), 'error')
      return false
    }
  }

  // ---- what the server's settings say

  namesFor(adapter?: string): ProviderNames {
    return namesOf(providerOf(this.boot, adapter))
  }

  statusOf(provider?: string): CLIStatus | null {
    const first = this.boot?.providers[0]?.id
    if (!provider || provider === first) return this.cli
    return this.providerStatus[provider] ?? null
  }

  modelsFor(provider?: string): ModelInfo[] | null {
    const first = this.boot?.providers[0]?.id
    if (!provider || provider === first) return this.boot?.models?.length ? this.boot.models : null
    return this.providerStatus[provider]?.models ?? []
  }

  takesSlash(session: SessionView): boolean {
    const caps = providerOf(this.boot, session.adapter)?.capabilities ?? this.boot?.capabilities
    return !!caps?.slashPassthrough
  }

  loadProviders(fresh = false) {
    return this.client.loadProviders(fresh)
  }

  loadContainers() {
    return this.client.loadContainers()
  }

  loadCommands(id: string) {
    return this.client.session(id).loadCommands()
  }

  // ---- sessions

  async createSession(profileId: string, workdir: string, model: string, container = '', provider = '', shares: Share[] | null = null) {
    const v = await this.client.createSession(profileId, workdir, model, container, provider, shares)
    await this.select(v.id)
    return v
  }

  // importTranscript imports a saved CLI conversation and opens it on its
  // latest page.
  async importTranscript(id: string, profileId: string) {
    const r = await this.client.importTranscript(id, profileId)
    await this.select(r.session.id)
    this.client.session(r.session.id).showLatest()
    this.shell.toast(t(r.folderGone ? 'import.doneGone' : 'import.done', { n: r.pagesLoaded }), r.folderGone ? 'info' : 'success')
    return r
  }

  // moveSession puts a session at a position in the list as shown (drag
  // and drop, or Alt+arrows).
  async moveSession(id: string, to: number) {
    try {
      await this.client.moveSession(id, to)
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  async refreshPins() {
    try {
      await this.client.refreshPins()
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  // restart stops a session's CLI and starts it again on its conversation
  // (a frozen CLI, a container's changed mounts). An answer in progress
  // would be cut short, so that asks first.
  async restart(id: string) {
    const s = this.sessions.find(x => x.id === id)
    if (!s || s.archived) return
    const names = this.namesFor(s.adapter)
    if (s.busy && !(await this.shell.confirm({ title: t('restart.title'), message: t('restart.busy', names), confirmLabel: t('restart.label'), danger: true }))) return
    try {
      await this.client.session(id).restart()
      this.shell.toast(t('restart.done', names), 'success')
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  // archive archives a session (stopping its CLI) or restores it.
  archive(id: string, on: boolean) {
    return this.client.archive(id, on)
  }

  async remove(id: string) {
    await this.client.remove(id)
    if (this.currentId === id) {
      this.currentId = null
      const next = this.activeSessions[0]
      if (next) await this.select(next.id)
    }
  }

  upsertSession(v: SessionView) {
    this.client.upsertSession(v)
  }

  upsertPage(p: Page) {
    this.client.upsertPage(p)
  }

  // ---- updates (decision 0025)

  // checkForUpdates looks for a newer release now (Settings → Updates).
  async checkForUpdates() {
    try {
      this.client.patch({ update: await this.backend.checkForUpdates() })
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  // installUpdate downloads the newer release and hands it to the installer:
  // UNCLI quits, installs and starts again. An answer in progress would stop
  // where it is, so that's asked first.
  async installUpdate() {
    const v = this.update?.latest?.version ?? ''
    const busy = this.sessions.some(s => s.busy)
    if (!(await this.shell.confirm({
      title: t('update.confirmTitle', { version: v }),
      message: t(busy ? 'update.confirmBusy' : 'update.confirmBody'),
      confirmLabel: t('update.now'),
      danger: busy,
    }))) return
    try {
      this.client.patch({ update: await this.backend.installUpdate() })
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  // ---- the outline and summaries

  // outlineFor is what the outline shows for a page: the summary when there
  // is one (unless the user switched to headings), else the headings. The
  // answer's margin marks the same entries.
  outlineFor(page: Page) {
    const blocks = toBlocks(display(page, this.chat.dataOf(page.sessionId).live))
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
    try {
      await this.client.session(page.sessionId).summarise(page.id, summaryBlocks(toBlocks(page.answerMd)))
    } catch (e) {
      this.shell.toast(String(e), 'error')
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

  private async toasting(p: Promise<unknown>) {
    try {
      await p
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }
}

// The answer to show for a page: what's stored plus any block still streaming.
export function displayAnswer(page: Page, live: Live | undefined): string {
  return display(page, live)
}
