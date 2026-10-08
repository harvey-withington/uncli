// The container editor's rules (Settings → Containers). They mirror the
// backend's (internal/app/containers.go validProfile), so a mistake shows
// as it's typed; the backend still checks everything it saves.
import type { ContainerInfo, ContainerProfile } from './api'

const ID = /^[a-z0-9][a-z0-9-]{0,40}$/
const PACKAGE = /^[a-z0-9][a-z0-9.+_-]{0,63}$/
export const MAX_STEP = 4000

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
  }
}

export function blank(id: string, base: string): ContainerProfile {
  return { id, label: '', description: '', base, packages: [], setup: [], brain: 'sandboxed', mcp: 'none', connectors: 'none' }
}

// What stops a profile being saved, as locale keys.
export function problems(p: ContainerProfile): string[] {
  const out: string[] = []
  if (!p.label.trim()) out.push('containerEdit.problem.name')
  if (!ID.test(p.id)) out.push('containerEdit.problem.id')
  if (p.packages.some(x => !validPackage(x))) out.push('containerEdit.problem.package')
  if (p.setup.some(s => s.length > MAX_STEP || s.includes('\0'))) out.push('containerEdit.problem.step')
  return out
}

// The profile as it's saved: trimmed, empty steps left out.
export function tidy(p: ContainerProfile): ContainerProfile {
  return { ...p, label: p.label.trim(), description: p.description?.trim() ?? '', setup: p.setup.map(s => s.trim()).filter(Boolean) }
}

export function same(a: ContainerProfile, b: ContainerProfile): boolean {
  return JSON.stringify(tidy(a)) === JSON.stringify(tidy(b))
}
