// Svelte's view of a ChatClient (decision 0026): the client is plain
// TypeScript; this mirrors its state into runes. Each server-wide field is
// its own signal, set only when the client replaced it, and each session's
// data is a key of a SvelteMap, so one session's streaming text wakes only
// what reads that session.
import { SvelteMap } from 'svelte/reactivity'
import { EMPTY_SESSION, type ChatClient, type ClientState, type SessionData } from '@uncli/client'

type Fields = Omit<ClientState, 'data'>

export class ChatState {
  boot = $state.raw<Fields['boot']>(null)
  cli = $state.raw<Fields['cli']>(null)
  providerStatus = $state.raw<Fields['providerStatus']>({})
  progress = $state.raw<Fields['progress']>(null)
  containers = $state.raw<Fields['containers']>(null)
  remote = $state.raw<Fields['remote']>(null)
  devices = $state.raw<Fields['devices']>([])
  servers = $state.raw<Fields['servers']>([])
  pairRequest = $state.raw<Fields['pairRequest']>(null)
  connection = $state.raw<Fields['connection']>(null)
  update = $state.raw<Fields['update']>(null)
  usage = $state.raw<Fields['usage']>(null)
  sessions = $state.raw<Fields['sessions']>([])
  pins = $state.raw<Fields['pins']>([])
  private data = new SvelteMap<string, SessionData>()
  private last: ClientState | null = null
  private off: () => void

  constructor(client: ChatClient) {
    this.sync(client.snapshot())
    this.off = client.subscribe(s => this.sync(s))
  }

  // dataOf is a session's data, read reactively for that session alone.
  dataOf(id: string): SessionData {
    return this.data.get(id) ?? EMPTY_SESSION
  }

  destroy() {
    this.off()
  }

  private sync(s: ClientState) {
    const was = this.last
    this.last = s
    if (was?.boot !== s.boot) this.boot = s.boot
    if (was?.cli !== s.cli) this.cli = s.cli
    if (was?.providerStatus !== s.providerStatus) this.providerStatus = s.providerStatus
    if (was?.progress !== s.progress) this.progress = s.progress
    if (was?.containers !== s.containers) this.containers = s.containers
    if (was?.remote !== s.remote) this.remote = s.remote
    if (was?.devices !== s.devices) this.devices = s.devices
    if (was?.servers !== s.servers) this.servers = s.servers
    if (was?.pairRequest !== s.pairRequest) this.pairRequest = s.pairRequest
    if (was?.connection !== s.connection) this.connection = s.connection
    if (was?.update !== s.update) this.update = s.update
    if (was?.usage !== s.usage) this.usage = s.usage
    if (was?.sessions !== s.sessions) this.sessions = s.sessions
    if (was?.pins !== s.pins) this.pins = s.pins
    if (was?.data === s.data) return
    for (const [id, d] of Object.entries(s.data)) if (this.data.get(id) !== d) this.data.set(id, d)
    for (const id of [...this.data.keys()]) if (!(id in s.data)) this.data.delete(id)
  }
}

// keyed is a read-only record over a lookup: record[key] reads lookup(key)
// (reactively, when the lookup is), for code that still indexes by id.
export function keyed<T>(lookup: (key: string) => T | undefined): Readonly<Record<string, T>> {
  return new Proxy({} as Record<string, T>, {
    get: (_, k) => (typeof k === 'string' ? lookup(k) : undefined),
    has: (_, k) => typeof k === 'string' && lookup(k) !== undefined,
    set: () => {
      throw new Error('read-only: change it through the client')
    },
  })
}
