// The container editor's rules (Settings → Containers). They mirror the
// backend's (internal/app/containers.go validProfile), so a mistake shows
// as it's typed; the backend still checks everything it saves.
import type { ContainerFolder, ContainerInfo, ContainerProfile } from '@uncli/chat/lib/api'

const ID = /^[a-z0-9][a-z0-9-]{0,40}$/
const PACKAGE = /^[a-z0-9][a-z0-9.+_-]{0,63}$/
const FOLDER_NAME = /^[a-z0-9][a-z0-9._-]{0,40}$/
const VARS = ['USERPROFILE', 'APPDATA', 'LOCALAPPDATA']
export const MAX_STEP = 4000
const MAX_LOCAL = 200

export const validPackage = (name: string) => PACKAGE.test(name)

// An id for a new container, from its name: lower-case letters, digits
// and dashes, unlike any taken one.
export function idFor(label: string, taken: string[]): string {
  const base = label.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 36).replace(/-+$/, '') || 'container'
  let id = base
  for (let n = 2; taken.includes(id); n++) id = `${base}-${n}`
  return id
}

// Package names typed into the field: split on spaces and commas.
export function splitPackages(text: string): string[] {
  return text.split(/[\s,]+/).map(s => s.trim().toLowerCase()).filter(Boolean)
}

export function addPackages(have: string[], text: string): string[] {
  return [...new Set([...have, ...splitPackages(text)])]
}

// What a container's profile is, without its build state.
export function profileOf(c: ContainerInfo): ContainerProfile {
  return {
    id: c.id,
    label: c.label,
    description: c.description ?? '',
    base: c.base,
    packages: [...c.packages],
    setup: [...(c.setup ?? [])],
    brain: c.brain || 'sandboxed',
    mcp: c.mcp || 'none',
    connectors: c.connectors || 'none',
    secrets: c.secrets || 'none',
    folders: (c.folders ?? []).map(f => ({ source: f.source, name: f.name, access: f.access === 'rw' ? 'rw' : 'ro' })),
    local: [...(c.local ?? [])],
  }
}

export function blank(id: string, base: string): ContainerProfile {
  return { id, label: '', description: '', base, packages: [], setup: [], brain: 'sandboxed', mcp: 'none', connectors: 'none', secrets: 'none', folders: [], local: [] }
}

// What stops a profile being saved, as locale keys.
export function problems(p: ContainerProfile): string[] {
  const out: string[] = []
  if (!p.label.trim()) out.push('containerEdit.problem.name')
  if (!ID.test(p.id)) out.push('containerEdit.problem.id')
  if (p.packages.some(x => !validPackage(x))) out.push('containerEdit.problem.package')
  if (p.setup.some(s => s.length > MAX_STEP || s.includes('\0'))) out.push('containerEdit.problem.step')
  const folders = (p.folders ?? []).filter(f => f.source.trim() || f.name.trim())
  if (folders.some(f => folderProblem(f))) out.push('containerEdit.problem.folder')
  if (new Set(folders.map(f => f.name.trim())).size < folders.length) out.push('containerEdit.problem.folderTwice')
  if (localProblems(p.local ?? []).some(Boolean)) out.push('containerEdit.problem.local')
  return out
}

// A local path as it's mounted: forward slashes, no ./ or trailing slash.
export const cleanLocal = (s: string) => s.trim().replace(/\\/g, '/').split('/').filter(x => x && x !== '.').join('/')

// What's wrong with one local path (decision 0032), as a locale key; ''
// if nothing. Mirrors the backend's localPath.
export function localProblem(s: string): string {
  const v = s.trim()
  if (!v || v.length > MAX_LOCAL || /^[\\/]/.test(v) || v[1] === ':') return 'containerEdit.localFull'
  if (/[<>:"|?*]/.test(v) || [...v].some(ch => ch.charCodeAt(0) < 32 || ch.charCodeAt(0) === 127)) return 'containerEdit.localChars'
  const segs = v.replace(/\\/g, '/').split('/').filter(x => x && x !== '.')
  if (segs.includes('..')) return 'containerEdit.localUp'
  if (segs.some(x => x.toLowerCase() === '.git')) return 'containerEdit.localGit'
  if (segs.some(x => /^ |[. ]$/.test(x))) return 'containerEdit.localChars'
  if (!segs.length) return 'containerEdit.localFull'
  return ''
}

// Each local path's problem, a clash with one above it included; blank
// rows have none (they're dropped when it's saved).
export function localProblems(paths: string[]): string[] {
  const seen: string[] = []
  return paths.map(p => {
    if (!p.trim()) return ''
    const why = localProblem(p)
    if (why) return why
    const c = cleanLocal(p).toLowerCase()
    if (seen.includes(c)) return 'containerEdit.localTwice'
    if (seen.some(q => c.startsWith(`${q}/`) || q.startsWith(`${c}/`))) return 'containerEdit.localNested'
    seen.push(c)
    return ''
  })
}

// What's wrong with one folder, as a locale key; '' if nothing the editor
// can tell. The backend also refuses folders inside ones with sign-ins.
export function folderProblem(f: ContainerFolder): string {
  const name = f.name.trim()
  const src = f.source.trim()
  if (!FOLDER_NAME.test(name)) return 'containerEdit.folderBadName'
  const vars = [...src.matchAll(/%([^%]*)%/g)].map(m => m[1]!.toUpperCase())
  if (vars.some(v => !VARS.includes(v))) return 'containerEdit.folderBadVar'
  if (!vars.length && !/^[A-Za-z]:[\\/]/.test(src)) return 'containerEdit.folderNotFull'
  const bare = src.replace(/[\\/]+$/, '').toUpperCase()
  if (/^[A-Z]:$/.test(bare) || VARS.some(v => bare === `%${v}%`) || /^[A-Z]:\\(WINDOWS|PROGRAM FILES( \(X86\))?|PROGRAMDATA|USERS)$/.test(bare)) {
    return 'containerEdit.folderTooWide'
  }
  return ''
}

// The profile as it's saved: trimmed, empty steps left out.
export function tidy(p: ContainerProfile): ContainerProfile {
  return {
    ...p, label: p.label.trim(), description: p.description?.trim() ?? '', setup: p.setup.map(s => s.trim()).filter(Boolean),
    folders: (p.folders ?? []).map(f => ({ source: f.source.trim(), name: f.name.trim(), access: f.access === 'rw' ? 'rw' as const : 'ro' as const })).filter(f => f.source || f.name),
    local: (p.local ?? []).map(s => (localProblem(s) ? s.trim() : cleanLocal(s))).filter(Boolean),
  }
}

// A name for a folder from its source: its last part, as a folder name.
export function folderNameFor(source: string): string {
  const last = source.trim().replace(/[\\/]+$/, '').split(/[\\/]/).pop() ?? ''
  return last.replace(/%/g, '').toLowerCase().replace(/[^a-z0-9._-]+/g, '-').replace(/^[^a-z0-9]+/, '').slice(0, 40)
}

export function same(a: ContainerProfile, b: ContainerProfile): boolean {
  return JSON.stringify(tidy(a)) === JSON.stringify(tidy(b))
}
