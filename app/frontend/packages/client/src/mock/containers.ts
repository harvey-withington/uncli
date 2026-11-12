// Containers in the browser mock. ?containers=nowsl starts without WSL,
// ?containers=hung with WSL not responding, ?containers=built with the sandbox built and signed in; otherwise WSL is
// on and nothing is built. Building walks through the steps on timers.
import type { ContainerInfo, ContainerProfile, ContainerStep, ContainersInfo, SuggestedContainer, Suggestions } from '../types'

const STEPS: ContainerStep[] = ['download', 'remove', 'import', 'packages', 'setup', 'cli', 'lockdown', 'check']
// The mock's user has no secrets folder yet, so a container sharing it says so.
const SECRETS = 'C:\\Users\\you\\.secrets'

// A repository whose .uncli suggests two containers (decision 0028): one
// that reaches outside itself, one that stays inside.
const REPO = 'C:\\Users\\you\\projects\\demo' // where the mock's Choose… goes
const SUGGESTED: ContainerProfile[] = [
  {
    id: 'team-dev', label: 'Team dev', description: 'Node and Go, with the tools this repository builds with.', base: 'alpine-3.24',
    packages: ['nodejs', 'npm', 'go', 'make'], setup: ['npm i -g pnpm', 'curl -fsSL https://example.com/install.sh | sh'],
    brain: 'shared', mcp: 'none', connectors: 'none', secrets: 'shared', folders: [], local: ['node_modules'],
  },
  {
    id: 'docs', label: 'Docs', description: 'Python and MkDocs, for the documentation.', base: 'alpine-3.24',
    packages: ['python3', 'py3-pip'], setup: ['pip install mkdocs'], brain: 'sandboxed', mcp: 'none', connectors: 'none', secrets: 'none', folders: [],
  },
]
// What a container builds and shares, to compare (names and descriptions don't count).
const canon = (p: ContainerProfile) => JSON.stringify({
  base: p.base, packages: p.packages, setup: p.setup, brain: p.brain || 'sandboxed', mcp: p.mcp || 'none',
  connectors: p.connectors || 'none', secrets: p.secrets === 'shared' ? 'shared' : '', folders: p.folders ?? [], local: p.local ?? [],
})
const reachesOf = (p: ContainerProfile): SuggestedContainer['reaches'] => [
  ...(p.brain === 'shared' ? ['brain' as const] : []), ...(p.mcp === 'shared' ? ['mcp' as const] : []),
  ...(p.connectors === 'shared' ? ['connectors' as const] : []), ...(p.secrets === 'shared' ? ['secrets' as const] : []),
  ...(p.folders?.length ? ['folders' as const] : []),
]

export function mockContainers(emit: (c: ContainersInfo) => void) {
  const mode = new URLSearchParams(location.search).get('containers')
  const state: ContainersInfo = {
    wsl: { unresponsive: mode === 'hung', installed: mode !== 'nowsl' && mode !== 'hung', version: mode === 'nowsl' ? undefined : '3.0.1.0', kernel: mode === 'nowsl' ? undefined : '6.18.40.1-1', distros: [] },
    signedIn: mode === 'built',
    bases: [{ id: 'alpine-3.24', label: 'Alpine Linux 3.24' }],
    containers: [
      { id: 'sandbox', label: 'Sandbox', description: 'Linux with the usual tools, your memories and your MCP servers. A session there sees only its own folder.', base: 'alpine-3.24', baseLabel: 'Alpine Linux 3.24', brain: 'shared', mcp: 'shared', connectors: 'shared', packages: ['coreutils', 'findutils', 'grep', 'sed', 'less', 'curl', 'jq', 'ripgrep', 'make', 'nodejs', 'npm', 'python3', 'py3-pip'], setup: [], builtin: true, edited: false, built: mode === 'built', changes: mode === 'built' ? ['packages'] : [] },
      {
        id: 'node-dev', label: 'Node', base: 'alpine-3.24', baseLabel: 'Alpine Linux 3.24', packages: ['nodejs', 'npm'], setup: [], builtin: false, edited: true, built: false,
        secrets: 'shared', folders: [{ source: '%USERPROFILE%\\notes', name: 'notes', access: 'ro' }], missing: [SECRETS],
      },
    ],
  }
  // The built-in ones as shipped, for Reset; what each was built from, for changes.
  const shipped = new Map(state.containers.filter(c => c.builtin).map(c => [c.id, structuredClone(c)]))
  const builtFrom = new Map<string, { packages: string[]; setup: string[] }>()
  const changesOf = (c: ContainerInfo): ContainerInfo['changes'] => {
    const was = builtFrom.get(c.id)
    if (!c.built || !was) return c.changes
    return [
      ...(was.packages.join() !== [...c.packages].sort().join() ? ['packages' as const] : []),
      ...(JSON.stringify(was.setup) !== JSON.stringify(c.setup) ? ['steps' as const] : []),
    ]
  }
  if (mode === 'built') state.wsl.distros = ['uncli-sandbox']
  const snap = () => structuredClone(state)
  const send = () => emit(snap())
  async function save(p: ContainerProfile) {
    if (!p.label) throw new Error('a container needs a name')
    const base = state.bases.find(b => b.id === p.base)
    if (!base) throw new Error(`no base ${p.base}`)
    const i = state.containers.findIndex(c => c.id === p.id)
    const old = state.containers[i]
    const next: ContainerInfo = {
      ...p, baseLabel: base.label, builtin: shipped.has(p.id), edited: true, built: old?.built ?? false, changes: old?.changes,
      missing: p.secrets === 'shared' ? [SECRETS] : [],
    }
    next.changes = changesOf(next)
    if (old) state.containers[i] = next
    else state.containers.push(next)
    send()
  }
  return {
    async containers() { return snap() },
    async installWSL() {
      await new Promise(r => setTimeout(r, 600))
      throw new Error('Windows has to restart before WSL works. Restart, then open Settings again.')
    },
    async stopContainers() {
      await new Promise(r => setTimeout(r, 400))
      state.wsl = { installed: true, version: '3.0.1.0', kernel: '6.18.40.1-1', distros: [] }
      send()
    },
    async buildContainer(id: string) {
      const c = state.containers.find(x => x.id === id)
      if (!c) throw new Error(`no container ${id}`)
      c.built = false
      c.error = undefined
      STEPS.forEach((s, i) => setTimeout(() => { c.step = s; send() }, i * 500))
      setTimeout(() => {
        c.step = undefined
        c.built = true
        c.changes = []
        builtFrom.set(id, { packages: [...c.packages].sort(), setup: [...c.setup] })
        state.wsl.distros = [...new Set([...state.wsl.distros, `uncli-${id}`])]
        send()
      }, STEPS.length * 500)
      c.step = 'download'
      send()
    },
    async removeContainer(id: string) {
      const c = state.containers.find(x => x.id === id)
      if (c) c.built = false
      state.wsl.distros = state.wsl.distros.filter(d => d !== `uncli-${id}`)
      send()
    },
    saveContainer: (p: ContainerProfile) => save({ ...p, from: state.containers.find(c => c.id === p.id)?.from }),
    async suggestedContainers(folder: string): Promise<Suggestions> {
      if (!folder.toLowerCase().startsWith(REPO.toLowerCase())) return { containers: [] }
      return {
        repo: REPO, name: 'demo',
        containers: SUGGESTED.map(p => {
          const yours = state.containers.find(c => c.from?.repo === REPO && c.from.id === p.id)
          const s: SuggestedContainer = {
            profile: structuredClone(p), status: 'new', reaches: reachesOf(p),
            steps: p.setup.filter(st => st.includes('| sh')).map(step => ({ step, why: ['runs-code'] })),
          }
          if (yours) {
            s.yours = yours.id
            s.status = canon(yours) === canon(p) ? 'imported' : 'differs'
            if (s.status === 'differs') {
              s.by = yours.from!.sum === canon(p) ? 'you' : 'project'
              s.changed = '2026-11-03T10:00:00Z'
            }
          }
          return s
        }),
      }
    },
    async importContainer(p: ContainerProfile, repo: string, suggestion: string) {
      const sug = SUGGESTED.find(s => s.id === suggestion)
      if (repo !== REPO || !sug) throw new Error("that project doesn't suggest that container any more")
      await save({ ...p, from: { repo, id: suggestion, imported: new Date().toISOString(), sum: canon(sug) } })
    },
    async removeContainerConfig(id: string) {
      const i = state.containers.findIndex(c => c.id === id)
      const was = shipped.get(id)
      if (i < 0) return
      if (was) {
        const c: ContainerInfo = { ...structuredClone(was), built: state.containers[i]!.built }
        c.changes = changesOf(c)
        state.containers[i] = c
      } else {
        state.containers.splice(i, 1)
        state.wsl.distros = state.wsl.distros.filter(d => d !== `uncli-${id}`)
      }
      send()
    },
    async startContainerSignIn() {
      if (!state.containers.some(c => c.built)) throw new Error('build a container first: sign-in runs inside one')
      return 'https://claude.com/cai/oauth/authorize?code=true&client_id=mock&state=mock'
    },
    async finishContainerSignIn(code: string) {
      await new Promise(r => setTimeout(r, 500))
      if (!code.includes('#')) throw new Error('sign-in didn\'t finish: check the code and try again')
      state.signedIn = true
      send()
    },
    async cancelContainerSignIn() {},
    async signOutContainers() { state.signedIn = false; send() },
    async startContainerAccountSignIn() {
      if (!state.containers.some(c => c.built)) throw new Error('build a container first: sign-in runs inside one')
      return 'https://claude.com/cai/oauth/authorize?code=true&client_id=mock&scope=user%3Amcp_servers&state=mock'
    },
    async finishContainerAccountSignIn(code: string) {
      await new Promise(r => setTimeout(r, 500))
      if (!code.includes('#')) throw new Error("sign-in didn't finish: check the code and try again")
      state.accountSignedIn = true
      send()
    },
    async cancelContainerAccountSignIn() {},
    async signOutContainerAccount() { state.accountSignedIn = false; send() },
  }
}
