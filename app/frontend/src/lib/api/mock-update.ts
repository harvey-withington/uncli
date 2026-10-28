// UNCLI's own updates for the browser mock (decision 0025): a release build
// that's up to date. ?update=available has 0.4.0-alpha out; ?update=dev is a
// development build. Installing runs through downloading and installing,
// then stays there, as UNCLI would quit.
import type { UpdateStatus } from './types'

export function mockUpdate(emit: (s: UpdateStatus) => void) {
  const scenario = new URLSearchParams(globalThis.location?.search ?? '').get('update')
  const latest = (version: string) => ({ version, url: `https://github.com/harvey-withington/uncli/releases/tag/v${version}`, published: '2026-11-02T09:00:00Z', setup: `uncli-${version}-windows-x64-setup.exe` })
  let status: UpdateStatus = scenario === 'dev'
    ? { current: 'dev', dev: true, state: 'idle', available: false }
    : scenario === 'available'
      ? { current: '0.3.0-alpha', dev: false, state: 'idle', available: true, latest: latest('0.4.0-alpha'), checkedAt: Date.now() - 20 * 60_000 }
      : { current: '0.3.0-alpha', dev: false, state: 'idle', available: false }
  const set = (next: Partial<UpdateStatus>) => {
    status = { ...status, ...next }
    emit(structuredClone(status))
    return structuredClone(status)
  }
  return {
    async updateStatus() { return structuredClone(status) },
    async checkForUpdates() {
      if (status.dev) throw new Error("this is a development build: it's updated with npm run install-build, not from releases")
      return set({ state: 'idle', checkedAt: Date.now(), latest: status.latest ?? latest('0.3.0-alpha') })
    },
    async installUpdate() {
      if (!status.available) throw new Error("there's no newer release to install")
      set({ state: 'downloading', percent: 0 })
      for (const p of [35, 70, 100]) {
        await new Promise(r => setTimeout(r, 120))
        set({ percent: p })
      }
      return set({ state: 'installing' })
    },
  }
}
