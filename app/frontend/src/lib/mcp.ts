// A session's MCP servers as the UI shows them (decision 0020): which need
// the user, what to call their status, and which just connected after the
// user signed in or reconnected (for a toast).
import type { MCPServer } from './api'

// What a server needs from the user: a sign-in waiting for the browser, a
// sign-in, a connector signed in on the provider's site waiting to be
// reconnected, or a reconnect after it failed; null when nothing.
export type MCPNeed = 'signing' | 'sign-in' | 'connector' | 'failed' | null

export function needOf(sv: MCPServer): MCPNeed {
  if (sv.status === 'connected' || sv.status === 'disabled') return null
  if (sv.signing) return 'signing'
  if (sv.site) return 'connector'
  if (sv.status === 'needs-auth') return 'sign-in'
  if (sv.status === 'failed') return 'failed'
  return null
}

// The status as a short phrase key (mcp.status.*).
export function statusKey(sv: MCPServer): string {
  if (sv.signing) return 'mcp.status.signing'
  if (sv.site) return 'mcp.status.site'
  return `mcp.status.${sv.status}`
}

// connectedNow: servers the user acted on (by key "session:name") that are
// connected in the new list and weren't in the old one.
export function connectedNow(sessionId: string, before: MCPServer[] | undefined, after: MCPServer[] | undefined, acted: Set<string>): MCPServer[] {
  const was = new Map((before ?? []).map(s => [s.name, s.status]))
  return (after ?? []).filter(s => s.status === 'connected' && was.get(s.name) !== 'connected' && acted.has(`${sessionId}:${s.name}`))
}
