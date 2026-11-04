import { describe, expect, it } from 'vitest'
import type { SessionView } from './api'
import { attentionCount } from './attention'

const session = (over: Partial<SessionView>) => ({ id: 's', mcp: [], unattended: false, ...over }) as unknown as SessionView

describe('what wants attention in a folded header', () => {
  it('counts Unattended and each MCP server that needs the user', () => {
    expect(attentionCount(session({}))).toBe(0)
    expect(attentionCount(session({ unattended: true }))).toBe(1)
    expect(
      attentionCount(
        session({
          unattended: true,
          mcp: [
            { name: 'a', status: 'needs-auth' },
            { name: 'b', status: 'failed' },
            { name: 'c', status: 'connected' },
          ] as SessionView['mcp'],
        }),
      ),
    ).toBe(3)
  })

  it('counts each approval card waiting for the user, a host’s too, not one still being judged', () => {
    const approvals = [
      { requestId: 'a', tool: 'Bash', learn: [], askedAt: 1 },
      { requestId: 'b', tool: 'mcp__board__update_checklist', learn: [], askedAt: 2, host: { title: "Update checklist 'Preparation to Leave'" } },
      { requestId: 'c', tool: 'Bash', learn: [], askedAt: 3, judging: true },
    ] as SessionView['approvals']
    expect(attentionCount(session({ approvals }))).toBe(2)
  })
})
