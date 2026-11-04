// What in a session's header wants the user's eye while it's folded away
// (the compact header's Options, the desktop's collapsed header): a dot on
// the button that opens it. Unattended on, and each MCP server that needs
// signing in or has failed. Not what's merely unavailable here (Open
// folder on a phone): that never changes, so a dot would never go.
import type { SessionView } from './api'
import { needOf } from './mcp'

export function attentionCount(session: SessionView): number {
  const servers = (session.mcp ?? []).filter(sv => needOf(sv) !== null).length
  return (session.unattended ? 1 : 0) + servers
}
