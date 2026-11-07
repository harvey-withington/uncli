// Explain's options (decision 0029), remembered on this device: Explain
// runs with the last ones used, and its options dialog starts from them.
import type { ExplainOptions } from './api'

const KEY = 'uncli-explain'

export const EXPLAIN_DEFAULTS: ExplainOptions = { model: '', context: 'page', pages: 3, detail: 'brief' }

export function loadExplainOptions(): ExplainOptions {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null') as Partial<ExplainOptions> | null
    if (!v || typeof v !== 'object') return { ...EXPLAIN_DEFAULTS }
    return {
      model: typeof v.model === 'string' ? v.model : '',
      context: v.context === 'pages' || v.context === 'session' ? v.context : 'page',
      pages: Number.isInteger(v.pages) && v.pages! >= 1 && v.pages! <= 50 ? v.pages! : EXPLAIN_DEFAULTS.pages,
      detail: v.detail === 'thorough' ? 'thorough' : 'brief',
    }
  } catch {
    return { ...EXPLAIN_DEFAULTS }
  }
}

export function saveExplainOptions(o: ExplainOptions): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(o))
  } catch {
    // private browsing or full: it just isn't remembered
  }
}

// "about 4k tokens", for the size an explanation would send.
export function tokensLabel(n: number): string {
  return n < 1000 ? `${Math.max(100, Math.round(n / 100) * 100)}` : `${Math.round(n / 1000)}k`
}
