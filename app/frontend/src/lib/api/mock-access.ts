// When to prompt and the safe list, for the mock backend: the real
// backend's decision (internal/session/modes.go), cut down to a few
// commands and tools.
import type { ClassPreview, Explanation, SafeClass, SafeEntry, SessionView, ToolClassView, TraceReason, UnknownCommands } from './types'

// A tool use the mock's Claude asks about when the question mentions it.
export interface MockAsk {
  match: RegExp
  tool: string
  input: Record<string, unknown>
  class: ToolClassView
  why: TraceReason // why it's unsafe (or that UNCLI can't place it)
  learn: SafeClass // what "This is safe" remembers
  answers: Record<Outcome, string>
}

// How a tool use ended: run, denied on its card (or blocked), declined
// because the user is away.
export type Outcome = 'allow' | 'deny' | 'away'

const PUSH: SafeClass = { kind: 'command', words: 'git push' }
const NOTE: SafeClass = { kind: 'tool', words: 'mcp__notes__touch_note' }

export const MOCK_ASKS: MockAsk[] = [
  {
    match: /\bpush\b/i, tool: 'PowerShell',
    input: { command: 'git push origin main', description: 'Push the branch to origin' },
    class: { write: true, openWorld: true, source: 'built-in' },
    why: { part: 'git push origin main', by: 'unsafe', risk: 'publishes', class: PUSH },
    learn: PUSH,
    answers: {
      allow: 'Pushed **main** to origin.',
      deny: "I didn't push: the push was denied. The commit is still local.",
      away: "I didn't push: you're away, so the push was declined. The commit is still local.",
    },
  },
  {
    match: /\bnote\b/i, tool: 'mcp__notes__touch_note',
    input: { name: 'groceries', text: 'oat milk, lemons' },
    class: { write: true, destructive: true, openWorld: true, source: 'unknown' },
    why: { by: 'unknown', class: NOTE },
    learn: NOTE,
    answers: {
      allow: 'Saved the note **groceries**.',
      deny: "I didn't save the note: saving it was denied.",
      away: "I didn't save the note: you're away, so it was declined.",
    },
  },
]

// The session types' lists, as in config/defaults/profiles.yaml.
export const MOCK_ALLOWLISTS: Record<string, string[]> = {
  chat: ['WebSearch', 'WebFetch', 'Write(./artifacts/**)'],
  cowork: ['Read', 'Write', 'Edit', 'Glob', 'Grep', 'WebSearch', 'WebFetch'],
  code: ['Bash(git status:*)', 'Bash(git diff:*)', 'Bash(git log:*)', 'Bash(git show:*)', 'Bash(npm test:*)', 'Bash(npm run:*)', 'Bash(go test:*)', 'Bash(go build:*)', 'Bash(go vet:*)'],
}

// The safe-list entry that applies to a class in a session's project:
// the most specific (its words a prefix of the class's), this project's
// first.
export function entryFor(list: SafeEntry[], c: SafeClass, workdir: string): SafeEntry | undefined {
  const words = c.words.split(' ')
  let best: SafeEntry | undefined
  let bestScore = 0
  for (const e of list) {
    if (e.kind !== c.kind || (e.folder && e.folder !== workdir)) continue
    const ew = e.words.split(' ')
    const flagsOk = (c.flags ?? '').split(' ').filter(Boolean).every(f => (e.flags ?? '').split(' ').includes(f))
    if (ew.length > words.length || !ew.every((w, i) => w === words[i]) || !flagsOk) continue
    const score = ew.length * 2 + (e.folder ? 1 : 0)
    if (score > bestScore) [best, bestScore] = [e, score]
  }
  return best
}

export interface MockVerdict {
  action: 'allow' | 'deny' | 'ask' | 'judge'
  why: TraceReason[]
  by?: string
}

const JUDGED: TraceReason = { by: 'unsafe', risk: 'publishes', judged: 'haiku', note: 'Saves a note to your notes service, outside this computer.', class: NOTE }

// mockJudge: what a session does with one of the mock's tool uses.
export function mockJudge(s: SessionView, ask: MockAsk, list: SafeEntry[], setting: UnknownCommands, judged: boolean): MockVerdict {
  const mode = s.mode || 'unsafe'
  const base = ask.why
  const entry = entryFor(list, ask.learn, s.workdir)
  if (entry?.verdict === 'blocked') return { action: 'deny', why: [{ ...base, by: 'blocked', entry }] }
  if (mode === 'never') return { action: 'allow', why: [{ ...base, by: 'never' }], by: 'never' }
  if (mode === 'always') return { action: 'ask', why: [{ ...base, by: 'always' }] }
  if (entry?.verdict === 'safe') return { action: 'allow', why: [{ ...base, by: 'listed', entry }], by: 'listed' }
  if (entry?.verdict === 'unsafe') return { action: 'ask', why: [{ ...base, by: 'unsafe', entry }] }
  if (base.by === 'unknown') {
    if (setting === 'inside') return { action: 'allow', why: [{ ...base, by: 'inside' }], by: 'inside' }
    if (setting === 'model') return judged ? { action: 'ask', why: [JUDGED] } : { action: 'judge', why: [{ ...base, by: 'judging' }] }
  }
  return { action: 'ask', why: [base] }
}

const LOOKS = /^(ls|dir|pwd|cat|echo|get-childitem|gci|get-content|test-path|git (status|diff|log|show)\b)/i
const FILTERS = /^(head|tail|grep|sort|select-object|select-string|format-\w+|measure-object)\b/i
const ROUTINE = /^(npm|pnpm|yarn|go|cargo|make|npx|git (add|commit|switch|fetch|pull|stash)|new-item|set-content|mkdir)\b/i
const RISKY: [RegExp, TraceReason['risk']][] = [
  [/^git push\b/i, 'publishes'], [/^(npm|pnpm|yarn) publish\b/i, 'publishes'], [/\s(-g|--global)\b/, 'installs'],
  [/^(rm|del|remove-item)\b.*\s(-r|-rf|-recurse)\b/i, 'deletes'], [/^git (reset --hard|clean)\b/i, 'deletes'],
  [/^(winget|choco|pip install)\b/i, 'installs'], [/^(stop-process|taskkill|kill)\b/i, 'stops'],
]

// The class a part would be remembered as: the program and, for programs
// with subcommands, the subcommand (npm run keeps the script).
export function mockClassOf(part: string): SafeClass {
  const w = part.trim().split(/\s+/).filter(x => !x.startsWith('-'))
  const prog = (w[0] ?? '').toLowerCase()
  let words = [prog]
  if (['npm', 'pnpm', 'yarn'].includes(prog) && w[1] === 'run') words = [prog, 'run', w[2] ?? '']
  else if (['git', 'npm', 'pnpm', 'yarn', 'go', 'cargo', 'docker', 'gh'].includes(prog) && w[1]) words = [prog, w[1]]
  const flags = /\s(-g|--global)\b/.test(part) ? '--global' : /\s--force\b/.test(part) ? '--force' : undefined
  return { kind: 'command', words: words.filter(Boolean).join(' '), flags }
}

export function mockPreview(command: string): ClassPreview[] {
  return command.split(/\|\||&&|[|;]/).map(p => p.trim()).filter(p => p && !FILTERS.test(p)).map(part =>
    /\.\.\/|[A-Za-z]:[\\/]/.test(part) ? { part, fixed: 'context' as const }
      : /\s-e\s|\s-c\s/.test(part) ? { part, fixed: 'inline' as const }
        : { part, class: mockClassOf(part) })
}

// mockExplain: the real backend's explanation, cut down to a few programs,
// the session type's list, the safe list and the setting for what it
// can't place.
export function mockExplain(s: SessionView, command: string, list: SafeEntry[], allowlist: string[], setting: UnknownCommands): Explanation {
  const mode = s.mode || 'unsafe'
  const parts = command.split(/\|\||&&|[|;]/).map(p => p.trim()).filter(p => p && !FILTERS.test(p))
  const why: TraceReason[] = parts.map(part => {
    const cls = mockClassOf(part)
    const entry = entryFor(list, cls, s.workdir)
    if (entry?.verdict === 'blocked') return { part, by: 'blocked', entry, class: cls }
    if (LOOKS.test(part)) return { part, by: 'looks' }
    if (mode === 'never') return { part, by: 'never', class: cls }
    if (mode === 'always') return { part, by: 'always', class: cls }
    if (entry) return { part, by: entry.verdict === 'safe' ? 'listed' : 'unsafe', entry, class: cls }
    const risky = RISKY.find(([re]) => re.test(part))
    if (risky) return { part, by: 'unsafe', risk: risky[1], class: cls }
    const allow = allowlist.find(a => {
      const m = /^Bash\((.+?)(:\*)?\)$/.exec(a)
      return !!m && (m[2] ? part.startsWith(m[1] as string) : part === m[1])
    })
    if (allow) return { part, by: 'builtin', allow, class: cls }
    if (ROUTINE.test(part)) return { part, by: 'safe', class: cls }
    if (setting === 'inside') return { part, by: 'inside', class: cls }
    if (setting === 'model') return { part, by: 'safe', judged: 'haiku', note: 'Runs one of the project’s own tools.', class: cls }
    return { part, by: 'unknown', class: cls }
  })
  const by = new Set(why.map(w => w.by))
  const action = by.has('blocked') ? 'deny' : ['always', 'unsafe', 'unknown'].some(b => by.has(b as TraceReason['by'])) ? 'ask' : 'allow'
  return { action, why }
}
