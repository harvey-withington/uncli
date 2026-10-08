// Containers in the browser mock. ?containers=nowsl starts without WSL,
// ?containers=hung with WSL not responding, ?containers=built with the sandbox built and signed in; otherwise WSL is
// on and nothing is built. Building walks through the steps on timers.
import type { ContainerStep, ContainersInfo } from './types'

const STEPS: ContainerStep[] = ['download', 'remove', 'import', 'packages', 'cli', 'lockdown', 'check']

export function mockContainers(emit: (c: ContainersInfo) => void) {
  const mode = new URLSearchParams(location.search).get('containers')
  const state: ContainersInfo = {
    wsl: { unresponsive: mode === 'hung', installed: mode !== 'nowsl' && mode !== 'hung', version: mode === 'nowsl' ? undefined : '3.0.1.0', kernel: mode === 'nowsl' ? undefined : '6.18.40.1-1', distros: [] },
    signedIn: mode === 'built',
    containers: [
      { id: 'sandbox', label: 'Sandbox', description: 'Linux with the usual tools, your memories and your MCP servers. A session there sees only its own folder.', base: 'alpine-3.24', baseLabel: 'Alpine Linux 3.24', brain: 'shared', mcp: 'shared', connectors: 'shared', packages: ['coreutils', 'findutils', 'grep', 'sed', 'less', 'curl', 'jq', 'ripgrep', 'make', 'nodejs', 'npm', 'python3', 'py3-pip'], built: mode === 'built', changes: mode === 'built' ? ['packages'] : [] },
      { id: 'node-dev', label: 'Node', base: 'alpine-3.24', baseLabel: 'Alpine Linux 3.24', packages: ['nodejs', 'npm'], built: false },
    ],
  }
  if (mode === 'built') state.wsl.distros = ['uncli-sandbox']
  const snap = () => structuredClone(state)
  const send = () => emit(snap())
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
