// Theme: follow the system by default; the user can pin light or dark.
// Whatever the choice, the scheme it resolves to is set on <html> as
// data-uncli-scheme, which app.css and theme files key on.
export type Theme = 'system' | 'light' | 'dark'
export type Scheme = 'light' | 'dark'

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

const dark = typeof window !== 'undefined' ? window.matchMedia?.('(prefers-color-scheme: dark)') : undefined

export function scheme(): Scheme {
  if (theme.value !== 'system') return theme.value
  return dark?.matches ? 'dark' : 'light'
}

export function applyTheme() {
  document.documentElement.setAttribute('data-uncli-scheme', scheme())
}

// The system's scheme can change while UNCLI runs.
dark?.addEventListener?.('change', () => {
  if (theme.value === 'system') applyTheme()
})

export function cycleTheme() {
  setTheme(theme.value === 'system' ? 'light' : theme.value === 'light' ? 'dark' : 'system')
}

export function setTheme(v: Theme) {
  theme.value = v
  try {
    localStorage.setItem(KEY, theme.value)
  } catch {
    // not persisted; still applied for this run
  }
  applyTheme()
}
