// A theme from outside UNCLI: values for the public --uncli-* tokens
// (docs/decisions/0010). The desktop app reads them from theme.yaml in its
// config folder; an embedding host will set the properties itself. A theme
// supplies values only: unknown names are skipped and a value that could add
// rules or fetch anything is refused.

// The public tokens, without the --uncli- prefix. app.css reads each one
// and docs/UI-CONVENTIONS.md lists them; a test keeps the three in step.
export const THEME_TOKENS = [
  'bg', 'surface', 'surface-2', 'surface-3',
  'text', 'text-muted', 'text-faint',
  'border', 'border-strong',
  'accent', 'accent-hover', 'accent-text',
  'danger', 'warning', 'success',
  'shadow-sm', 'shadow-md', 'shadow-lg', 'scrim',
  'chart-bar',
  'font', 'mono',
  'radius-sm', 'radius', 'radius-lg',
] as const

export type ThemeValues = Record<string, string>
export interface ThemeFile {
  light?: ThemeValues
  dark?: ThemeValues
}

const known = new Set<string>(THEME_TOKENS)
const unsafe = /[;{}<>\\]|url\(|@|\/\*/i

export function themeCSS(file: ThemeFile): { css: string; warnings: string[] } {
  const warnings: string[] = []
  const rules: string[] = []
  for (const scheme of ['light', 'dark'] as const) {
    const values = file[scheme]
    if (!values || typeof values !== 'object') continue
    const decls: string[] = []
    for (const [name, raw] of Object.entries(values)) {
      if (!known.has(name)) {
        warnings.push(`${scheme}.${name}: not a theme token`)
        continue
      }
      const v = String(raw ?? '').trim()
      if (v === '' || v.length > 300 || unsafe.test(v) || (v.split('"').length - 1) % 2 || (v.split("'").length - 1) % 2) {
        warnings.push(`${scheme}.${name}: value refused`)
        continue
      }
      decls.push(`  --uncli-${name}: ${v};`)
    }
    if (decls.length) rules.push(`:root[data-uncli-scheme='${scheme}'] {\n${decls.join('\n')}\n}`)
  }
  return { css: rules.join('\n'), warnings }
}

const ID = 'uncli-theme-file'

// applyThemeFile replaces the theme file's styles (none removes them).
export function applyThemeFile(file: ThemeFile | null): string[] {
  document.getElementById(ID)?.remove()
  if (!file) return []
  const { css, warnings } = themeCSS(file)
  if (css) {
    const el = document.createElement('style')
    el.id = ID
    el.textContent = css
    document.head.appendChild(el)
  }
  for (const w of warnings) console.warn('theme.yaml', w)
  return warnings
}
