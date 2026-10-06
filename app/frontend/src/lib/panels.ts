// The sidebar's width: a per-viewer convenience kept in localStorage
// (defaults apply if storage is unavailable).
export const SIDEBAR_MIN = 200
export const SIDEBAR_MAX = 440
export const SIDEBAR_DEFAULT = 272

const KEY = 'uncli-sidebar'

export function loadSidebarWidth(): number {
  try {
    const v = Number(JSON.parse(localStorage.getItem(KEY) ?? 'null')?.width)
    if (Number.isFinite(v) && v > 0) return Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, Math.round(v)))
  } catch {
    // fall through to the default
  }
  return SIDEBAR_DEFAULT
}

export function saveSidebarWidth(width: number) {
  try {
    localStorage.setItem(KEY, JSON.stringify({ width }))
  } catch {
    // not persisted; still applied for this run
  }
}

// Whether the question header is collapsed to one line (compact mode).
const QUESTION_KEY = 'uncli-question'

export function loadQuestionCompact(): boolean {
  try {
    return JSON.parse(localStorage.getItem(QUESTION_KEY) ?? 'null')?.compact === true
  } catch {
    return false
  }
}

export function saveQuestionCompact(compact: boolean) {
  try {
    localStorage.setItem(QUESTION_KEY, JSON.stringify({ compact }))
  } catch {
    // not persisted; still applied for this run
  }
}

// The side panel's tabs, "On this page" and "Artifacts": which one is
// showing. Whether the panel is open, and its width (one for both tabs),
// are the outline layout's (lib/outline.ts).
export type PanelTab = 'outline' | 'artifacts'

const PANEL_KEY = 'uncli-panel'

export interface PanelLayout {
  tab: PanelTab
}

export function loadPanelLayout(): PanelLayout {
  try {
    const v = JSON.parse(localStorage.getItem(PANEL_KEY) ?? 'null') as Partial<PanelLayout> | null
    if (v) return { tab: v.tab === 'artifacts' ? 'artifacts' : 'outline' }
  } catch {
    // fall through to the defaults
  }
  return { tab: 'outline' }
}

export function savePanelLayout(l: PanelLayout) {
  try {
    localStorage.setItem(PANEL_KEY, JSON.stringify(l))
  } catch {
    // not persisted; still applied for this run
  }
}
