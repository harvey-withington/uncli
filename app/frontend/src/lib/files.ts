// The files a page's turn changed, as the page lists them: paths shown
// relative to the session folder, and what opening one does.
import type { Editor, Preferences, TouchedFile } from './api'

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
