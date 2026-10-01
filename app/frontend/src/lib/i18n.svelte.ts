// Every user-visible string goes through t(); keys live in locales/en.json.
import en from '../locales/en.json'

type Dict = Record<string, string>

const base: Dict = en
const locales: Record<string, Dict> = { en: base }

export const i18n = $state({ locale: 'en' })

export function t(key: string, params?: Record<string, string | number>): string {
  const dict = locales[i18n.locale] ?? base
  let s = dict[key] ?? base[key] ?? key
  if (params) {
    for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, String(v))
  }
  return s
}
