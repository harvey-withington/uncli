// Every action the app offers, in one registry: the command palette lists
// them, the keyboard map shows their shortcuts, and the window's key handler
// runs them. A command is only in the list while it can run (no page, no
// bookmark commands), so none of the three ever offers a dead action.
import type { SessionMode } from './api'
import { back, forward, nextBookmark, prevBookmark } from './nav'
import { t } from './i18n.svelte'
import { cycleTheme } from './theme.svelte'
import { showToast } from './toasts.svelte'
import type { AppStore } from '../stores/app.svelte'

export type Group = 'session' | 'page' | 'panel' | 'prompt' | 'model' | 'app'

export const GROUPS: Group[] = ['session', 'page', 'panel', 'prompt', 'model', 'app']

// A shortcut. ctrl means Ctrl, or ⌘ on a Mac. Plain keys (no ctrl) never
// fire while the user is typing; ctrl ones and function keys do.
export interface Key {
  key: string // KeyboardEvent.key, case-insensitive for letters
  ctrl?: boolean
  shift?: boolean
}

export interface Command {
  id: string
  group: Group
  label: string
  keys?: Key[] // the first is the one shown
  checked?: boolean // a state it sets that is in force now (the prompt level in use)
  run: () => unknown
}

// Every shortcut, in one table: the commands take their keys from it, and
// the keyboard map shows it whole (whatever can run at the moment). label
// is the i18n key the map uses.
export const SHORTCUTS: { id: string; group: Group; label: string; keys: Key[] }[] = [
  { id: 'palette', group: 'app', label: 'cmd.palette', keys: [{ key: 'p', ctrl: true, shift: true }, { key: 'F1' }] },
  { id: 'keys', group: 'app', label: 'cmd.keys', keys: [{ key: '?' }] },
  { id: 'new', group: 'session', label: 'cmd.new', keys: [{ key: 'n', ctrl: true }] },
  { id: 'search', group: 'session', label: 'cmd.search', keys: [{ key: 'k', ctrl: true }] },
  { id: 'back', group: 'page', label: 'cmd.back', keys: [{ key: 'ArrowLeft' }] },
  { id: 'forward', group: 'page', label: 'cmd.forward', keys: [{ key: 'ArrowRight' }] },
  { id: 'bookmark', group: 'page', label: 'keys.bookmark', keys: [{ key: 'b' }] },
  { id: 'pin', group: 'page', label: 'keys.pin', keys: [{ key: 'p' }] },
  { id: 'outline', group: 'panel', label: 'cmd.outline', keys: [{ key: 'o' }] },
  { id: 'artifacts', group: 'panel', label: 'cmd.artifacts', keys: [{ key: 'a' }] },
]

const keysOf = (id: string) => SHORTCUTS.find(x => x.id === id)?.keys

const run = (p: unknown) => {
  if (p instanceof Promise) p.catch(e => showToast(String(e), 'error'))
}

// commands lists what can run now, in a stable order (group, then as written).
export function commands(app: AppStore): Command[] {
  const out: Command[] = []
  const add = (c: Command) => out.push(c)
  const ready = app.cliReady
  const session = app.current
  const page = app.currentPage
  const pages = app.currentPages
  const i = app.currentIndex

  // App and sessions.
  add({ id: 'palette', group: 'app', label: t('cmd.palette'), keys: keysOf('palette'), run: () => (app.paletteOpen = !app.paletteOpen) })
  add({ id: 'keys', group: 'app', label: t('cmd.keys'), keys: keysOf('keys'), run: () => (app.keysOpen = true) })
  add({ id: 'settings', group: 'app', label: t('cmd.settings'), run: () => app.openSettings() })
  for (const s of ['notify', 'editor', 'safe', 'decider'] as const) {
    add({ id: `settings.${s}`, group: 'app', label: t(`cmd.settings.${s}`), run: () => app.openSettings(s) })
  }
  add({ id: 'usage', group: 'app', label: t('cmd.usage'), run: () => (app.usageOpen = true) })
  add({ id: 'theme', group: 'app', label: t('cmd.theme'), run: () => cycleTheme() })
  add({ id: 'theme.file', group: 'app', label: t('cmd.themeFile'), run: () => run(app.loadThemeFile(true)) })
  if (ready) {
    add({ id: 'new', group: 'session', label: t('cmd.new'), keys: keysOf('new'), run: () => app.openNewSession() })
    add({ id: 'search', group: 'session', label: t('cmd.search'), keys: keysOf('search'), run: () => app.searchFocus++ })
    for (const s of app.activeSessions) {
      if (s.id !== app.currentId) add({ id: `go.${s.id}`, group: 'session', label: t('cmd.go', { title: s.title || t('session.untitled') }), run: () => run(app.select(s.id)) })
    }
    if (app.boot?.capabilities.import) add({ id: 'import', group: 'session', label: t('cmd.import'), run: () => (app.importOpen = true) })
    if (app.archivedSessions.length) add({ id: 'archived', group: 'session', label: t('cmd.archivedList'), run: () => (app.showArchived = !app.showArchived) })
  }
  if (!session) return out

  add({ id: session.archived ? 'restore' : 'archive', group: 'session', label: t(session.archived ? 'cmd.restore' : 'cmd.archive'), run: () => run(app.archive(session.id, !session.archived)) })
  if (session.busy) add({ id: 'stop', group: 'session', label: t('cmd.stop'), run: () => run(app.backend.interrupt(session.id)) })

  // Pages.
  if (pages.length) {
    if (i > 0) add({ id: 'back', group: 'page', label: t('cmd.back'), keys: keysOf('back'), run: () => app.goTo(back(i)) })
    if (i < pages.length - 1) add({ id: 'forward', group: 'page', label: t('cmd.forward'), keys: keysOf('forward'), run: () => app.goTo(forward(i, pages.length)) })
    if (i > 0) add({ id: 'first', group: 'page', label: t('cmd.first'), run: () => app.goTo(0) })
    if (i < pages.length - 1) add({ id: 'last', group: 'page', label: t('cmd.last'), run: () => app.goTo(pages.length - 1) })
    if (i > 0) add({ id: 'prevBookmark', group: 'page', label: t('cmd.prevBookmark'), run: () => app.goTo(prevBookmark(pages, i)) })
    if (i < pages.length - 1) add({ id: 'nextBookmark', group: 'page', label: t('cmd.nextBookmark'), run: () => app.goTo(nextBookmark(pages, i)) })
  }
  if (page) {
    add({ id: 'bookmark', group: 'page', label: t(page.bookmarked ? 'cmd.unbookmark' : 'cmd.bookmark'), keys: keysOf('bookmark'), run: () => run(app.toggleBookmark()) })
    add({ id: 'pin', group: 'page', label: t(page.pinned ? 'cmd.unpin' : 'cmd.pin'), keys: keysOf('pin'), run: () => run(app.togglePin()) })
    if (page.status !== 'open') add({ id: 'summarise', group: 'page', label: t('cmd.summarise'), run: () => run(app.summarise(page)) })
    add({ id: 'copyQuestion', group: 'page', label: t('cmd.copyQuestion'), run: () => run(app.backend.copyText(page.question)) })
  }

  // The side panel.
  add({ id: 'outline', group: 'panel', label: t('cmd.outline'), keys: keysOf('outline'), run: () => app.showTab('outline') })
  if (app.hasArtifacts(session.id)) add({ id: 'artifacts', group: 'panel', label: t('cmd.artifacts'), keys: keysOf('artifacts'), run: () => app.showTab('artifacts') })
  add({ id: 'panel', group: 'panel', label: t('cmd.panel'), run: () => app.togglePanel() })
  add({ id: 'compactHeader', group: 'panel', label: t(app.headerCompact ? 'cmd.expandHeader' : 'cmd.compactHeader'), run: () => app.toggleHeaderCompact() })
  add({ id: 'compact', group: 'panel', label: t(app.questionCompact ? 'cmd.expandQuestion' : 'cmd.compactQuestion'), run: () => app.toggleQuestionCompact() })

  // When to prompt.
  const mode: SessionMode = session.mode || 'unsafe'
  for (const m of ['always', 'unsafe', 'never'] as const) {
    add({
      id: `mode.${m}`, group: 'prompt', label: t(`cmd.mode.${m}`), checked: mode === m,
      run: () => run(app.backend.setMode(session.id, m).then(v => app.upsertSession(v))),
    })
  }
  add({
    id: 'unattended', group: 'prompt', label: t(session.unattended ? 'cmd.unattendedOff' : 'cmd.unattendedOn'),
    run: () => run(app.backend.setUnattended(session.id, !session.unattended).then(v => app.upsertSession(v))),
  })

  // Model, modifiers and the toolbar's slash commands.
  for (const m of app.boot?.models ?? []) {
    add({ id: `model.${m.value}`, group: 'model', label: t('cmd.model', { model: m.displayName || m.value }), checked: session.model === m.value, run: () => run(app.backend.setModel(session.id, m.value)) })
  }
  for (const m of app.boot?.modifiers ?? []) {
    const on = session.modifiers.includes(m.id)
    add({
      id: `modifier.${m.id}`, group: 'model', label: t(on ? 'cmd.modifierOff' : 'cmd.modifierOn', { modifier: m.label }),
      run: () => run(app.backend.toggleModifier(session.id, m.id, !on).then(v => app.upsertSession(v))),
    })
  }
  if (!session.archived && !session.busy) {
    for (const item of app.boot?.toolbar ?? []) {
      const command = item.command
      if (item.kind === 'slash' && command) add({ id: `slash.${command}`, group: 'model', label: t('cmd.slash', { label: item.label ?? command }), run: () => run(app.send(command)) })
    }
  }
  return out
}

// matches says whether a key event is a command's shortcut.
export function matches(k: Key, e: Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'>): boolean {
  if (e.altKey || !!k.ctrl !== (e.ctrlKey || e.metaKey)) return false
  if (k.key.length === 1 && /[a-z]/i.test(k.key)) {
    return e.key.toLowerCase() === k.key.toLowerCase() && (k.ctrl ? !!k.shift === e.shiftKey : true)
  }
  return e.key === k.key && (k.shift === undefined || k.shift === e.shiftKey)
}

// keyLabel writes a shortcut as the keyboard map and palette show it.
export function keyLabel(k: Key, mac = false): string[] {
  const named: Record<string, string> = { ArrowLeft: '←', ArrowRight: '→', ArrowUp: '↑', ArrowDown: '↓', Home: 'Home', End: 'End', Escape: 'Esc' }
  const out: string[] = []
  if (k.ctrl) out.push(mac ? '⌘' : 'Ctrl')
  if (k.shift) out.push(mac ? '⇧' : 'Shift')
  out.push(named[k.key] ?? (k.key.length === 1 ? k.key.toUpperCase() : k.key))
  return out
}

// score says how well a query matches a label: every query word must
// appear, in order, as a prefix of some word (or a run of its letters for
// short queries); better matches score higher. 0 = no match.
export function score(query: string, label: string): number {
  const q = query.toLowerCase().trim()
  if (!q) return 1
  const l = label.toLowerCase()
  if (l.startsWith(q)) return 100
  const words = l.split(/[^\p{L}\p{N}]+/u).filter(Boolean)
  let total = 0
  let from = 0
  for (const part of q.split(/\s+/)) {
    const at = words.findIndex((w, i) => i >= from && w.startsWith(part))
    if (at >= 0) {
      total += 20 - Math.min(at, 10)
      from = at + 1
      continue
    }
    if (!l.includes(part)) return subsequence(q.replace(/\s+/g, ''), l) ? 1 : 0
    total += 5
  }
  return total
}

function subsequence(q: string, l: string): boolean {
  let j = 0
  for (const c of l) if (c === q[j]) j++
  return j === q.length
}

// rank filters and orders commands for a query: by score, recent first
// among equals (and on an empty query), then as listed.
export function rank(list: readonly Command[], query: string, recent: readonly string[]): Command[] {
  const pos = new Map(list.map((c, i) => [c.id, i]))
  const rec = (id: string) => {
    const i = recent.indexOf(id)
    return i < 0 ? recent.length : i
  }
  return list
    .map(c => ({ c, s: score(query, c.label) }))
    .filter(x => x.s > 0)
    .sort((a, b) => b.s - a.s || rec(a.c.id) - rec(b.c.id) || (pos.get(a.c.id) ?? 0) - (pos.get(b.c.id) ?? 0))
    .map(x => x.c)
}

// Shortcuts that belong to one place rather than a command (the map lists
// them too, under "Elsewhere").
export const LOCAL_KEYS: { keys: string; where: string; does: string }[] = [
  { keys: 'Enter / Shift+Enter', where: 'keys.where.composer', does: 'keys.does.send' },
  { keys: 'Escape', where: 'keys.where.dialogs', does: 'keys.does.close' },
  { keys: 'Alt+↑ / Alt+↓', where: 'keys.where.sidebar', does: 'keys.does.move' },
  { keys: '← / → / Home / End', where: 'keys.where.tabs', does: 'keys.does.tab' },
  { keys: '← / →', where: 'keys.where.resize', does: 'keys.does.resize' },
  { keys: '↑ / ↓ / Enter', where: 'keys.where.search', does: 'keys.does.results' },
]

const RECENT_KEY = 'uncli-recent-commands'

export function loadRecent(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(RECENT_KEY) ?? '[]')
    return Array.isArray(v) ? v.filter(x => typeof x === 'string').slice(0, 20) : []
  } catch {
    return []
  }
}

export function saveRecent(id: string, recent: readonly string[]): string[] {
  const next = [id, ...recent.filter(x => x !== id)].slice(0, 20)
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(next))
  } catch {
    // not kept; still used for this run
  }
  return next
}
