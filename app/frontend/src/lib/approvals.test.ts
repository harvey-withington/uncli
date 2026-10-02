import { describe, expect, it } from 'vitest'
import type { Approval } from './api'
import { describeApproval, mcpName, ruleLabel } from './approvals'

const approval = (tool: string, input: unknown, description = ''): Approval => ({ requestId: 'r', tool, input, description, suggestions: [], askedAt: 0 })

describe('approvals in plain words', () => {
  it('describes common tools', () => {
    expect(describeApproval(approval('Bash', { command: 'git push', description: 'Push it' }))).toMatchObject({ title: 'Run a command', target: 'git push', note: 'Push it' })
    expect(describeApproval(approval('PowerShell', { command: 'npm test', description: 'Run tests' }))).toMatchObject({ title: 'Run a command', target: 'npm test', note: 'Run tests' })
    expect(describeApproval(approval('Write', { file_path: 'a.txt', content: 'hi' }))).toMatchObject({ title: 'Write a file', target: 'a.txt', body: 'hi' })
    expect(describeApproval(approval('Edit', { file_path: 'a.go', old_string: 'x', new_string: 'y' }))).toMatchObject({ title: 'Edit a file', body: '- x\n+ y', lang: 'diff' })
    expect(describeApproval(approval('WebFetch', { url: 'https://x' }))).toMatchObject({ title: 'Fetch a web page', target: 'https://x' })
    const long = Array.from({ length: 20 }, (_, i) => `line ${i}`).join('\n')
    expect(describeApproval(approval('Write', { file_path: 'b', content: long })).body).toMatch(/line 11\n… 8 more lines$/)
  })

  it('names MCP tools by server', () => {
    expect(mcpName('mcp__bruv__create_card')).toEqual({ server: 'bruv', name: 'create_card' })
    expect(describeApproval(approval('mcp__bruv__create_card', { title: 'x' }))).toMatchObject({ title: 'Use create_card (bruv)', lang: 'json' })
  })

  it('labels rules', () => {
    expect(ruleLabel({ tool: 'Bash', prefix: 'git push' })).toBe('git push …')
    expect(ruleLabel({ tool: 'Bash', prefix: 'git:local' })).toMatch(/^Local git changes/)
    expect(ruleLabel({ tool: 'Bash' })).toBe('Any command')
    expect(ruleLabel({ tool: 'PowerShell' })).toBe('Any command')
    expect(ruleLabel({ tool: 'PowerShell', prefix: 'npm test' })).toBe('npm test …')
    expect(ruleLabel({ tool: 'Write' })).toBe('Write')
    expect(ruleLabel({ tool: 'mcp__bruv__create_card' })).toBe('create_card (bruv)')
  })
})
