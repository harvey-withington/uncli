// What in Settings needs the user, per tab: a dot on the Settings button and
// on the tab says so (docs/UI-CONVENTIONS.md). Only what UNCLI can't fix by
// itself counts: a container to rebuild, a failed build, a sign-in that
// containers need, WSL not answering.
import type { ContainersInfo } from './api'
import type { SettingsTab } from './settings'

export type Attention = Partial<Record<SettingsTab, string[]>> // tab -> i18n keys of what needs doing

export function settingsAttention(c: ContainersInfo | null): Attention {
  const out: Attention = {}
  const add = (tab: SettingsTab, key: string) => {
    const list = (out[tab] ??= [])
    if (!list.includes(key)) list.push(key)
  }
  if (!c || c.error) {
    if (c?.error) add('containers', 'attention.containersFile')
    return out
  }
  if (c.wsl.unresponsive) add('containers', 'attention.wslStuck')
  const built = c.containers.filter(x => x.built)
  for (const x of c.containers) {
    if (x.step) continue // being built: nothing to do yet
    if (x.error) add('containers', 'attention.buildFailed')
    if (x.built && (x.changes?.length ?? 0) > 0) add('containers', 'attention.rebuild')
    if (x.built && x.connectors === 'shared' && !c.accountSignedIn) add('providers', 'attention.accountSignIn')
  }
  if (built.length > 0 && !c.signedIn && !c.accountSignedIn) add('providers', 'attention.containerSignIn')
  return out
}

// needsAttention: anything at all.
export function needsAttention(a: Attention): boolean {
  return Object.values(a).some(l => (l?.length ?? 0) > 0)
}
