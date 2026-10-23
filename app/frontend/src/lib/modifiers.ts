// How the toolbar shows modifiers: one on its own is a toggle; the members
// of a group (mutually exclusive, modifiers.yaml) are one segmented choice
// with "Normal" for none of them. Choices are ordered by what they cost:
// tone success (cheaper) before Normal, the rest after, in config order.
import type { Modifier } from './api'

export type ModifierItem = { kind: 'toggle'; modifier: Modifier } | { kind: 'group'; id: string; members: Modifier[] }

// arrange keeps the config's order, a group at its first member.
export function arrange(mods: Modifier[]): ModifierItem[] {
  const out: ModifierItem[] = []
  const groups = new Map<string, Modifier[]>()
  for (const m of mods) {
    if (!m.group) {
      out.push({ kind: 'toggle', modifier: m })
      continue
    }
    let members = groups.get(m.group)
    if (!members) {
      groups.set(m.group, (members = []))
      out.push({ kind: 'group', id: m.group, members })
    }
    members.push(m)
  }
  // A group of one is just a toggle.
  return out.map(x => (x.kind === 'group' && x.members.length === 1 && x.members[0] ? { kind: 'toggle', modifier: x.members[0] } : x))
}

export interface GroupChoice {
  id: string // '' for Normal
  label: string
  icon?: string
  tone?: '' | 'success' | 'warning' | 'accent'
}

// choicesOf orders a group's choices, Normal among them.
export function choicesOf(members: Modifier[], normal: string): GroupChoice[] {
  const of = (m: Modifier): GroupChoice => ({ id: m.id, label: m.label, icon: m.icon, tone: m.tone ?? '' })
  return [
    ...members.filter(m => m.tone === 'success').map(of),
    { id: '', label: normal, tone: '' },
    ...members.filter(m => m.tone !== 'success').map(of),
  ]
}

// chosenIn is the group member a session has on, or '' (Normal).
export function chosenIn(members: Modifier[], active: string[]): string {
  return members.find(m => active.includes(m.id))?.id ?? ''
}
