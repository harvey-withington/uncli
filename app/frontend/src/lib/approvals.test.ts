import { describe, expect, it } from 'vitest'
import type { Approval, TraceReason } from './api'
import { MODES, allowLabel, approvedLabel, askingReasons, classLabel, commandLines, describeApproval, fixedReason, mcpName, modeOf, promptable, reasonLabel, reasonTone, ruleLabel, scopeLabel, shortCommand } from './approvals'

const approval = (tool: string, input: unknown, description = ''): Approval => ({ requestId: 'r', tool, input, description, learn: [], askedAt: 0 })

describe('a command coloured by what UNCLI makes of it', () => {
  const cmd = 'cd "C:/code" && git add src && git commit -q -m "fix the selection count, and more besides\n- details" && git log -2 | head'
  const why: TraceReason[] = [
    { part: 'cd "C:/code"', by: 'looks', class: { kind: 'command', words: 'cd' } },
    { part: 'git add src', by: 'safe', class: { kind: 'command', words: 'git add' } },
    { part: 'git commit -q -m "fix the selection count, and more besides\n- details"', by: 'unsafe', class: { kind: 'command', words: 'git commit' } },
    { part: 'git log -2', by: 'looks', class: { kind: 'command', words: 'git log' } },
  ]
  const show = (lines: ReturnType<typeof commandLines>) =>
    lines.map(l => l.map(s => (s.kind === 'class' ? `[${s.text}:${s.tone}]` : s.kind === 'sep' ? `<${s.text.trim()}>` : s.text)).join(''))

  it('puts each part on its own line, its class words in its tone and the rest quiet', () => {
    expect(show(commandLines(cmd, why, true))).toEqual([
      '[cd:read] "C:/code" <&&>',
      '[git:ran] [add:ran] src <&&>',
      '[git:prompt] [commit:prompt] -q -m "fix the selection count, and…" <&&>',
      '[git:read] [log:read] -2 <|>',
      'head',
    ])
  })

  it('shows long quoted text in full when asked, and leaves a part it can\'t find plain', () => {
    expect(show(commandLines(cmd, why, false))[2]).toContain('and more besides')
    expect(show(commandLines('echo $(date) && rm -rf x', [{ part: 'rm -rf x', by: 'unsafe', class: { kind: 'command', words: 'rm x', flags: '-f -r' } }], false)))
      .toEqual(['echo $(date) <&&>', '[rm:prompt] -rf [x:prompt]'])
  })
})

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

describe('the safe list in plain words', () => {
  it('names classes and scopes', () => {
    expect(classLabel({ kind: 'command', words: 'npm run test' })).toBe('npm run test')
    expect(classLabel({ kind: 'command', words: 'git push', flags: '--force' })).toBe('git push --force')
    expect(classLabel({ kind: 'command', words: 'git:local' })).toMatch(/^Local git changes/)
    expect(classLabel({ kind: 'tool', words: 'mcp__notes__touch_note' })).toBe('touch_note (notes)')
    expect(classLabel({ kind: 'tool', words: 'WebFetch' })).toBe('WebFetch')
    expect(scopeLabel({ folder: 'C:\p' })).toBe('This project')
    expect(scopeLabel({})).toBe('All projects')
  })

  it('reads a missing level as When unsafe', () => {
    expect(modeOf({})).toBe('unsafe')
    expect(modeOf({ mode: '' })).toBe('unsafe')
    expect(modeOf({ mode: 'always' })).toBe('always')
    expect(MODES.map(m => m.id)).toEqual(['always', 'unsafe', 'never'])
  })

  it('offers "This should prompt" only for parts UNCLI judged safe', () => {
    const go = { kind: 'command' as const, words: 'go vet' }
    expect(promptable([{ part: 'ls', by: 'looks' }, { part: 'go vet ./...', by: 'safe', class: go }, { part: 'go vet .', by: 'safe', class: go }])).toEqual([go])
    expect(promptable([{ part: 'git push', by: 'listed', class: { kind: 'command', words: 'git push' } }])).toEqual([])
  })

  it('says why a card can only be allowed once', () => {
    const a = { ...approval('Bash', { command: 'node -e x' }), why: [{ part: 'node -e x', by: 'unsafe' as const, risk: 'runs-code' as const, fixed: 'inline' as const }] }
    expect(fixedReason(a)).toMatch(/^It runs code it was handed/)
    expect(fixedReason(approval('Bash', { command: 'ls' }))).toBeUndefined()
  })
})

describe('why a tool use ran', () => {
  it('reads session-type allowlist entries', () => {
    expect(allowLabel('Bash(npm test:*)')).toBe('npm test …')
    expect(allowLabel('Bash(npm test)')).toBe('npm test')
    expect(allowLabel('Write(./artifacts/**)')).toBe('Write in ./artifacts/**')
    expect(allowLabel('Read')).toBe('Read')
  })

  it('says why each part ran', () => {
    expect(reasonLabel({ part: 'git status', by: 'rule', rule: { tool: 'Bash', prefix: 'git:read', action: 'allow' } })).toMatch(/^your project rule: Read-only git/)
    expect(reasonLabel({ part: 'npm test', by: 'profile', allow: 'Bash(npm test:*)' })).toBe('the session type allows npm test …')
    expect(reasonLabel({ part: 'ls', by: 'looks' })).toBe('only reads, so it runs')
    expect(reasonLabel({ part: 'git push', by: 'listed', entry: { kind: 'command', words: 'git push', verdict: 'safe' } })).toBe('on your safe list (All projects)')
    expect(reasonLabel({ part: 'rm -r x', by: 'blocked', entry: { kind: 'command', words: 'rm', flags: '-r', verdict: 'blocked', folder: 'p' } })).toBe('blocked by you (This project)')
    expect(reasonLabel({ part: 'npm publish', by: 'unsafe', risk: 'publishes' })).toBe('unsafe: it sends or publishes something beyond this computer')
    expect(reasonLabel({ part: 'npm publish', by: 'none' })).toBe('nothing allows it, so the assistant asks')
    expect(reasonLabel({ part: 'rm x', by: 'readonly' })).toBe('changes things: refused in Read-only')
  })

  it('names every reason on a mixed command', () => {
    const why: TraceReason[] = [
      { part: 'git status --short', by: 'rule', rule: { tool: 'Bash', prefix: 'git:read', action: 'allow' } },
      { part: 'npm test', by: 'profile', allow: 'Bash(npm test:*)' },
      { part: 'npm run lint', by: 'profile', allow: 'Bash(npm run:*)' },
    ]
    expect(approvedLabel('mixed', why)).toBe('ran: project rule, session type')
    expect(approvedLabel('mixed', [{ part: 'ls', by: 'looks' }, { part: 'go vet', by: 'safe' }, { part: 'git push', by: 'listed' }])).toBe('ran: only reads, safe, your safe list')
    expect(approvedLabel('safe')).toBe('ran: safe')
    expect(approvedLabel('rule')).toBe('allowed by a project rule')
    expect(approvedLabel('profile')).toBe('allowed by the session type')
  })
})

describe('why a card is asking', () => {
  it('lists only the parts that need the user, in plain words', () => {
    const a: Approval = {
      ...approval('PowerShell', { command: 'npm test; git push; frob' }),
      why: [
        { part: 'npm test', by: 'safe' },
        { part: 'git push', by: 'unsafe', risk: 'publishes' },
        { part: 'frob', by: 'unknown' },
        { part: 'wipe', by: 'unsafe', risk: 'deletes', judged: 'haiku', note: 'Deletes the build cache.' },
        { part: 'deploy', by: 'unsafe', entry: { kind: 'command', words: 'deploy', verdict: 'unsafe', folder: 'p' } },
      ],
    }
    expect(askingReasons(a)).toEqual([
      { parts: ['git push'], text: 'Sends or publishes something beyond this computer.' },
      { parts: ['frob'], text: "UNCLI can't tell whether this is safe, and your setting is to prompt you about those." },
      { parts: ['wipe'], text: 'Deletes the build cache.' },
      { parts: ['deploy'], text: 'You marked this unsafe (This project).' },
    ])
    const always: Approval = { ...a, why: [{ part: 'npm test', by: 'always' }, { part: 'git push', by: 'always' }] }
    expect(askingReasons(always)).toEqual([{ parts: [], text: 'You asked to be prompted before anything except reading.' }])
  })

  it('names parts by their short form and groups those with the same reason', () => {
    const entry = { kind: 'command' as const, words: 'git', verdict: 'unsafe' as const }
    const commit = 'git commit -q -m "fix cell selection count, Escape/deselectAll range clearing\n\n- add SelectionModel.getSelectedCellCount"'
    const a: Approval = {
      ...approval('Bash', { command: `git add src && ${commit}` }),
      why: [
        { part: 'git add src', by: 'unsafe', entry, class: { kind: 'command', words: 'git add' } },
        { part: commit, by: 'unsafe', entry, class: { kind: 'command', words: 'git commit' } },
        { part: 'git log --oneline -2', by: 'looks' },
      ],
    }
    expect(askingReasons(a)).toEqual([{ parts: ['git add', 'git commit'], text: 'You marked this unsafe (All projects).' }])
    // Without a class, the part itself, with the long quoted text shortened.
    expect(askingReasons({ ...a, why: [{ part: commit, by: 'unknown' }] })[0]?.parts).toEqual(['git commit -q -m "fix cell selection count,…"'])
  })

  it('shortens long quoted text in a command, keeping what it does', () => {
    expect(shortCommand('git commit -m "wip"')).toBe('git commit -m "wip"')
    expect(shortCommand('cd app && git commit -q -m "fix cell selection count, Escape range clearing\n- more" && git log -2'))
      .toBe('cd app && git commit -q -m "fix cell selection count,…" && git log -2')
    expect(shortCommand("node -e 'line one\nline two'")).toBe("node -e 'line one…'")
  })

  it('adds the decision model\'s reason to a judged part', () => {
    expect(reasonLabel({ by: 'routine', judged: 'haiku', note: 'Rebuilds the docs.' })).toBe('routine work in this folder: Rebuilds the docs. (judged by haiku)')
  })
})

describe('reason tones and named entries', () => {
  it('colours reasons by what happened, the judged ones apart', () => {
    expect(reasonTone({ part: 'ls', by: 'looks' })).toBe('read')
    expect(reasonTone({ part: 'go vet', by: 'safe' })).toBe('ran')
    expect(reasonTone({ part: 'docgen', by: 'safe', judged: 'haiku', note: 'x' })).toBe('judged')
    expect(reasonTone({ part: 'wipe', by: 'unsafe', judged: 'haiku', note: 'x' })).toBe('judged')
    expect(reasonTone({ part: 'git push', by: 'unsafe' })).toBe('prompt')
    expect(reasonTone({ part: 'rm -r', by: 'blocked' })).toBe('blocked')
  })

  it('names your own entries by their label', () => {
    const entry = { kind: 'command' as const, words: 'npm run ship', verdict: 'safe' as const, folder: 'p', label: 'Deploys the site' }
    expect(reasonLabel({ part: 'npm run ship', by: 'listed', entry })).toBe('on your safe list as “Deploys the site” (This project)')
    expect(reasonLabel({ part: 'npm run ship', by: 'blocked', entry: { ...entry, verdict: 'blocked' } })).toBe('blocked by you: “Deploys the site” (This project)')
    expect(reasonLabel({ part: 'npm run ship', by: 'listed', entry: { ...entry, label: undefined } })).toBe('on your safe list (This project)')
  })
})
