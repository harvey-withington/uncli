// The app's store: the window's (or the phone's) shell around the chat
// (decision 0026). ChatStore is what the chat components need to show a
// session; this adds what only the app has: dialogs, search, the sidebar
// and its pinned pages, Settings, updates, the theme file, and starting on
// the session open last time.
import { namesOf, parseRef, providerOf, type Backend, type CLIStatus, type Notice, type PinnedPage, type SearchHit, type SearchResult, type Share, type Bootstrap } from '@uncli/chat/lib/api'
import { loadSidebarWidth, saveSidebarWidth } from '@uncli/chat/lib/panels'
import { loadLastSession, saveLastSession } from '@uncli/chat/lib/keep'
import type { SearchRange } from '@uncli/chat/lib/search'
import { setNames, t } from '@uncli/chat/lib/i18n.svelte'
import { applyThemeFile } from '../lib/hosttheme'
import { settingsAttention } from '../lib/attention'
import { loadGroups, saveGroups, toggled, type GroupFlag, type GroupStates } from '../lib/sidebargroups'
import { ChatStore, type ChatStoreOptions } from '@uncli/chat/stores/chat.svelte'

export { displayAnswer, type Live } from '@uncli/chat/stores/chat.svelte'

// Where a new session from a walkthrough starts, and what it starts with.
export interface NewSessionSeed {
  where: string // LOCAL or a paired server's key
  container: string // the container it ran in, '' for none
  prompt: string // its first message (lib/walkcontext.ts)
  title: string // its name; '' names it by the first message
}

export class AppStore extends ChatStore {
  newSessionOpen = $state(false)
  newSessionProfile = $state<string | null>(null) // profile the dialog opens on
  newSessionFolder = $state<string | null>(null) // folder it opens with (a dropped folder)
  newSessionSeq = $state(0) // each open is a fresh dialog, even mid fade-out
  newSessionSeed = $state<NewSessionSeed | null>(null) // a walkthrough's new session: where, and its first message
  settingsOpen = $state(false)
  usageOpen = $state(false) // the usage dashboard
  settingsAt = $state('') // a section to open Settings at ('safe', 'notify'…: #settings-<section>)
  paletteOpen = $state(false) // the command palette
  keysOpen = $state(false) // the keyboard map
  importOpen = $state(false) // the import picker (saved CLI conversations)
  sidebarWidth = $state(loadSidebarWidth())
  // Search across sessions: the box's text and filters (the sidebar shows
  // results instead of the session list while there is text), and a counter
  // that focuses the box (Ctrl+K).
  searchText = $state('')
  searchFilters = $state<{ profiles: string[]; bookmarked: boolean; thisSession: boolean; range: SearchRange }>({
    profiles: [], bookmarked: false, thisSession: false, range: 'any',
  })
  searchFocus = $state(0)
  search = $state<{ result: SearchResult | null; active: number }>({ result: null, active: 0 })
  // The sidebar's groups (This PC, each paired server): collapsed, archived
  // hidden, Archived open (lib/sidebargroups.ts).
  sidebarGroups = $state<GroupStates>(loadGroups())

  // openLast: start on the session open last time (the window); the phone
  // starts on its session list instead.
  private openLast: boolean

  // The app always has them.
  declare openSettings: (section?: string) => void
  declare openUsage: () => void

  constructor(backend: Backend, opts: ChatStoreOptions & { openLast?: boolean } = {}) {
    super(backend, opts)
    this.openLast = opts.openLast ?? true
    // The chat's ways into the app.
    this.openSettings = (section = '') => {
      this.settingsAt = section
      this.settingsOpen = true
    }
    this.openUsage = () => {
      this.usageOpen = true
    }
    // A walkthrough's New session: the dialog opens in the session's place
    // (its PC, type, folder and container), and Start sends the context.
    this.newSessionFrom = (id, prompt, title) => {
      const s = this.sessions.find(x => x.id === id)
      if (!s) return
      this.openNewSession(s.profileId, s.workdir || null, {
        where: parseRef(id).server, container: s.runtime === 'wsl' ? (s.runtimeRef ?? '') : '', prompt, title,
      })
    }
  }

  get devices() { return this.chat.devices }
  // The servers this PC is paired with (decision 0031).
  get servers() { return this.chat.servers }
  set servers(v) { this.client.patch({ servers: v }) }
  groupState(key: string) {
    return this.sidebarGroups[key] ?? {}
  }

  // toggleGroup flips (or sets) one of a sidebar group's flags, remembered.
  toggleGroup(key: string, flag: GroupFlag, on?: boolean) {
    this.sidebarGroups = toggled(this.sidebarGroups, key, flag, on)
    saveGroups(this.sidebarGroups)
  }

  // toggleArchived opens or closes a group's Archived list, showing the
  // group and its archived sessions first if they were hidden.
  toggleArchived(key: string) {
    const g = this.groupState(key)
    const open = g.collapsed || g.hideArchived ? true : !g.archivedOpen
    this.sidebarGroups = { ...this.sidebarGroups, [key]: { ...g, collapsed: false, hideArchived: false, archivedOpen: open } }
    saveGroups(this.sidebarGroups)
  }

  loadServers() {
    return this.client.loadServers()
  }
  set devices(v) { this.client.patch({ devices: v }) }
  get pairRequest() { return this.chat.pairRequest }
  set pairRequest(v) { this.client.patch({ pairRequest: v }) }
  get update() { return this.chat.update }
  get pins(): PinnedPage[] { return this.chat.pins }

  // patchBoot changes what the server said at start (a setting saved).
  patchBoot(p: Partial<Bootstrap>) {
    if (this.boot) this.client.patch({ boot: { ...this.boot, ...p } })
  }

  setProviderStatus(provider: string, st: CLIStatus) {
    this.client.patch({ providerStatus: { ...this.providerStatus, [provider]: st } })
  }

  // What in Settings needs the user, per tab (lib/attention.ts).
  attention = $derived(settingsAttention(this.containers))

  // loaded follows the first state: the theme file, then (the window) the
  // session open last time, else the first.
  protected override async loaded() {
    // The app's own screens (setup, import) talk to its one server: its
    // provider's names are their default.
    setNames(namesOf(providerOf(this.boot)))
    await super.loaded()
    void this.loadThemeFile(false)
    if (!this.openLast) return
    const last = loadLastSession()
    const first = this.activeSessions.find(s => s.id === last) ?? this.activeSessions[0]
    if (first) await this.select(first.id)
  }

  protected override onNotice(n: Notice) {
    if (n.kind === 'notifyOpen') void this.openFromNotification(n.sessionId)
    else super.onNotice(n)
  }

  override async select(id: string) {
    saveLastSession(id)
    await super.select(id)
  }

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

  // openNewSession opens the new-session dialog, on a given profile if one
  // was picked (the welcome cards), else on the first; seed starts it from
  // a walkthrough.
  openNewSession(profileId: string | null = null, folder: string | null = null, seed: NewSessionSeed | null = null) {
    this.newSessionProfile = profileId
    this.newSessionFolder = folder
    this.newSessionSeed = seed
    this.newSessionSeq++
    this.newSessionOpen = true
  }

  setSidebarWidth(width: number) {
    this.sidebarWidth = width
    saveSidebarWidth(width)
  }

  loadProviders(fresh = false) {
    return this.client.loadProviders(fresh)
  }

  loadContainers() {
    return this.client.loadContainers()
  }

  // createSession starts a session on this PC, or on a paired server
  // through its backend (decision 0031).
  async createSession(profileId: string, workdir: string, model: string, container = '', provider = '', shares: Share[] | null = null, on: Backend = this.backend, title = '') {
    const v = await this.client.createSession(profileId, workdir, model, container, provider, shares, '', title, on)
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

  async refreshPins() {
    try {
      await this.client.refreshPins()
    } catch (e) {
      this.shell.toast(String(e), 'error')
    }
  }

  // remove deletes a session; the window then shows the first one left.
  override async remove(id: string) {
    const shown = this.currentId === id
    await super.remove(id)
    const next = this.activeSessions[0]
    if (shown && next) await this.select(next.id)
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
}
