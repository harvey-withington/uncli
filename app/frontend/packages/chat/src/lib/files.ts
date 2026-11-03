// The files a page's turn changed, as the page lists them: paths shown
// relative to the session folder, and what opening one does.
import type { Editor, GitHead, HeadMove, Preferences, TouchedFile } from './api'
import { t } from './i18n.svelte'

// How a file was touched, as the row's icon.
export const HOW_ICON: Record<TouchedFile['how'], string> = {
  write: 'file-plus',
  edit: 'file-pen',
  command: 'terminal',
  deleted: 'file-x',
}

const sep = /[\\/]/

// relPath shows path relative to the session folder when it's inside it
// (Windows paths compare without case, either slash), else as it is.
export function relPath(workdir: string, path: string): string {
  const w = workdir.split(sep).filter(Boolean)
  const p = path.split(sep).filter(Boolean)
  const windows = /^[a-z]:/i.test(path) || path.includes('\\')
  const same = (a: string, b: string) => (windows ? a.toLowerCase() === b.toLowerCase() : a === b)
  if (w.length === 0 || p.length <= w.length || !w.every((part, i) => same(part, p[i] ?? ''))) return path
  return p.slice(w.length).join('/')
}

// splitByMove separates a turn's files when the repo's HEAD moved: what the
// agent's tools wrote or edited stays listed, and what commands changed
// (mostly the checkout's, pull's or rebase's) is folded into one line.
export function splitByMove(files: TouchedFile[], move: HeadMove | undefined): { own: TouchedFile[]; moved: TouchedFile[] } {
  if (!move) return { own: files, moved: [] }
  const byCommand = (f: TouchedFile) => f.how === 'command' || f.how === 'deleted'
  return { own: files.filter(f => !byCommand(f)), moved: files.filter(byCommand) }
}

const short = (h: GitHead) => h.commit.slice(0, 7)

// moveLabel says where HEAD went: "Switched from main to server-mode",
// "main moved from 1a2b3c4 to 5d6e7f8" (a pull, merge, rebase or reset),
// or "Checked out 5d6e7f8" (detached). n is how many files it folds.
export function moveLabel(move: HeadMove, n: number): string {
  const { from, to } = move
  const files = n === 1 ? t('files.move.filesOne') : t('files.move.files', { n })
  if (to.branch && from.branch && to.branch !== from.branch) return t('files.move.switched', { from: from.branch, to: to.branch, files })
  if (to.branch && to.branch === from.branch) return t('files.move.moved', { branch: to.branch, from: short(from), to: short(to), files })
  if (to.branch) return t('files.move.switchedTo', { to: to.branch, files })
  return t('files.move.detached', { to: short(to), files })
}

// splitPath gives the folder part (with its trailing slash) and the name.
export function splitPath(path: string): { dir: string; name: string } {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  return i < 0 ? { dir: '', name: path } : { dir: path.slice(0, i + 1), name: path.slice(i + 1) }
}

// editorLabel names the editor that Open uses in a session that links to
// an IDE, following the Go side's ide.Resolve: a preset, auto (the first
// installed) or the user's own command. Empty when there is none.
export function editorLabel(prefs: Pick<Preferences, 'editor' | 'editorCommand'> | undefined, editors: Editor[]): string {
  const choice = prefs?.editor || 'auto'
  if (choice === 'custom') return (prefs?.editorCommand ?? '').trim().split(/\s+/)[0]?.replace(/"/g, '') ?? ''
  if (choice === 'auto') return editors.find(e => e.found)?.label ?? ''
  return editors.find(e => e.id === choice)?.label ?? ''
}

// Documents and media the default app may open (kept in step with
// documentTypes in internal/ide): anything else is only shown in its folder.
const DOCUMENT_TYPES = new Set([
  'html', 'htm', 'svg', 'md', 'markdown', 'txt', 'pdf', 'png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico',
  'csv', 'tsv', 'json', 'xml', 'yaml', 'yml', 'log', 'docx', 'xlsx', 'pptx', 'odt', 'ods', 'odp', 'rtf',
  'mp3', 'wav', 'mp4', 'webm', 'mov',
])

export function canOpenDefault(path: string): boolean {
  const name = splitPath(path).name
  const dot = name.lastIndexOf('.')
  return dot > 0 && DOCUMENT_TYPES.has(name.slice(dot + 1).toLowerCase())
}
