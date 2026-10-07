import { t } from './i18n.svelte'

export function tokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 10_000) return `${Math.round(n / 1000)}k`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

export function cost(usd: number): string {
  if (usd <= 0) return '$0'
  if (usd < 0.01) return `$${usd.toFixed(4)}`
  return `$${usd.toFixed(2)}`
}

export function duration(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)} s`
  if (s < 3600) return `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`
  return `${Math.floor(s / 3600)} h ${Math.round((s % 3600) / 60)} min`
}

export function bytes(n: number): string {
  if (n >= 1 << 30) return `${(n / (1 << 30)).toFixed(1)} GB`
  if (n >= 1 << 20) return `${Math.round(n / (1 << 20))} MB`
  return `${Math.round(n / 1024)} KB`
}

export function relativeTime(ms: number, now = Date.now()): string {
  const s = Math.round((now - ms) / 1000)
  if (s < 60) return t('time.now')
  if (s < 3600) return t('time.minutes', { n: Math.floor(s / 60) })
  if (s < 86400) return t('time.hours', { n: Math.floor(s / 3600) })
  return t('time.days', { n: Math.floor(s / 86400) })
}

export function resetsIn(unixSeconds: number, now = Date.now()): string {
  const mins = Math.max(0, Math.round((unixSeconds * 1000 - now) / 60000))
  if (mins < 60) return t('time.inMinutes', { n: mins })
  const h = Math.floor(mins / 60)
  if (h < 48) return t('time.inHours', { n: h })
  return t('time.inDays', { n: Math.round(h / 24) })
}
