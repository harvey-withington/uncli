// The names of the AI provider behind a session, for t()'s {agent}, {cli}
// and {account}. UNCLI's text never names a CLI itself (docs/UI-CONVENTIONS.md).
import type { Bootstrap, Provider } from './api'

// A type, not an interface, so it passes as t()'s params.
export type ProviderNames = {
  agent: string
  cli: string
  account: string
}

export function providerOf(boot: Bootstrap | null | undefined, adapter?: string): Provider | undefined {
  const ps = boot?.providers ?? []
  return ps.find(p => p.id === adapter) ?? ps[0]
}

// namesOf is what a session's text says for its provider.
export function namesOf(p: Provider | undefined): ProviderNames {
  return { agent: p?.agent ?? 'the assistant', cli: p?.name ?? 'the CLI', account: p?.account ?? 'your' }
}
