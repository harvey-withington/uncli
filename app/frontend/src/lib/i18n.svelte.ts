// Every user-visible string goes through t(); keys live in locales/en.json.
import en from '../locales/en.json'

type Dict = Record<string, string>

const base: Dict = en
const locales: Record<string, Dict> = { en: base }

export const i18n = $state({ locale: 'en' })

// Names UNCLI's text never hard-codes: the AI provider's (lib/providers.ts).
// A string says {agent}, {cli} or {account}; t() fills them from here unless
// the caller passes a session's own.
const names: Record<string, string> = { agent: 'the assistant', cli: 'the CLI', account: 'your' }

export function setNames(n: Partial<Record<'agent' | 'cli' | 'account', string>>) {
  for (const [k, v] of Object.entries(n)) if (v) names[k] = v
}

export function t(key: string, params?: Record<string, string | number>): string {
  const dict = locales[i18n.locale] ?? base
  let s = dict[key] ?? base[key] ?? key
  for (const [k, v] of Object.entries({ ...names, ...params })) s = s.replaceAll(`{${k}}`, String(v))
  return s
}
