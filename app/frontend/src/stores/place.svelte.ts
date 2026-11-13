// Where a new session runs (decision 0031): this PC, or a paired server.
// Each has its own profiles, providers, models and containers; New session
// reads them from the place chosen, and starts the session there.
import { LOCAL, namesOf, providerOf, type Backend, type Bootstrap, type CLIStatus, type ContainersInfo, type ModelInfo, type ProviderNames } from '@uncli/chat/lib/api'
import type { AppStore } from './app.svelte'

export interface Place {
  key: string // LOCAL, or the server's key
  backend: Backend
  boot: Bootstrap | null
  containers: ContainersInfo | null
  statusOf(provider?: string): CLIStatus | null
  modelsFor(provider?: string): ModelInfo[] | null
  namesFor(adapter?: string): ProviderNames
  // reloadContainers reads its containers again (after an import or a
  // build starts); a server's are followed until its builds finish.
  reloadContainers(): Promise<void>
}

// thisPC is the app's own machine state.
export function thisPC(app: AppStore): Place {
  return {
    key: LOCAL,
    get backend() { return app.backend },
    get boot() { return app.boot },
    get containers() { return app.containers },
    statusOf: p => app.statusOf(p),
    modelsFor: p => app.modelsFor(p),
    namesFor: a => app.namesFor(a),
    reloadContainers: () => app.loadContainers().then(() => {}),
  }
}

// How often a server's containers are asked about while one builds: its
// container changes aren't sent to this PC (only its sessions are).
const BUILD_POLL_MS = 2000

// ServerPlace is a paired server's, loaded when it's chosen.
export class ServerPlace implements Place {
  boot = $state<Bootstrap | null>(null)
  containers = $state<ContainersInfo | null>(null)
  error = $state('')
  private statuses = $state<Record<string, CLIStatus>>({})

  constructor(readonly key: string, readonly backend: Backend) {}

  async load() {
    try {
      const boot = await this.backend.bootstrap()
      const all = await Promise.all(boot.providers.map(p => this.backend.providerStatus(p.id, false).catch(() => null)))
      const statuses: Record<string, CLIStatus> = {}
      boot.providers.forEach((p, i) => {
        const s = all[i]
        if (s) statuses[p.id] = s
      })
      this.statuses = statuses
      this.boot = boot
      if (boot.platform === 'windows') this.containers = await this.backend.containers().catch(() => null)
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e)
    }
  }

  async reloadContainers() {
    if (this.boot?.platform !== 'windows') return
    this.containers = await this.backend.containers().catch(() => this.containers)
    // A build there: asked again until it's done (or failed).
    if (this.containers?.containers.some(c => c.step) && !this.polling) {
      this.polling = true
      setTimeout(() => {
        this.polling = false
        void this.reloadContainers()
      }, BUILD_POLL_MS)
    }
  }

  private polling = false

  statusOf(provider?: string): CLIStatus | null {
    return this.statuses[provider || this.boot?.providers[0]?.id || ''] ?? null
  }

  modelsFor(provider?: string): ModelInfo[] | null {
    const first = this.boot?.providers[0]?.id
    if (!provider || provider === first) return this.boot?.models?.length ? this.boot.models : null
    return this.statuses[provider]?.models ?? []
  }

  namesFor(adapter?: string): ProviderNames {
    return namesOf(providerOf(this.boot, adapter))
  }
}
