// Where a chat remembers the viewer's layout and preferences (decision
// 0026): the side panel (open, width, tab), the compact header and question,
// the compact header's Options, and the scope "This is safe" last used. Not
// per-chat state (drafts, scroll spots, a session's File tab), which stays
// with each chat. The app remembers it under its own keys, as it always
// has; an embedded chat only under the storage key its host gives it, and
// without one only for as long as the element is mounted, so UNCLI never
// writes to a host's storage unasked. Every chat on the page with the same
// key shares one memory: a change in one is made in the others at once,
// and a chat mounted later starts from it.
import type { Scope } from './api'
import { loadScope, saveScope } from './approvals'
import { OUTLINE_DEFAULT, loadLayout, saveLayout, type OutlineLayout } from './outline'
import {
  loadHeaderCompact, loadOptionsOpen, loadPanelLayout, loadQuestionCompact, saveHeaderCompact, saveOptionsOpen, savePanelLayout, saveQuestionCompact,
  type PanelLayout,
} from './panels'

export interface ChatLayout {
  outline: OutlineLayout
  panel: PanelLayout
  questionCompact: boolean
  headerCompact: boolean
  optionsOpen: boolean // the compact header's Options
  scope: Scope // where "This is safe" and "This should prompt" last applied
}

export interface LayoutMemory {
  load(): ChatLayout
  save(l: ChatLayout): void
  // follow calls fn when another chat sharing this memory saves; it
  // returns a function that stops it.
  follow?(fn: (l: ChatLayout) => void): () => void
}

// An embedded chat starts with its side panel closed: it often sits in a
// narrow space, and in the compact layout the panel covers the page.
export function defaultLayout(): ChatLayout {
  return {
    outline: { open: false, width: OUTLINE_DEFAULT },
    panel: { tab: 'outline' },
    questionCompact: false,
    headerCompact: false,
    optionsOpen: false,
    scope: 'project',
  }
}

// The app's memory: its existing keys.
export const appMemory: LayoutMemory = {
  load: () => ({
    outline: loadLayout(),
    panel: loadPanelLayout(),
    questionCompact: loadQuestionCompact(),
    headerCompact: loadHeaderCompact(),
    optionsOpen: loadOptionsOpen(),
    scope: loadScope(),
  }),
  save: l => {
    saveLayout(l.outline)
    savePanelLayout(l.panel)
    saveQuestionCompact(l.questionCompact)
    saveHeaderCompact(l.headerCompact)
    saveOptionsOpen(l.optionsOpen)
    saveScope(l.scope)
  },
}

// The chats following each storage key on the page, by the memory they
// came from (so a chat isn't told of its own save).
const followers = new Map<string, Set<{ from: LayoutMemory; fn: (l: ChatLayout) => void }>>()

function parse(raw: string | null): ChatLayout {
  try {
    const v = JSON.parse(raw ?? 'null') as Partial<ChatLayout> | null
    const d = defaultLayout()
    return v ? { ...d, ...v, outline: { ...d.outline, ...v.outline }, panel: { ...d.panel, ...v.panel } } : d
  } catch {
    return defaultLayout()
  }
}

// keyedMemory is an embedded chat's: under "uncli-layout:<key>" in
// localStorage, shared with every chat on the page given the same key; or,
// with no key, the chat's own for as long as it's mounted.
export function keyedMemory(key: string | null | undefined): LayoutMemory {
  if (!key) return { load: defaultLayout, save: () => {} }
  const at = `uncli-layout:${key}`
  const self: LayoutMemory = {
    load: () => {
      try {
        return parse(localStorage.getItem(at))
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
      for (const f of followers.get(at) ?? []) if (f.from !== self) f.fn(l)
    },
    follow: fn => {
      const entry = { from: self, fn }
      const set = followers.get(at) ?? new Set()
      set.add(entry)
      followers.set(at, set)
      return () => {
        set.delete(entry)
        if (set.size === 0) followers.delete(at)
      }
    },
  }
  return self
}
