// The sidebar's groups, one per place sessions run (This PC and each paired
// server, decision 0031): whether each is collapsed, whether its archived
// sessions are hidden, and whether its Archived list is open. Remembered
// between runs, by the group's key (LOCAL or the server's).
import { store } from '@uncli/chat/lib/storage'

export interface GroupState {
  collapsed?: boolean // the whole group folded under its heading
  hideArchived?: boolean // no Archived list at all
  archivedOpen?: boolean // the Archived list expanded
}

export type GroupStates = Record<string, GroupState>
export type GroupFlag = keyof GroupState

const KEY = 'uncli-sidebar-groups'

export function loadGroups(): GroupStates {
  try {
    const v: unknown = JSON.parse(store.getItem(KEY) ?? '{}')
    if (!v || typeof v !== 'object' || Array.isArray(v)) return {}
    const out: GroupStates = {}
    for (const [k, g] of Object.entries(v as Record<string, unknown>)) {
      if (!g || typeof g !== 'object') continue
      const s = g as Record<string, unknown>
      out[k] = { collapsed: s.collapsed === true, hideArchived: s.hideArchived === true, archivedOpen: s.archivedOpen === true }
    }
    return out
  } catch {
    return {}
  }
}

export function saveGroups(groups: GroupStates) {
  try {
    store.setItem(KEY, JSON.stringify(groups))
  } catch {
    // not remembered; still applied for this run
  }
}

// toggled is groups with one group's flag flipped (or set to on).
export function toggled(groups: GroupStates, key: string, flag: GroupFlag, on?: boolean): GroupStates {
  const g = groups[key] ?? {}
  return { ...groups, [key]: { ...g, [flag]: on ?? !g[flag] } }
}
