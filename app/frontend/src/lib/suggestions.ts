// Containers a repository suggests in .uncli (decision 0028): what the
// import form starts from, and what it says reaches outside the container
// as the user changes it.
import type { ContainerProfile, StepRisks, SuggestedContainer } from '@uncli/chat/lib/api'

export type Reach = SuggestedContainer['reaches'][number]

// The editor's starting point for importing (or syncing) a suggestion: the
// project's version, under the user's own container's id when syncing.
export function importStart(s: SuggestedContainer): ContainerProfile {
  const p = s.profile
  return {
    id: s.yours ?? p.id,
    label: p.label,
    description: p.description ?? '',
    base: p.base,
    packages: [...(p.packages ?? [])],
    setup: [...(p.setup ?? [])],
    brain: p.brain || 'sandboxed',
    mcp: p.mcp || 'none',
    connectors: p.connectors || 'none',
    secrets: p.secrets || 'none',
    folders: (p.folders ?? []).map(f => ({ source: f.source, name: f.name, access: f.access === 'rw' ? 'rw' : 'ro' })),
  }
}

// What of the user's a container would share, as it stands in the form.
export function reachesOf(p: ContainerProfile): Reach[] {
  const out: Reach[] = []
  if (p.brain === 'shared') out.push('brain')
  if (p.mcp === 'shared') out.push('mcp')
  if (p.connectors === 'shared') out.push('connectors')
  if (p.secrets === 'shared') out.push('secrets')
  if ((p.folders ?? []).some(f => f.source.trim())) out.push('folders')
  return out
}

// The flagged steps still in the form (the user may have removed or
// changed them; a changed one is the user's own).
export function flaggedIn(setup: string[], flagged: StepRisks[]): StepRisks[] {
  const have = new Set(setup.map(s => s.trim()))
  return flagged.filter(f => have.has(f.step.trim()))
}

// The reason a step is flagged, as a locale key.
export const whyKey = (why: string) => `suggested.why.${['publishes', 'remote', 'secrets', 'runs-code', 'cloud', 'mounts', 'unreadable'].includes(why) ? why : 'other'}`
