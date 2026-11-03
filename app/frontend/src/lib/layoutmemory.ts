// Where a chat remembers its layout: the side panel (open, width, tab) and
// the compact header and question (decision 0026). The app remembers it
// under its own keys, as it always has; an embedded chat only under the
// storage key its host gives it, and without one it remembers nothing,
// so UNCLI never writes to a host's storage unasked.
import { OUTLINE_DEFAULT, loadLayout, saveLayout, type OutlineLayout } from './outline'
import {
  loadHeaderCompact, loadPanelLayout, loadQuestionCompact, saveHeaderCompact, savePanelLayout, saveQuestionCompact,
  type PanelLayout,
} from './panels'

export interface ChatLayout {
  outline: OutlineLayout
  panel: PanelLayout
  questionCompact: boolean
  headerCompact: boolean
}

export interface LayoutMemory {
  load(): ChatLayout
  save(l: ChatLayout): void
}

export function defaultLayout(): ChatLayout {
  return { outline: { open: true, width: OUTLINE_DEFAULT }, panel: { tab: 'outline' }, questionCompact: false, headerCompact: false }
}

// The app's memory: its existing keys.
export const appMemory: LayoutMemory = {
  load: () => ({ outline: loadLayout(), panel: loadPanelLayout(), questionCompact: loadQuestionCompact(), headerCompact: loadHeaderCompact() }),
  save: l => {
    saveLayout(l.outline)
    savePanelLayout(l.panel)
    saveQuestionCompact(l.questionCompact)
    saveHeaderCompact(l.headerCompact)
  },
}

// keyedMemory is an embedded chat's: under "uncli-layout:<key>", or
// nowhere when there's no key.
export function keyedMemory(key: string | null | undefined): LayoutMemory {
  if (!key) return { load: defaultLayout, save: () => {} }
  const at = `uncli-layout:${key}`
  return {
    load: () => {
      try {
        const v = JSON.parse(localStorage.getItem(at) ?? 'null') as Partial<ChatLayout> | null
        const d = defaultLayout()
        return v ? { ...d, ...v, outline: { ...d.outline, ...v.outline }, panel: { ...d.panel, ...v.panel } } : d
      } catch {
        return defaultLayout()
      }
    },
    save: l => {
      try {
        localStorage.setItem(at, JSON.stringify(l))
      } catch {
        // not remembered; still applied for this run
      }
    },
  }
}
