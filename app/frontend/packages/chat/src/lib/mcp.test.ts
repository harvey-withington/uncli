import { describe, expect, it } from 'vitest'
import type { MCPServer } from './api'
import { needOf, statusKey } from './mcp'

describe('mcp', () => {
  const drive: MCPServer = { name: 'Drive', status: 'needs-auth', tools: 0 }

  it('says what a server needs from the user', () => {
    expect(needOf(drive)).toBe('sign-in')
    expect(needOf({ ...drive, status: 'failed' })).toBe('failed')
    expect(needOf({ ...drive, signing: true })).toBe('signing')
    expect(needOf({ ...drive, status: 'connected', tools: 4 })).toBeNull()
  })

  it('treats a server the user turned off as off, whatever its CLI still says', () => {
    const off = { ...drive, off: true }
    expect(needOf(off)).toBeNull()
    expect(statusKey(off)).toBe('mcp.status.disabled')
    expect(statusKey({ ...off, signing: true })).toBe('mcp.status.disabled')
  })
})
