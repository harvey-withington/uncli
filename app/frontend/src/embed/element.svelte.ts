// <uncli-chat>: one UNCLI chat in any web page (decision 0026). A small
// custom element around the same Svelte chat UNCLI's own app shows: it
// attaches a shadow root, adopts UNCLI's theme read as :host, and mounts
// Chat.svelte there on a ChatStore of its own. Every element given the
// same backend shares one client (one event stream).
//
//   const chat = document.createElement('uncli-chat')
//   chat.backend = httpBackend({ base, token: () => token })  // or any Backend
//   chat.session = '<session id>'
//
// Attributes: session, scheme ("light" or "dark"; the system's otherwise),
// storage-key (remember the side panel and compact layout under it).
// Events (composed, so they cross into the host's page): uncli-ready,
// uncli-busy, uncli-idle, uncli-needs-approval; detail: { session }.
import { mount, unmount } from 'svelte'
import { chatClient, type Backend, type SessionSnapshot } from '@uncli/client'
import themeCss from '../styles/theme.css?inline'
import Chat from '../components/Chat.svelte'
import { keyedMemory } from '../lib/layoutmemory'
import { ChatStore } from '../stores/chat.svelte'
import { shadowCss } from './shadow'

// One stylesheet, adopted by every element's shadow root.
let sheet: CSSStyleSheet | null = null
function stylesheet(): CSSStyleSheet | null {
  if (sheet || typeof CSSStyleSheet === 'undefined' || !('replaceSync' in CSSStyleSheet.prototype)) return sheet
  sheet = new CSSStyleSheet()
  sheet.replaceSync(shadowCss(themeCss))
  return sheet
}

export class UncliChat extends HTMLElement {
  static observedAttributes = ['session', 'scheme']

  #backend: Backend | null = null
  #props = $state<{ store: ChatStore; session: string } | null>(null)
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

  // The session shown (the session attribute).
  get session(): string {
    return this.getAttribute('session') ?? ''
  }

  set session(id: string) {
    this.setAttribute('session', id)
  }

  connectedCallback() {
    if (!this.shadowRoot) {
      const root = this.attachShadow({ mode: 'open' })
      const s = stylesheet()
      if (s) root.adoptedStyleSheets = [s]
      else {
        const style = document.createElement('style')
        style.textContent = shadowCss(themeCss)
        root.append(style)
      }
    }
    // The host page's font, for UNCLI's text unless the host sets --uncli-font.
    const font = getComputedStyle(this.parentElement ?? document.body).fontFamily
    if (font) this.style.setProperty('--uncli-host-font', font)
    this.#scheme()
    this.#mount()
  }

  disconnectedCallback() {
    this.#unmount()
  }

  attributeChangedCallback(name: string) {
    if (name === 'scheme') this.#scheme()
    if (name === 'session') {
      if (this.#props) this.#props.session = this.session
      this.#watch()
    }
  }

  // UNCLI's own scheme attribute, which the theme keys on.
  #scheme() {
    const s = this.getAttribute('scheme')
    if (s === 'light' || s === 'dark') this.setAttribute('data-uncli-scheme', s)
    else this.removeAttribute('data-uncli-scheme')
  }

  #mount() {
    if (this.#app || !this.isConnected || !this.#backend || !this.shadowRoot) return
    const store = new ChatStore(this.#backend, { client: chatClient(this.#backend), memory: keyedMemory(this.getAttribute('storage-key')) })
    const target = document.createElement('div')
    target.className = 'uncli-root'
    this.shadowRoot.append(target)
    this.#props = { store, session: this.session }
    this.#app = mount(Chat, { target, props: this.#props })
    store.init().then(() => this.#emit('uncli-ready'), () => {})
    this.#watch()
  }

  #unmount() {
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
    this.dispatchEvent(new CustomEvent(type, { bubbles: true, composed: true, detail: { session: this.session } }))
  }
}

// defineUncliChat registers <uncli-chat> (once).
export function defineUncliChat(name = 'uncli-chat') {
  if (!customElements.get(name)) customElements.define(name, UncliChat)
}
