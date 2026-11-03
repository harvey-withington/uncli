// What a remote client can't do (decision 0021): things that happen on
// the server's own screen or machine. The UI shows them disabled, with
// the reason, rather than hiding them.
import type { Backend } from './api'
import { t } from './i18n.svelte'

// atServer: the backend is the window on the server itself (or the mock),
// so the server's screen, folders and editors are here.
export function atServer(backend: Pick<Backend, 'kind'>): boolean {
  return backend.kind !== 'http'
}

// onlyAtServer is the reason shown on what a remote client can't use.
export function onlyAtServer(): string {
  return t('remote.onlyAtServer')
}

// serverTitle is the tooltip of a control that works only at the server:
// its label there, and the label with the reason on a remote client.
export function serverTitle(backend: Pick<Backend, 'kind'>, label: string): string {
  return atServer(backend) ? label : t('remote.onlyAtServerFor', { what: label })
}
