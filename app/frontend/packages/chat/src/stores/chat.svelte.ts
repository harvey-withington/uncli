// The chat's store: what UNCLI's chat components need to show one
// session, around a ChatClient (decision 0026). The session shown, its side
// panel's layout, the outline and summaries, and the session's actions with
// their errors as toasts on this root's shell. The window's and the phone's
// AppStore build on it; an embedded chat has one of its own. What only an
// app can do (open Settings, the usage dashboard) are hooks an embed leaves
// out, and the chat hides what needs them.
import {
  ChatClient, displayAnswer as display,
  type AttachmentRef, type Backend, type CLIStatus, type ModelInfo, type Notice, type Page, type SessionView,
  type Bootstrap, type Live, type ProviderNames, type Scope,
} from '../lib/api'
import { ChatState, keyed } from '../lib/chat.svelte'
import { autoSummaryFor, outlineOf, summaryBlocks, summaryEntries, type OutlineEntry, type OutlineLayout } from '../lib/outline'
import type { PanelLayout, PanelTab } from '../lib/panels'
import { appMemory, type ChatLayout, type LayoutMemory } from '../lib/layoutmemory'
import { loadWatched, pick, saveWatched, type Watched } from '../lib/watched'
import { toBlocks } from '../lib/render/markdown'
import { t } from '../lib/i18n.svelte'
import { namesOf, providerOf } from '../lib/api'
import { Shell } from '../lib/shell.svelte'
import type { EnterKey } from '../lib/enterkey.svelte'
import type { ComposerControl } from '../lib/composercontrols'
import type { HostReview, ReviewAnswer } from '../lib/hostreview'

export type { Live }

// A host's @mention: what the menu shows, and what picking it puts in
// the message (a link in the host's own scheme, say).
export interface Mention {
  label: string
  detail?: string
  insert: string
}

export interface ChatStoreOptions {
  shell?: Shell
  // The client to use (shared with other chats on the page); one of the
  // store's own otherwise, which it closes with itself.
  client?: ChatClient
  // Where the layout is remembered; the app's own keys otherwise.
  memory?: LayoutMemory
}

export class ChatStore {
  readonly client: ChatClient
  protected chat: ChatState
  // This root's toasts and confirm dialog (lib/shell).
  readonly shell: Shell
  ready = $state(false)
  currentId = $state<string | null>(null)
  outline = $state<OutlineLayout>({ open: true, width: 0 }) // the "On this page" panel
  // The side panel's tab ("On this page", "Artifacts" or "File"; its open
  // state and width are outline's).
  panel = $state<PanelLayout>({ tab: 'outline' })
  // The File tab's choices per session (lib/watched.ts), loaded as they're
  // first changed, and a request for its switcher (the File tab takes it).
  watched = $state<Record<string, Watched>>({})
  fileSwitcher = $state(false)
  questionCompact = $state(false) // the question header collapsed to one line
  headerCompact = $state(false) // the session header collapsed to two lines
  optionsOpen = $state(false) // the compact header's Options showing
  scope = $state<Scope>('project') // where "This is safe" last applied, for the next card
  reveal = $state<{ pageId: string; terms: string[] } | null>(null) // a page to scroll to a search match on
  preferHeadings = $state<Record<string, boolean>>({}) // page id → show its headings, not its summary
  private autoSummarised = new Set<string>() // tried once, so a failure isn't retried in a loop
  private offNotices: (() => void) | null = null
  private ownsClient: boolean
  private memory: LayoutMemory
  private offMemory: (() => void) | undefined

  // What only an app can do; an embedded chat leaves them out, and the
  // buttons that need them don't show.
  openSettings: ((section?: string) => void) | undefined = undefined
  openUsage: (() => void) | undefined = undefined
  // An embedding host's links: openLink sees every link in an answer first,
  // and true means the host took it (its own bruv:card:… links, say).
  openLink: ((href: string) => boolean) | undefined = undefined
  // In a host's page (the elements): the chat offers the host its slots.
  embedded = false
  // An embedding host's word on what the user has in front of them, asked
  // for as each turn is sent and told to the model with it (hidden).
  turnContext: (() => string | null | undefined) | undefined = undefined
  // An embedding host's @mentions: what matches what's typed after an @.
  complete: ((query: string) => Promise<Mention[]> | Mention[]) | undefined = undefined
  // An embedding host's word on what Enter does in the message box
  // (lib/enterkey.svelte.ts); undefined: the viewer's setting, or the device.
  enterKey = $state<EnterKey | undefined>(undefined)
  // In a host's page, where there may be no session list: New chat in the
  // header (newChat), and word to the host that the chat was deleted from
  // its header (deleted). Unset in the app, which has its sidebar.
  newChat: (() => void) | undefined = undefined
  deleted: ((id: string) => void) | undefined = undefined
  // An embedding host's own controls in the message box
  // (lib/composercontrols.ts), and whether it put anything in the
  // composer-controls slot.
  composerControls = $state.raw<ComposerControl[]>([])
  composerSlot = $state(false)
  // An embedding host's reviews of its own batches of changes, each a card
  // in its session's approval place (lib/hostreview.ts).
  hostReviews = $state.raw<HostReview[]>([])

  reviewsOf(sessionId: string): HostReview[] {
    return this.hostReviews.filter(r => r.session === sessionId)
  }

  // answerReview settles a host's review with the user's answer (the
  // element, which raised it, takes it off the list).
  answerReview(id: string, answer: ReviewAnswer) {
    this.hostReviews.find(x => x.id === id)?.resolve(answer)
  }

  constructor(backend: Backend, opts: ChatStoreOptions = {}) {
    this.client = opts.client ?? new ChatClient(backend)
    this.ownsClient = !opts.client
    this.chat = new ChatState(this.client)
    this.shell = opts.shell ?? new Shell()
    this.memory = opts.memory ?? appMemory
    this.apply(this.memory.load())
    // Another chat on the page with the same memory changed it.
    this.offMemory = this.memory.follow?.(l => this.apply(l))
  }

  private apply(l: ChatLayout) {
    this.outline = { ...l.outline }
    this.panel = { ...l.panel }
    this.questionCompact = l.questionCompact
    this.headerCompact = l.headerCompact
    this.optionsOpen = l.optionsOpen
    this.scope = l.scope
  }

  // ---- the client's state, as runes

  get backend(): Backend { return this.client.backend }
  get boot(): Bootstrap | null { return this.chat.boot }
  get cli() { return this.chat.cli }
  set cli(v) { this.client.patch({ cli: v }) }
  get providerStatus() { return this.chat.providerStatus }
  get progress() { return this.chat.progress } // a CLI download
  get containers() { return this.chat.containers }
  get remote() { return this.chat.remote }
  set remote(v) { this.client.patch({ remote: v }) }
  get connection() { return this.chat.connection }
  get usage() { return this.chat.usage }
  set usage(v) { this.client.patch({ usage: v }) }
  get sessions(): SessionView[] { return this.chat.sessions }

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

  // loaded follows the first state. Text about a session names its
  // provider with that session's names (namesFor), so two chats on servers
  // with different providers each say their own.
  protected async loaded() {
    this.ready = true
  }

  destroy() {
    this.offNotices?.()
    this.offMemory?.()
    if (this.ownsClient) this.client.destroy()
    this.chat.destroy()
  }

  resync() {
    return this.client.resync()
  }

  protected onNotice(n: Notice) {
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
      case 'resynced':
        if (this.currentId && !this.sessions.some(s => s.id === this.currentId)) this.currentId = null
        break
      case 'error':
        this.shell.toast(String(n.error), 'error')
        break
    }
  }

  // ---- the session shown

  async select(id: string) {
    this.currentId = id
    try {
      await this.client.session(id).load()
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
    if (this.outline.open) void this.refreshArtifacts(id)
    void this.loadCommands(id)
    await this.backend.focus(id)
  }

  goTo(i: number) {
    if (this.currentId) this.client.session(this.currentId).goTo(i)
  }

  async send(text: string, attachments: AttachmentRef[] = []) {
    if (!this.currentId) return
    const context = this.turnContext?.()?.trim()
    await this.client.session(this.currentId).send(text, attachments, context ? { context } : undefined)
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

  // ---- the side panel

  // togglePanel opens or closes the side panel, on the tab it had.
  togglePanel() {
    this.outline.open = !this.outline.open
    this.remember()
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
    this.remember()
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
    this.remember()
  }

  setOptionsOpen(open: boolean) {
    this.optionsOpen = open
    this.remember()
  }

  setScope(scope: Scope) {
    this.scope = scope
    this.remember()
  }

  toggleQuestionCompact() {
    this.questionCompact = !this.questionCompact
    this.remember()
  }

  setOutlineWidth(width: number) {
    this.outline.width = width
    this.remember()
  }

  // ---- MCP sign-in (decision 0020), with errors as toasts

  mcpSignIn = (sessionId: string, server: string) => this.toasting(this.client.session(sessionId).mcpSignIn(server))
  mcpReconnect = (sessionId: string, server: string) => this.toasting(this.client.session(sessionId).mcpReconnect(server))
  mcpToggle = (sessionId: string, server: string, on: boolean) => this.toasting(this.client.session(sessionId).mcpToggle(server, on))
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

  loadCommands(id: string) {
    return this.client.session(id).loadCommands()
  }

  // ---- the session itself

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

  // moveSession puts a session at a position in the list as shown (drag
  // and drop, or Alt+arrows).
  async moveSession(id: string, to: number) {
    try {
      await this.client.moveSession(id, to)
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  // remove deletes a session from UNCLI (its CLI's transcript is kept).
  async remove(id: string) {
    await this.client.remove(id)
    if (this.currentId === id) this.currentId = null
  }

  upsertSession(v: SessionView) {
    this.client.upsertSession(v)
  }

  upsertPage(p: Page) {
    this.client.upsertPage(p)
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

  // remember saves the layout where this chat remembers it.
  private remember() {
    this.memory.save({
      outline: { ...this.outline },
      panel: { ...this.panel },
      questionCompact: this.questionCompact,
      headerCompact: this.headerCompact,
      optionsOpen: this.optionsOpen,
      scope: this.scope,
    })
  }

  protected async toasting(p: Promise<unknown>) {
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
