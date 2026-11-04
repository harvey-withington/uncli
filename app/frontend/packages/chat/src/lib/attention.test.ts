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
})
