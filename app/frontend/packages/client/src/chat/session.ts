// SessionStore: one session of a ChatClient (decision 0026). Its view,
// pages, the page shown, the text still streaming, and what can be done
// to it, with no session argument: it is that session. subscribe() is
// told only when this session changes, so a UI showing one chat isn't
// woken by another's streaming text.
import type { AttachmentRef, Page, SessionView } from '../types'
import type { ChatClient } from './client'
import type { SessionData } from './state'

export interface SessionSnapshot extends SessionData {
  id: string
  view: SessionView | null
  page: Page | null // the page shown
}

export class SessionStore {
  private listeners = new Set<(s: SessionSnapshot) => void>()
  private snap: SessionSnapshot | null = null
  private loading: Promise<void> | null = null

  constructor(
    private readonly client: ChatClient,
    readonly id: string,
  ) {}

  // snapshot is the session now; the same object until it changes.
  snapshot(): SessionSnapshot {
    const view = this.client.viewOf(this.id)
    const data = this.client.dataOf(this.id)
    const s = this.snap
    if (s && s.view === view && s.pages === data.pages && s.index === data.index && s.live === data.live && s.commands === data.commands &&
      s.commandsPending === data.commandsPending && s.artifactSel === data.artifactSel && s.artifactFiles === data.artifactFiles &&
      s.mcpLinks === data.mcpLinks && s.summarising === data.summarising) return s
    this.snap = { ...data, id: this.id, view, page: data.pages?.[data.index] ?? null }
    return this.snap
  }

  subscribe(fn: (s: SessionSnapshot) => void): () => void {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }

  get view(): SessionView | null {
    return this.client.viewOf(this.id)
  }

  get data(): SessionData {
    return this.client.dataOf(this.id)
  }

  // ---- pages

  // load fetches the session's pages the first time, and shows the latest.
  load(): Promise<void> {
    if (this.data.pages) return Promise.resolve()
    this.loading ??= this.reload().finally(() => (this.loading = null))
    return this.loading
  }

  // reload fetches the pages afresh, showing the latest.
  async reload() {
    const pages = await this.client.backend.pages(this.id)
    this.client.setData(this.id, { pages, index: Math.max(0, pages.length - 1) })
  }

  // goTo shows page i.
  goTo(i: number) {
    this.client.setData(this.id, { index: i })
  }

  // show shows a page by id, fetching the pages afresh if it isn't among
  // them yet; false if it doesn't exist.
  async show(pageId: string): Promise<boolean> {
    if (!this.data.pages?.some(p => p.id === pageId)) await this.reload()
    const i = this.data.pages?.findIndex(p => p.id === pageId) ?? -1
    if (i >= 0) this.goTo(i)
    return i >= 0
  }

  // showLatest shows the newest page (where an answer or approval is).
  showLatest() {
    const n = this.data.pages?.length ?? 0
    if (n > 0) this.goTo(n - 1)
  }

  async send(text: string, attachments: AttachmentRef[] = []) {
    await this.client.backend.send(this.id, text, attachments)
  }

  async interrupt() {
    await this.client.backend.interrupt(this.id)
  }

  async setBookmark(pageId: string, on: boolean) {
    this.client.upsertPage(await this.client.backend.setBookmark(this.id, pageId, on))
  }

  // toggleBookmark bookmarks or un-bookmarks the page shown.
  async toggleBookmark() {
    const p = this.snapshot().page
    if (p) await this.setBookmark(p.id, !p.bookmarked)
  }

  async setPinned(pageId: string, on: boolean) {
    this.client.upsertPage(await this.client.backend.setPinned(this.id, pageId, on))
    await this.client.refreshPins()
  }

  // togglePin pins or unpins the page shown.
  async togglePin() {
    const p = this.snapshot().page
    if (p) await this.setPinned(p.id, !p.pinned)
  }

  // summarise asks the quick-task model for a page's table of contents,
  // given the blocks the UI split its answer into.
  async summarise(pageId: string, blocks: string[]) {
    if (this.data.summarising[pageId]) return
    this.client.setData(this.id, { summarising: { ...this.data.summarising, [pageId]: true } })
    try {
      this.client.upsertPage(await this.client.backend.summarisePage(this.id, pageId, blocks))
    } finally {
      const rest = { ...this.data.summarising }
      delete rest[pageId]
      this.client.setData(this.id, { summarising: rest })
    }
  }

  // ---- the session itself

  // restart stops its CLI and starts it again on its conversation. An
  // answer in progress is cut short: the UI asks first.
  async restart() {
    this.client.setData(this.id, { live: null })
    this.client.upsertSession(await this.client.backend.restart(this.id))
  }

  archive(on: boolean) {
    return this.client.archive(this.id, on)
  }

  // ---- slash commands

  // loadCommands fetches the slash commands its CLI takes. None known yet
  // means the backend has started the CLI to report them: they come with a
  // 'commands' event (pending until then, or 20 s).
  async loadCommands() {
    try {
      const list = await this.client.backend.commands(this.id)
      const v = this.view
      const pending = list.length === 0 && !!v && this.client.takesSlash(v)
      if (pending && !this.data.commandsPending) setTimeout(() => this.client.setData(this.id, { commandsPending: false }), 20000)
      this.client.setData(this.id, { commands: list, commandsPending: pending || (this.data.commandsPending && list.length === 0) })
    } catch {
      this.client.setData(this.id, { commandsPending: false }) // none offered; typing a command still works
    }
  }

  // ---- artifacts

  // refreshArtifacts reads what is in its artifacts folder now.
  async refreshArtifacts() {
    if (!this.client.hasArtifacts(this.id)) return
    this.client.setData(this.id, { artifactFiles: await this.client.backend.artifactFiles(this.id) })
  }

  selectArtifact(path: string) {
    this.client.setData(this.id, { artifactSel: path })
  }

  // ---- MCP sign-in (decision 0020)

  // mcpSignIn opens a server's sign-in in the browser; a connector of the
  // provider's own is signed in on its site, and the backend reconnects it
  // until it connects. A notice says when a server acted on connects.
  async mcpSignIn(server: string) {
    const key = `${this.id}:${server}`
    try {
      this.client.mcpActed.add(key)
      const r = await this.client.backend.mcpSignIn(this.id, server)
      if (r.url) {
        this.client.setData(this.id, { mcpLinks: { ...this.data.mcpLinks, [server]: r.url } })
        await this.client.backend.openURL(r.url)
      }
    } catch (e) {
      this.client.mcpActed.delete(key)
      throw e
    }
  }

  async mcpReconnect(server: string) {
    this.client.mcpActed.add(`${this.id}:${server}`)
    await this.client.backend.mcpReconnect(this.id, server)
  }

  async mcpSignOut(server: string) {
    await this.client.backend.mcpSignOut(this.id, server)
  }

  async mcpCancel(server: string) {
    this.client.mcpActed.delete(`${this.id}:${server}`)
    await this.client.backend.mcpCancel(this.id, server)
  }

  // mcpPaste hands on the address the browser ended up on.
  async mcpPaste(server: string, url: string) {
    await this.client.backend.mcpCallback(this.id, server, url.trim())
  }

  // ---- internal to the package

  /** @internal */
  changed() {
    if (this.listeners.size === 0) return
    const s = this.snapshot()
    for (const fn of this.listeners) fn(s)
  }
}
