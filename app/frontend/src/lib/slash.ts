// The slash-command menu's rules: when typing in the message box is typing
// a command, which of the CLI's commands match, and what choosing one puts
// in the box. The commands come from the backend (Backend.commands), which
// leaves out ones that only work in the CLI's terminal and ones UNCLI never
// sends.
import type { CommandInfo } from './api'

export const MENU_SIZE = 8

// The command being typed: the text after the slash up to the caret, while
// the whole message is still only "/name" (nothing after it yet); null
// otherwise.
export function slashQuery(text: string, caret: number): string | null {
  if (caret < 1 || !/^\/[\w:.-]*$/.test(text)) return null
  return text.slice(1, caret)
}

// The commands for a query: names that start with it first, then names
// and descriptions that contain it, alphabetically within each.
export function matchCommands(list: CommandInfo[], query: string, limit = MENU_SIZE): CommandInfo[] {
  const q = query.toLowerCase()
  const byName = (a: CommandInfo, b: CommandInfo) => a.name.localeCompare(b.name)
  const starts = list.filter(c => c.name.toLowerCase().startsWith(q)).sort(byName)
  const contains = q
    ? list.filter(c => !starts.includes(c) && (c.name.toLowerCase().includes(q) || (c.description ?? '').toLowerCase().includes(q))).sort(byName)
    : []
  return [...starts, ...contains].slice(0, limit)
}

// The message once a command is chosen: "/name " and whatever followed the
// command being typed.
export function complete(text: string, name: string): string {
  const rest = text.replace(/^\/[\w:.-]*/, '').replace(/^\s+/, '')
  return `/${name} ${rest}`
}
