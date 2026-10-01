// Theme: follow the system by default; the user can pin light or dark.
export type Theme = 'system' | 'light' | 'dark'

const KEY = 'uncli-theme'

function load(): Theme {
  const forced = new URLSearchParams(location.search).get('theme')
  if (forced === 'light' || forced === 'dark') return forced
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch {
    // storage unavailable: follow the system
  }
  return 'system'
}

export const theme = $state<{ value: Theme }>({ value: load() })

export function applyTheme() {
  const root = document.documentElement
  if (theme.value === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', theme.value)
}

export function cycleTheme() {
  theme.value = theme.value === 'system' ? 'light' : theme.value === 'light' ? 'dark' : 'system'
  try {
    localStorage.setItem(KEY, theme.value)
  } catch {
    // not persisted; still applied for this run
  }
  applyTheme()
}
