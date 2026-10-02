// Approval cards and permission rules in plain words: what a tool use
// will do, and what a rule covers.
import type { Approval, ToolRule } from './api'
import { t } from './i18n.svelte'

export interface ApprovalView {
  title: string // "Run a command", "Write a file"…
  target?: string // the command, path, URL or query
  note?: string // Claude's own one-line explanation, as plain text
  body?: string // content to show below, already trimmed
  bodyLabel?: string
  lang?: string // for code-looking bodies
}

const PREVIEW_LINES = 12

// Tools that run a command line: Bash, and PowerShell on Windows. Their
// rules are about the command, whichever shell asks.
export const isShell = (tool: string) => tool === 'Bash' || tool === 'PowerShell'

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function preview(text: string): string {
  const lines = text.split('\n')
  if (lines.length <= PREVIEW_LINES) return text
  return lines.slice(0, PREVIEW_LINES).join('\n') + '\n' + t('approval.moreLines', { n: lines.length - PREVIEW_LINES })
}

// mcpName splits "mcp__server__tool" into its server and tool.
export function mcpName(tool: string): { server: string; name: string } | null {
  const m = /^mcp__(.+?)__(.+)$/.exec(tool)
  return m ? { server: m[1] as string, name: m[2] as string } : null
}

export function describeApproval(a: Approval): ApprovalView {
  const input = (a.input ?? {}) as Record<string, unknown>
  switch (a.tool) {
    case 'Bash':
    case 'PowerShell':
      return { title: t('approval.bash'), target: str(input.command), note: str(input.description) || undefined }
    case 'Write':
      return { title: t('approval.write'), target: str(input.file_path), body: preview(str(input.content)), bodyLabel: t('approval.content') }
    case 'Edit':
      return {
        title: t('approval.edit'), target: str(input.file_path),
        body: preview(`- ${str(input.old_string).replace(/\n/g, '\n- ')}\n+ ${str(input.new_string).replace(/\n/g, '\n+ ')}`), lang: 'diff',
      }
    case 'MultiEdit':
      return { title: t('approval.edit'), target: str(input.file_path), body: t('approval.edits', { n: Array.isArray(input.edits) ? input.edits.length : 0 }) }
    case 'NotebookEdit':
      return { title: t('approval.edit'), target: str(input.notebook_path) }
    case 'Read':
      return { title: t('approval.read'), target: str(input.file_path) }
    case 'WebFetch':
      return { title: t('approval.fetch'), target: str(input.url) }
    case 'WebSearch':
      return { title: t('approval.search'), target: str(input.query) }
  }
  const mcp = mcpName(a.tool)
  const json = JSON.stringify(a.input ?? {}, null, 2)
  return {
    title: mcp ? t('approval.mcp', { name: mcp.name, server: mcp.server }) : t('approval.tool', { tool: a.tool }),
    target: a.description && a.description !== a.tool ? a.description : undefined,
    body: json === '{}' ? undefined : preview(json), bodyLabel: t('approval.input'), lang: 'json',
  }
}

// ruleLabel says what a rule covers, for "Always allow …" and the rules list.
export function ruleLabel(r: Pick<ToolRule, 'tool' | 'prefix'>): string {
  if (isShell(r.tool)) {
    if (!r.prefix) return t('rule.allCommands')
    if (r.prefix.startsWith('git:')) return t(`rule.${r.prefix.replace(':', '.')}`)
    return t('rule.command', { prefix: r.prefix })
  }
  const mcp = mcpName(r.tool)
  if (mcp) return t('rule.mcp', { name: mcp.name, server: mcp.server })
  return t('rule.tool', { tool: r.tool })
}

// The git classes, for the rules dialog, with what each covers.
export const GIT_CLASSES = ['git:read', 'git:local', 'git:commit', 'git:publish', 'git:destructive'] as const
