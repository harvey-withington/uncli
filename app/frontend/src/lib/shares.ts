import type { Capabilities, NewSessionChoices, Profile, Share } from './api'

// What an isolated session can have of the user's own: their MCP servers
// and connectors (tools) and their skills (decision 0016). The backend
// decides for an existing session (SessionView.shareable); these mirror it
// for one not made yet.
export const SHARES: Share[] = ['tools', 'skills']

// What a new session could share: only an isolated type's, only on this
// machine (a container shares what its profile does), only what its CLI can.
export function shareableFor(profile: Profile | undefined, caps: Capabilities | undefined, container: string): Share[] {
  if (!profile?.isolated || container) return []
  return SHARES.filter(s => (s === 'tools' ? caps?.sharedTools : caps?.sharedSkills))
}

// What a new session starts with: what this type's last session shared,
// else the type's defaults, of what it can.
export function startingShares(profile: Profile | undefined, last: NewSessionChoices | undefined, can: Share[]): Share[] {
  const want = (profile && last?.shares?.[profile.id]) ?? profile?.shares ?? []
  return can.filter(s => want.includes(s))
}

// shares with one switched on or off, in the usual order.
export function toggled(shares: Share[], share: Share, on: boolean): Share[] {
  return SHARES.filter(s => (s === share ? on : shares.includes(s)))
}

// The header's words for what a session has (a shares.note.* key).
export function sharesNote(shares: Share[] | undefined): string {
  const has = (s: Share) => !!shares?.includes(s)
  if (has('tools') && has('skills')) return 'shares.note.both'
  if (has('tools')) return 'shares.note.tools'
  if (has('skills')) return 'shares.note.skills'
  return 'shares.note.none'
}
