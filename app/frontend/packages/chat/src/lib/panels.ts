// The sidebar's width: a per-viewer convenience kept in localStorage
// (defaults apply if storage is unavailable).
import { store } from './storage'

export const SIDEBAR_MIN = 200
export const SIDEBAR_MAX = 440
export const SIDEBAR_DEFAULT = 272

const KEY = 'uncli-sidebar'

export function loadSidebarWidth(): number {
  try {
    const v = Number(JSON.parse(store.getItem(KEY) ?? 'null')?.width)
    if (Number.isFinite(v) && v > 0) return Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, Math.round(v)))
  } catch {
    // fall through to the default
  }
  return SIDEBAR_DEFAULT
}

export function saveSidebarWidth(width: number) {
  try {
    store.setItem(KEY, JSON.stringify({ width }))
  } catch {
    // not persisted; still applied for this run
  }
}

// Whether the session header is collapsed to two lines (compact mode): the
// title with the folder after it, and the toolbar.
const HEADER_KEY = 'uncli-header'

export function loadHeaderCompact(): boolean {
  try {
    return JSON.parse(store.getItem(HEADER_KEY) ?? 'null')?.compact === true
  } catch {
    return false
  }
}

export function saveHeaderCompact(compact: boolean) {
  try {
    store.setItem(HEADER_KEY, JSON.stringify({ compact }))
  } catch {
    // not persisted; still applied for this run
  }
}

// Whether the question header is collapsed to one line (compact mode).
const QUESTION_KEY = 'uncli-question'

export function loadQuestionCompact(): boolean {
  try {
    return JSON.parse(store.getItem(QUESTION_KEY) ?? 'null')?.compact === true
  } catch {
    return false
  }
}

export function saveQuestionCompact(compact: boolean) {
  try {
    store.setItem(QUESTION_KEY, JSON.stringify({ compact }))
  } catch {
    // not persisted; still applied for this run
  }
}

// Whether the compact header's Options are open.
const OPTIONS_KEY = 'uncli-options'

export function loadOptionsOpen(): boolean {
  try {
    return JSON.parse(store.getItem(OPTIONS_KEY) ?? 'null')?.open === true
  } catch {
    return false
  }
}

export function saveOptionsOpen(open: boolean) {
  try {
    store.setItem(OPTIONS_KEY, JSON.stringify({ open }))
  } catch {
    // not persisted; still applied for this run
  }
}

// The side panel's tabs, "On this page", "Artifacts" and "File" (a file
// from the session folder, watched live): which one is showing. Whether the panel is open, and its width (one for both tabs),
// are the outline layout's (lib/outline.ts).
export type PanelTab = 'outline' | 'artifacts' | 'file'

const PANEL_KEY = 'uncli-panel'

export interface PanelLayout {
  tab: PanelTab
}

export function loadPanelLayout(): PanelLayout {
  try {
    const v = JSON.parse(store.getItem(PANEL_KEY) ?? 'null') as Partial<PanelLayout> | null
    if (v) return { tab: v.tab === 'artifacts' || v.tab === 'file' ? v.tab : 'outline' }
  } catch {
    // fall through to the defaults
  }
  return { tab: 'outline' }
}

export function savePanelLayout(l: PanelLayout) {
  try {
    store.setItem(PANEL_KEY, JSON.stringify(l))
  } catch {
    // not persisted; still applied for this run
  }
}
