// What in a session wants the user's eye while its header is folded away
// (the compact header's Options, the desktop's collapsed header): a dot on
// the button that opens it. Each approval card waiting for the user (a
// host's among them; not one the quick-task model is still judging),
// Unattended on, and each MCP server that needs signing in or has failed.
// Not what's merely unavailable here (Open folder on a phone): that never
// changes, so a dot would never go.
import type { SessionView } from './api'
import { needOf } from './mcp'

export function attentionCount(session: SessionView): number {
  const servers = (session.mcp ?? []).filter(sv => needOf(sv) !== null).length
  const cards = (session.approvals ?? []).filter(a => !a.judging).length
  return cards + (session.unattended ? 1 : 0) + servers
}
