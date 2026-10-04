// Approval cards and the safe list in plain words: what a tool use will
// do, why it prompts, and what an entry covers.
import type { Approval, Approver, Risk, SafeClass, SafeEntry, Scope, SessionMode, SessionView, ToolRule, TraceReason } from './api'
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

// ruleLabel says what an older trace's rule covered.
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

// The prompt levels, in the order the switch shows them.
export const MODES: { id: SessionMode; icon: string }[] = [
  { id: 'always', icon: 'hand' },
  { id: 'unsafe', icon: 'shield' },
  { id: 'never', icon: 'zap' },
]

export const modeOf = (s: Pick<SessionView, 'mode'>): SessionMode => s.mode || 'unsafe'

// classLabel names a safe-list class: "npm run test", "git push --force",
// "Local git changes (…)", "touch_note (notes)".
export function classLabel(c: SafeClass): string {
  if (c.kind === 'tool') {
    const mcp = mcpName(c.words)
    return mcp ? t('rule.mcp', { name: mcp.name, server: mcp.server }) : c.words
  }
  if (c.words.startsWith('git:')) return t(`rule.${c.words.replace(':', '.')}`)
  return c.flags ? `${c.words} ${c.flags}` : c.words
}

// scopeLabel: where an entry applies.
export const scopeLabel = (e: Pick<SafeEntry, 'folder'>) => t(e.folder ? 'scope.project' : 'scope.all')
export const scopeOf = (e: Pick<SafeEntry, 'folder'>): Scope => (e.folder ? 'project' : 'all')

// allowLabel says what a session type's allowlist entry covers, from the
// CLI's syntax: "Bash(npm test:*)" is "npm test …", "Write(./artifacts/**)"
// is "Write in ./artifacts/**".
export function allowLabel(entry: string): string {
  const m = /^(\w+)(?:\((.*)\))?$/.exec(entry.trim())
  if (!m) return entry
  const tool = m[1] as string
  const spec = m[2]?.trim()
  if (!spec) return ruleLabel({ tool })
  if (isShell(tool)) return spec.endsWith(':*') ? t('rule.command', { prefix: spec.slice(0, -2).trim() }) : spec
  return t('allow.path', { tool, path: spec })
}

// riskLabel says how something unsafe could do harm, as the end of a
// sentence ("deletes or overwrites things for good").
export const riskLabel = (risk: Risk | undefined) => t(`risk.${risk ?? 'system'}`)

// reasonLabel says why one part of a command (or a tool use) ran, prompted
// or was blocked. A judgement by the quick-task model adds its reason.
export function reasonLabel(r: TraceReason): string {
  const rule = r.rule ? ruleLabel(r.rule) : ''
  const scope = r.entry ? scopeLabel(r.entry) : ''
  let text: string
  switch (r.by) {
    case 'listed': text = t('why.listed', { scope }); break
    case 'blocked': text = t('why.blocked', { scope }); break
    case 'unsafe': text = r.entry ? t('why.marked', { scope }) : t('why.unsafe', { risk: riskLabel(r.risk) }); break
    case 'builtin':
    case 'profile': text = t('why.profile', { entry: allowLabel(r.allow ?? '') }); break
    case 'rule': text = t('why.rule', { rule }); break
    case 'session': text = t('why.session', { rule }); break
    case 'deny': text = t('why.deny', { rule }); break
    case 'ask': text = t('why.ask', { rule }); break
    case 'risky': text = t('why.risky', { risk: riskLabel(r.risk) }); break
    default: text = t(`why.${r.by}`)
  }
  return r.judged && r.note ? t('why.judged', { text, note: r.note, model: r.judged }) : text
}

// The reasons a card is waiting for the user, in plain words: each part
// that needs them, with what it could do. Parts that would just run aren't
// listed; under Always one line says why everything prompts.
export function askingReasons(a: Approval): { part?: string; text: string }[] {
  const out: { part?: string; text: string }[] = []
  for (const r of a.why ?? []) {
    let text = ''
    switch (r.by) {
      case 'unsafe':
        text = r.entry ? t('asking.marked', { scope: scopeLabel(r.entry) }) : r.note || capital(riskLabel(r.risk)) + '.'
        break
      case 'risky': text = r.note || capital(riskLabel(r.risk)) + '.'; break
      case 'unknown': text = t('asking.unknown'); break
      case 'always':
        if (!out.some(o => !o.part)) out.unshift({ text: t('asking.always') })
        continue
      case 'ask': text = t('why.ask', { rule: r.rule ? ruleLabel(r.rule) : '' }); break
      case 'outside': text = t('asking.outside'); break
      default: continue
    }
    out.push({ part: r.part, text })
  }
  return out
}

// fixedReason says why a card can only be allowed once: a part whose risk
// comes from what it touches or does there, not from the program.
export function fixedReason(a: Approval): string | undefined {
  const r = (a.why ?? []).find(w => w.fixed && w.by !== 'looks')
  return r?.fixed ? t(`fixed.${r.fixed}`) : undefined
}

// promptable: the classes a trace row's "This should prompt" would mark
// unsafe: the parts that ran because UNCLI judged them safe.
export function promptable(why: TraceReason[] = []): SafeClass[] {
  const out: SafeClass[] = []
  for (const r of why) {
    if (!r.class || !['safe', 'inside', 'builtin'].includes(r.by)) continue
    if (!out.some(c => c.kind === r.class?.kind && c.words === r.class.words && (c.flags ?? '') === (r.class.flags ?? ''))) out.push(r.class)
  }
  return out
}

const capital = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

// Short names of who allowed something, for a command whose parts ran for
// different reasons.
const SHORT: Partial<Record<TraceReason['by'], string>> = {
  looks: 'who.looks', safe: 'who.safe', listed: 'who.listed', builtin: 'who.profile', inside: 'who.inside', never: 'who.never',
  rule: 'who.rule', session: 'who.session', profile: 'who.profile', routine: 'who.routine', full: 'who.full',
}

// approvedLabel says who let a tool use run, on its trace row.
export function approvedLabel(approved: Approver, why: TraceReason[] = []): string {
  if (approved !== 'mixed') return t(`trace.allowed.${approved}`)
  const who = [...new Set(why.map(w => SHORT[w.by]).filter((k): k is string => !!k))].map(k => t(k))
  return t('trace.allowed.mixed', { who: who.join(', ') })
}

// The scope "This is safe" and "This should prompt" last used, so the next
// card starts there; This project the first time.
const SCOPE_KEY = 'uncli-scope'

export function loadScope(): Scope {
  try {
    return localStorage.getItem(SCOPE_KEY) === 'all' ? 'all' : 'project'
  } catch {
    return 'project'
  }
}

export function saveScope(scope: Scope) {
  try {
    localStorage.setItem(SCOPE_KEY, scope)
  } catch {
    // only a convenience
  }
}
