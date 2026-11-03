// <uncli-session-list>: a UNCLI server's sessions as a list any web page
// can show (decision 0026), beside an <uncli-chat> say. Reorder, rename,
// archive and delete work as in UNCLI's sidebar; picking a session tells
// the host with an uncli-select event, and the host shows it.
//
//   list.backend = chat.backend                  // the same backend: one client
//   list.addEventListener('uncli-select', e => (chat.session = e.detail.session))
//
// Attributes: selected (the session the host shows, marked as current),
// scheme ("light" or "dark"; the system's otherwise).
// Events (composed): uncli-ready, uncli-select; detail: { session }.
import { mount, unmount } from 'svelte'
import { chatClient, type Backend } from '@uncli/client'
import SessionList from '../components/SessionList.svelte'
import { keyedMemory } from '../lib/layoutmemory'
import { ChatStore } from '../stores/chat.svelte'
import { applyScheme, hostFont, shadowFor } from './root'

export class UncliSessionList extends HTMLElement {
  static observedAttributes = ['selected', 'scheme']

  #backend: Backend | null = null
  #props = $state<{ store: ChatStore; selected: string; onpick: (id: string) => void } | null>(null)
  #app: ReturnType<typeof mount> | null = null

  get backend(): Backend | null {
    return this.#backend
  }

  set backend(b: Backend | null) {
    if (b === this.#backend) return
    this.#backend = b
    this.#unmount()
    this.#mount()
  }

  // The session the host shows (the selected attribute).
  get selected(): string {
    return this.getAttribute('selected') ?? ''
  }

  set selected(id: string) {
    this.setAttribute('selected', id)
  }

  connectedCallback() {
    shadowFor(this)
    hostFont(this)
    applyScheme(this)
    this.#mount()
  }

  disconnectedCallback() {
    this.#unmount()
  }

  attributeChangedCallback(name: string) {
    if (name === 'scheme') applyScheme(this)
    if (name === 'selected' && this.#props) this.#props.selected = this.selected
  }

  #mount() {
    if (this.#app || !this.isConnected || !this.#backend || !this.shadowRoot) return
    const store = new ChatStore(this.#backend, { client: chatClient(this.#backend), memory: keyedMemory(null) })
    const target = document.createElement('div')
    target.className = 'uncli-root'
    this.shadowRoot.append(target)
    this.#props = { store, selected: this.selected, onpick: id => this.#emit('uncli-select', id) }
    this.#app = mount(SessionList, { target, props: this.#props })
    store.init().then(() => this.#emit('uncli-ready', ''), () => {})
  }

  #unmount() {
    if (this.#app) void unmount(this.#app)
    this.#app = null
    this.#props?.store.destroy()
    this.#props = null
    this.shadowRoot?.querySelector('.uncli-root')?.remove()
  }

  #emit(type: string, session: string) {
    this.dispatchEvent(new CustomEvent(type, { bubbles: true, composed: true, detail: { session } }))
  }
}

// defineUncliSessionList registers <uncli-session-list> (once).
export function defineUncliSessionList(name = 'uncli-session-list') {
  if (!customElements.get(name)) customElements.define(name, UncliSessionList)
}
