// Shiki highlighting, loaded lazily with a fixed set of languages and the
// JavaScript regex engine (no wasm). Both themes are emitted as CSS
// variables so light and dark switch without re-highlighting.
import type { HighlighterCore } from 'shiki/core'

let loading: Promise<HighlighterCore> | null = null

const LANGS: Record<string, () => Promise<unknown>> = {
  bash: () => import('shiki/langs/bash.mjs'),
  c: () => import('shiki/langs/c.mjs'),
  cpp: () => import('shiki/langs/cpp.mjs'),
  csharp: () => import('shiki/langs/csharp.mjs'),
  css: () => import('shiki/langs/css.mjs'),
  diff: () => import('shiki/langs/diff.mjs'),
  go: () => import('shiki/langs/go.mjs'),
  html: () => import('shiki/langs/html.mjs'),
  java: () => import('shiki/langs/java.mjs'),
  javascript: () => import('shiki/langs/javascript.mjs'),
  json: () => import('shiki/langs/json.mjs'),
  jsx: () => import('shiki/langs/jsx.mjs'),
  markdown: () => import('shiki/langs/markdown.mjs'),
  powershell: () => import('shiki/langs/powershell.mjs'),
  python: () => import('shiki/langs/python.mjs'),
  rust: () => import('shiki/langs/rust.mjs'),
  sql: () => import('shiki/langs/sql.mjs'),
  svelte: () => import('shiki/langs/svelte.mjs'),
  toml: () => import('shiki/langs/toml.mjs'),
  tsx: () => import('shiki/langs/tsx.mjs'),
  typescript: () => import('shiki/langs/typescript.mjs'),
  yaml: () => import('shiki/langs/yaml.mjs'),
}

const ALIASES: Record<string, string> = {
  sh: 'bash', shell: 'bash', zsh: 'bash', console: 'bash', js: 'javascript', ts: 'typescript',
  py: 'python', yml: 'yaml', md: 'markdown', ps1: 'powershell', pwsh: 'powershell', golang: 'go',
  'c++': 'cpp', cs: 'csharp', rs: 'rust',
}

export function normalizeLang(lang: string): string | null {
  const l = lang.toLowerCase()
  const name = ALIASES[l] ?? l
  return name in LANGS ? name : null
}

async function highlighter(): Promise<HighlighterCore> {
  loading ??= (async () => {
    const [{ createHighlighterCore }, { createJavaScriptRegexEngine }, light, dark] = await Promise.all([
      import('shiki/core'),
      import('shiki/engine/javascript'),
      import('shiki/themes/github-light.mjs'),
      import('shiki/themes/github-dark.mjs'),
    ])
    return createHighlighterCore({
      themes: [light.default, dark.default],
      langs: [],
      engine: createJavaScriptRegexEngine(),
    })
  })()
  return loading
}

// highlight returns themed HTML for code, or null if the language isn't
// supported (the caller shows plain code).
export async function highlight(code: string, lang: string): Promise<string | null> {
  const name = normalizeLang(lang)
  if (!name) return null
  const h = await highlighter()
  if (!h.getLoadedLanguages().includes(name)) {
    const mod = (await LANGS[name]?.()) as { default: Parameters<HighlighterCore['loadLanguage']>[0] }
    await h.loadLanguage(mod.default)
  }
  return h.codeToHtml(code, {
    lang: name,
    themes: { light: 'github-light', dark: 'github-dark' },
    defaultColor: false,
  })
}
