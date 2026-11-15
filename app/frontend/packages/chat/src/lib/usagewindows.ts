// The subscription usage windows the CLI reports by name (five_hour,
// seven_day, seven_day_opus…), as the usage popover shows them. A plan can
// report windows UNCLI hasn't named yet (seven_day_overage_included came
// with one): those are named from their parts rather than shown as a key.
import { t } from './i18n.svelte'

export function windowLabel(name: string): string {
  const key = `usage.window.${name}`
  const named = t(key)
  if (named !== key) return named
  // A known window with a qualifier: "This week (overage included)".
  for (const base of ['seven_day', 'five_hour']) {
    if (name.startsWith(`${base}_`)) {
      return t('usage.window.qualified', { window: t(`usage.window.${base}`), what: words(name.slice(base.length + 1)) })
    }
  }
  const w = words(name)
  return w.charAt(0).toUpperCase() + w.slice(1)
}

const words = (s: string) => s.split('_').filter(Boolean).join(' ')
