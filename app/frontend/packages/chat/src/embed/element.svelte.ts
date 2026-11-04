// <uncli-chat>: one UNCLI chat in any web page (decision 0026). A small
// custom element around the same Svelte chat UNCLI's own app shows: it
// attaches a shadow root, adopts UNCLI's theme read as :host, and mounts
// Chat.svelte there on a ChatStore of its own. Every element given the
// same backend shares one client (one event stream).
//
//   <uncli-chat src="/uncli" session="<session id>"></uncli-chat>
//
// Attributes: src (the path the host's server forwards to its own UNCLI,
// decision 0027; or set the backend property to any Backend), session, scheme ("light" or "dark"; the system's otherwise),
// compact (the phone's layout; compact="false" the desktop's; by the
// element's width when left out), storage-key (remember the side panel and
// compact layout under it), link-schemes (the host's own link schemes kept
// in answers, "bruv" say, for host.openLink).
// Property host (ChatHost): openLink(href) sees every link in an answer
// first, true when the host took it; t(key, params) gives the host's text
// for UNCLI's, or nothing to keep UNCLI's; complete(query) offers
// @mentions; turnContext() says what the user has in front of them, sent
// (hidden) with each turn; headers() is the host's own sign-in, sent with
// every request to src; enter says what Enter does in the message box.
// Events (composed, so they cross into the host's page): uncli-ready,
// uncli-busy, uncli-idle, uncli-needs-approval, uncli-missing (no such
// session here), uncli-deleted (deleted from the header); detail:
// { session }. New chat in the header sends uncli-new-chat, cancelable (a
// host that cancels makes the session itself), then uncli-new-session,
// detail: { session, previous }. Before
// the chat acts on Escape or Ctrl+Enter, uncli-escape and uncli-submit go
// out, cancelable: a host that cancels one keeps the key for itself.
import { mount, unmount } from 'svelte'
import { chatClient, setHostHeaders, type Backend, type SessionSnapshot } from '@uncli/client'
import Chat from '../components/Chat.svelte'
import { setHostText } from '../lib/i18n.svelte'
import { keyedMemory } from '../lib/layoutmemory'
import { keepInMemory } from '../lib/storage'
import { allowLinkSchemes } from '../lib/render/markdown'
import { ChatStore, type Mention } from '../stores/chat.svelte'
import type { EnterKey } from '../lib/enterkey.svelte'
import type { ComposerControl } from '../lib/composercontrols'
import { applyScheme, hostFont, shadowFor, srcBackend } from './root'

// What a host can do for the chat (decision 0026).
export interface ChatHost {
  // openLink: a link in an answer was clicked; true when the host took it.
  openLink?(href: string): boolean
  // t: the host's text for one of UNCLI's strings, or nothing to keep UNCLI's.
  t?(key: string, params?: Record<string, string | number>): string | undefined | null
  // complete: what matches an @mention being typed (the word after the @).
  complete?(query: string): Promise<Mention[]> | Mention[]
  // turnContext: what the user has in front of them now, asked for as each
  // turn is sent and told to the model with it, hidden in the chat.
  turnContext?(): string | undefined | null
  // headers: what the host's server needs on each request to src, its own
  // sign-in (Authorization: Bearer <token>), for a host that doesn't sign in
  // with a cookie. Asked for on every request.
  headers?(): Record<string, string> | undefined | null
  // enter: what Enter does in the message box, to match the host's own
  // chat: "send" (Shift+Enter for a new line) or "newline" (Ctrl+Enter
  // sends). Left out: the viewer's setting, or a new line on touch screens.
  enter?: EnterKey
  // composerControls: the host's own controls in the message box (buttons,
  // toggles, drop-downs; lib/composercontrols.ts), drawn by UNCLI. Set host
  // again with new values to change them. Anything else goes in the
  // composer-controls slot.
  composerControls?: ComposerControl[]
}

export class UncliChat extends HTMLElement {
  static observedAttributes = ['src', 'session', 'scheme', 'compact', 'link-schemes']

  #backend: Backend | null = null
  #host: ChatHost | null = null
  #props = $state<{ store: ChatStore; session: string; compact: boolean | undefined } | null>(null)
  #app: ReturnType<typeof mount> | null = null
  #offSession: (() => void) | null = null

  // The backend to reach UNCLI through: the HTTP one for a UNCLI server,
  // or any other Backend.
  get backend(): Backend | null {
    return this.#backend
  }

  set backend(b: Backend | null) {
    if (b === this.#backend) return
    this.#backend = b
    this.#unmount()
    this.#mount()
  }

  // What the host does for the chat: its links and its strings.
  get host(): ChatHost | null {
    return this.#host
  }

  set host(h: ChatHost | null) {
    this.#host = h
    setHostText(h?.t ? (key, params) => h.t?.(key, params) : null)
    setHostHeaders(h?.headers ? () => h.headers?.() : null)
    if (this.#props) {
      this.#props.store.enterKey = h?.enter
      this.#props.store.composerControls = h?.composerControls ?? []
    }
  }

  // The session shown (the session attribute).
  get session(): string {
    return this.getAttribute('session') ?? ''
  }

  set session(id: string) {
    this.setAttribute('session', id)
  }

  // newSession starts a session on this chat's UNCLI and shows it: a chat in
  // a fresh folder by default, or a profile's (cowork, code) in workdir.
  // opts.meta is the host's word on what it's for (a card, say), which the
  // host's server reads back with core.SessionMeta, from the session's very
  // first start (MCPServers).
  async newSession(profile = 'chat', workdir = '', model = '', opts: { meta?: string } = {}): Promise<string> {
    if (!this.#backend) throw new Error('<uncli-chat> has no src or backend')
    const v = await chatClient(this.#backend).createSession(profile, workdir, model, '', '', null, opts.meta ?? '')
    this.session = v.id
    return v.id
  }

  connectedCallback() {
    shadowFor(this)
    hostFont(this)
    applyScheme(this)
    this.#linkSchemes()
    this.shadowRoot?.addEventListener('keydown', this.#keys, true)
    // The host may put its own controls in the composer-controls slot later.
    this.#children.observe(this, { childList: true })
    this.#mount()
  }

  #children = new MutationObserver(() => {
    if (this.#props) this.#props.store.composerSlot = this.#hasComposerSlot()
  })

  #hasComposerSlot(): boolean {
    return [...this.children].some(c => c.getAttribute('slot') === 'composer-controls')
  }

  disconnectedCallback() {
    this.shadowRoot?.removeEventListener('keydown', this.#keys, true)
    this.#children.disconnect()
    this.#unmount()
  }

  attributeChangedCallback(name: string) {
    if (name === 'src') this.backend = srcBackend(this)
    if (name === 'scheme') applyScheme(this)
    if (name === 'link-schemes') this.#linkSchemes()
    if (name === 'compact' && this.#props) this.#props.compact = this.#compact()
    if (name === 'session') {
      if (this.#props) this.#props.session = this.session
      this.#watch()
      this.#checkMissing()
    }
  }

  #linkSchemes() {
    const s = this.getAttribute('link-schemes')
    if (s) allowLinkSchemes(s.split(/[\s,]+/))
  }

  // Escape and Ctrl+Enter (or Cmd+Enter) go to the host first: cancelling
  // the event keeps the key from the chat (a host's layered dialogs, say).
  #keys = (e: Event) => {
    const k = e as KeyboardEvent
    const type = k.key === 'Escape' ? 'uncli-escape' : k.key === 'Enter' && (k.ctrlKey || k.metaKey) ? 'uncli-submit' : ''
    if (!type) return
    const go = this.dispatchEvent(new CustomEvent(type, { bubbles: true, composed: true, cancelable: true, detail: { session: this.session } }))
    if (!go) {
      k.preventDefault()
      k.stopPropagation()
    }
  }

  // compact: the attribute present means yes, compact="false" no, absent
  // lets the width decide.
  #compact(): boolean | undefined {
    const c = this.getAttribute('compact')
    return c === null ? undefined : c !== 'false'
  }

  // The chat connects a moment after it has a src (or backend), not at
  // once: a host that sets src in its markup and host (with its sign-in
  // headers) in its script, in the same go, gets both in place before the
  // first request.
  #pending: ReturnType<typeof setTimeout> | null = null

  #mount() {
    if (this.#app || this.#pending || !this.isConnected || !this.#backend) return
    this.#pending = setTimeout(() => {
      this.#pending = null
      this.#mountNow()
    }, 0)
  }

  #mountNow() {
    if (this.#app || !this.isConnected || !this.#backend || !this.shadowRoot) return
    const store = new ChatStore(this.#backend, { client: chatClient(this.#backend), memory: keyedMemory(this.getAttribute('storage-key')) })
    store.openLink = href => this.#host?.openLink?.(href) ?? false
    store.turnContext = () => this.#host?.turnContext?.()
    store.complete = query => this.#host?.complete?.(query) ?? []
    store.enterKey = this.#host?.enter
    store.composerControls = this.#host?.composerControls ?? []
    store.composerSlot = this.#hasComposerSlot()
    store.embedded = true
    store.newChat = () => void this.#newChat()
    store.deleted = id => this.#emitFor('uncli-deleted', { session: id })
    const target = document.createElement('div')
    target.className = 'uncli-root'
    this.shadowRoot.append(target)
    this.#props = { store, session: this.session, compact: this.#compact() }
    this.#app = mount(Chat, { target, props: this.#props })
    store.init().then(() => {
      this.#emit('uncli-ready')
      this.#checkMissing()
    }, () => {})
    this.#watch()
  }

  // New chat from the header: the host may take it (cancel uncli-new-chat,
  // then call newSession with its own meta); otherwise a new session of the
  // same type, in the same folder unless the type makes a fresh one, shown
  // here, and the host told with uncli-new-session.
  async #newChat() {
    const previous = this.session
    if (!this.dispatchEvent(new CustomEvent('uncli-new-chat', { bubbles: true, composed: true, cancelable: true, detail: { session: previous } }))) return
    const store = this.#props?.store
    const cur = store?.sessions.find(s => s.id === previous)
    const profile = cur?.profileId ?? 'chat'
    const scratch = store?.boot?.profiles.find(p => p.id === profile)?.folder === 'scratch'
    const id = await this.newSession(profile, scratch ? '' : (cur?.workdir ?? ''))
    this.#emitFor('uncli-new-session', { session: id, previous })
  }

  // The host asked for a session this UNCLI doesn't have (deleted, say):
  // uncli-missing, so the host can start a new one.
  #checkMissing() {
    const store = this.#props?.store
    const id = this.session
    if (store?.ready && id && !store.sessions.some(s => s.id === id)) this.#emit('uncli-missing')
  }

  #unmount() {
    if (this.#pending) clearTimeout(this.#pending)
    this.#pending = null
    this.#offSession?.()
    this.#offSession = null
    if (this.#app) void unmount(this.#app)
    this.#app = null
    this.#props?.store.destroy()
    this.#props = null
    this.shadowRoot?.querySelector('.uncli-root')?.remove()
  }

  // The session's busy and approval changes, told to the host as events.
  #watch() {
    this.#offSession?.()
    this.#offSession = null
    const store = this.#props?.store
    const id = this.session
    if (!store || !id) return
    let busy = false
    let asking = 0
    const seen = (s: SessionSnapshot) => {
      const v = s.view
      if (!v) return
      if (v.busy !== busy) this.#emit(v.busy ? 'uncli-busy' : 'uncli-idle')
      busy = v.busy
      const n = v.approvals?.length ?? 0
      if (n > asking) this.#emit('uncli-needs-approval')
      asking = n
    }
    const session = store.client.session(id)
    seen(session.snapshot())
    this.#offSession = session.subscribe(seen)
  }

  #emit(type: string) {
    this.#emitFor(type, { session: this.session })
  }

  #emitFor(type: string, detail: Record<string, string>) {
    this.dispatchEvent(new CustomEvent(type, { bubbles: true, composed: true, detail }))
  }
}

// defineUncliChat registers <uncli-chat> (once).
export function defineUncliChat(name = 'uncli-chat') {
  // A page that shows UNCLI's elements is a host's: what the chat remembers
  // stays in memory, not the host's storage (lib/storage.ts).
  keepInMemory()
  if (!customElements.get(name)) customElements.define(name, UncliChat)
}
