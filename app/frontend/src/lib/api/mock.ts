// A backend for running the UI in a plain browser (vite dev, screenshots,
// component tests). It behaves like the real one closely enough to
// exercise streaming, states, bookmarks and errors, with canned content.
import type {
  ActivityState, Approval, Backend, Bootstrap, CLIStatus, Handlers, NewSessionChoices, Page, Preferences, SearchHit, SearchQuery, SearchResult, SafeEntry, Scope, SessionMode, SessionView, UEvent,
} from './types'
import { MOCK_ALLOWLISTS, MOCK_ASKS, mockExplain, mockJudge, mockPreview, type MockAsk, type Outcome } from './mock-access'
import { SECTION_KINDS } from '../sections'

const profiles: Bootstrap['profiles'] = [
  { id: 'chat', label: 'Chat', icon: 'message-circle', hue: 205, folder: 'scratch', model: 'sonnet', tools: ['WebSearch', 'WebFetch', 'Write'], modifiersOn: [], ideLinks: false },
  { id: 'cowork', label: 'Co-work', icon: 'briefcase', hue: 38, folder: 'pick', model: 'sonnet', tools: null, modifiersOn: null, ideLinks: false },
  { id: 'code', label: 'Code', icon: 'code', hue: 280, folder: 'repo', model: 'opus', tools: [], modifiersOn: null, ideLinks: true },
]

const modifiers: Bootstrap['modifiers'] = [
  { id: 'use-agents', label: 'Use Agents', icon: 'users', scope: 'turn', requiresTools: ['Task'] },
  { id: 'efficiency', label: 'Efficiency Mode', icon: 'gauge', scope: 'turn', group: 'verbosity' },
  { id: 'thorough', label: 'Thorough', icon: 'microscope', scope: 'turn', group: 'verbosity', settings: { effort: 'high' } },
]

const toolbar: Bootstrap['toolbar'] = [
  { kind: 'model_picker' },
  { kind: 'modifiers' },
  { kind: 'separator' },
  { kind: 'slash', label: 'Compact', icon: 'shrink', command: '/compact' },
  { kind: 'slash', label: 'Context', icon: 'gauge', command: '/context' },
  { kind: 'native', label: 'Usage', icon: 'bar-chart', action: 'usage' },
  { kind: 'native', label: 'Open folder', icon: 'folder-open', action: 'open_workdir' },
]

const models: Bootstrap['models'] = [
  { value: 'opus', resolved: 'claude-opus-5-5', displayName: 'Opus 5.5', description: 'For complex work and everyday tasks', effortLevels: ['low', 'medium', 'high', 'xhigh', 'max'] },
  { value: 'sonnet', resolved: 'claude-sonnet-5-5', displayName: 'Sonnet 5.5', description: 'Most efficient for simpler tasks', effortLevels: ['low', 'medium', 'high', 'xhigh', 'max'] },
  { value: 'haiku', resolved: 'claude-haiku-4-5-20251001', displayName: 'Haiku 4.5', description: 'Fastest for quick answers', effortLevels: null },
]

const LISBON = `Here's a relaxed two-day plan that keeps walking to a sensible amount.

## Saturday

1. **Alfama** in the morning, before the tour groups. Start at the *Miradouro de Santa Luzia*.
2. Lunch at a *tasca* — grilled sardines if they're in season.
3. Tram 28 is crowded; walk down to **Baixa** instead.

## Sunday

| Time | Where | Why |
| --- | --- | --- |
| 10:00 | Belém | Pastéis de nata at the source |
| 13:00 | LX Factory | Lunch and bookshops |
| 17:00 | Cais do Sodré | Sunset by the river |

> Book the Jerónimos Monastery ahead; the queue is long after 11.

A rough budget per person:

\`\`\`text
Food        €70
Transport   €15
Entry fees  €25
\`\`\`
`

const PARSER = `The test was flaky because the parser reset its error state on every \`system/status\` line, and that line repeats for **each API call** inside a turn.

\`\`\`go
case "status":
	switch {
	case l.Status != nil && *l.Status == "requesting": // repeats per API call within a turn
		return []core.Event{ev(core.EvTurnStarted, nil)}, nil
	}
\`\`\`

Now only the \`result\` line resets turn state. I re-ran the suite three times:

- \`go test -count=3 ./internal/...\` passes
- no data races in the session tests`

const STREAM = `Sure. In short: **stream-json** keeps one CLI process alive and sends one JSON line per turn.

- Each answer streams as text deltas.
- A \`result\` line closes the turn with usage and cost.

\`\`\`ts
const turn = { type: 'user', message: { role: 'user', content: text } }
stdin.write(JSON.stringify(turn) + '\\n')
\`\`\`

That's all the adapter needs to get started.`

let idn = 100
const newId = () => `mock-${++idn}`

// A small stand-in for the store's full-text search: every word must
// appear (the last one as a prefix while typing, "quoted" as a phrase),
// ignoring case and accents; the snippet marks the first match.
function mockSearch(q: SearchQuery, sessions: SessionView[], pages: Record<string, Page[]>): SearchResult {
  const fold = (s: string) => s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()
  const terms: { t: string; prefix: boolean }[] = []
  let rest = q.text.replace(/"([^"]*)"/g, (_, p: string) => {
    if (p.trim()) terms.push({ t: fold(p.trim()), prefix: false })
    return ' '
  })
  rest = rest.replace(/"/g, ' ')
  const words = rest.split(/[^\p{L}\p{N}*]+/u).filter(Boolean)
  words.forEach((w, i) => terms.push({ t: fold(w.replace(/\*+$/, '')), prefix: w.endsWith('*') || (i === words.length - 1 && !/\s$/.test(q.text)) }))
  const live = terms.filter(x => x.t)
  if (live.length === 0) return { hits: [], terms: [] }
  const has = (text: string, x: { t: string; prefix: boolean }) => {
    const f = fold(text)
    if (x.t.includes(' ')) return f.includes(x.t)
    // Terms are letters and digits only, so they need no escaping.
    const re = new RegExp(`(^|[^\\p{L}\\p{N}])${x.t}${x.prefix ? '' : '(?![\\p{L}\\p{N}])'}`, 'u')
    return re.test(f)
  }
  const hits: SearchHit[] = []
  for (const s of sessions) {
    if (q.sessionId && s.id !== q.sessionId) continue
    if (q.profiles?.length && !q.profiles.includes(s.profileId)) continue
    for (const p of pages[s.id] ?? []) {
      if (q.bookmarked && !p.bookmarked) continue
      if (q.since && p.startedAt < q.since) continue
      if (q.until && p.startedAt >= q.until) continue
      const extra = (p.attachments ?? []).map(a => a.name).join(' ') + ' ' + (p.outline?.sections ?? []).map(x => x.title).join(' ')
      const all = [s.title, p.question, p.answerMd, extra].join('\n')
      if (!live.every(x => has(all, x))) continue
      const fields: [SearchHit['field'], string][] = [['answer', p.answerMd], ['question', p.question], ['extra', extra], ['title', s.title]]
      const [field, text] = fields.find(([, t]) => live.some(x => has(t, x))) ?? ['answer', p.answerMd]
      const f = fold(text)
      const at = Math.max(0, Math.min(...live.map(x => f.indexOf(x.t)).filter(i => i >= 0)))
      const from = Math.max(0, at - 40)
      let snip = text.slice(from, at + 80)
      for (const x of live) {
        const i = fold(snip).indexOf(x.t)
        if (i >= 0) {
          const end = x.prefix ? i + x.t.length + (fold(snip.slice(i + x.t.length)).match(/^[\p{L}\p{N}]*/u)?.[0].length ?? 0) : i + x.t.length
          snip = snip.slice(0, i) + '\x01' + snip.slice(i, end) + '\x02' + snip.slice(end)
        }
      }
      hits.push({ pageId: p.id, sessionId: s.id, sessionTitle: s.title, profileId: s.profileId, seq: p.seq, question: p.question, field, snippet: (from > 0 ? '…' : '') + snip + '…', bookmarked: p.bookmarked, startedAt: p.startedAt })
    }
  }
  hits.sort((a, b) => b.startedAt - a.startedAt)
  return { hits: hits.slice(0, q.limit || 60), terms: live.map(x => x.t) }
}

// The media type a file would attach as, judged by its extension (mock only).
function mockMedia(name: string): string {
  const ext = name.toLowerCase().replace(/^.*\./, '')
  if (['png', 'jpg', 'jpeg', 'gif', 'webp'].includes(ext)) return ext === 'jpg' ? 'image/jpeg' : `image/${ext}`
  if (ext === 'pdf') return 'application/pdf'
  if (['txt', 'md', 'go', 'ts', 'js', 'json', 'yaml', 'yml', 'csv', 'py', 'svelte', 'html', 'css'].includes(ext)) return 'text/plain'
  return ''
}

function page(sessionId: string, seq: number, question: string, answerMd: string, extra: Partial<Page> = {}): Page {
  return {
    id: `${sessionId}-p${seq}`, sessionId, seq, question, model: 'sonnet', modifiers: [], answerMd,
    trace: [], touchedFiles: [], status: 'done', bookmarked: false, pinned: false,
    inputTokens: 12, outputTokens: 640, cacheRead: 21500, cacheWrite: 1200, costUsd: 0.0182,
    durationMs: 8400, startedAt: Date.now() - 600000, finishedAt: Date.now() - 590000, ...extra,
  }
}

function session(id: string, profileId: string, title: string, state: ActivityState, extra: Partial<SessionView> = {}): SessionView {
  return {
    id, title, adapter: 'claude', runtime: 'local', profileId, workdir: `C:\\Users\\you\\${profileId}`,
    providerSid: '', cliVersion: '2.1.285', model: profiles.find(p => p.id === profileId)?.model ?? 'sonnet',
    modifiers: [], sortOrder: 0, archived: false, createdAt: Date.now(), updatedAt: Date.now(),
    state, running: state !== 'idle', busy: false, ...extra,
  }
}

export function mockBackend(opts: { cli?: Partial<CLIStatus>; empty?: boolean; unattended?: boolean; mode?: SessionMode; lastNew?: Partial<NewSessionChoices>; prefs?: Partial<Preferences> } = {}): Backend {
  const sessions: SessionView[] = opts.empty ? [] : [
    session('s-code', 'code', 'Fix the flaky parser test', 'idle', { model: 'opus', modifiers: ['thorough'], sortOrder: 3, unattended: opts.unattended, mode: opts.mode }),
    session('s-chat', 'chat', 'Plan a weekend in Lisbon', 'unread', { sortOrder: 2 }),
    session('s-cowork', 'cowork', 'Summarise the Q3 planning notes', 'idle', { modifiers: ['efficiency'], sortOrder: 1 }),
  ]
  const pages: Record<string, Page[]> = opts.empty ? {} : {
    's-code': [
      page('s-code', 1, 'Why does TestMultiTurnPartial fail about one run in five?', PARSER, {
        model: 'opus', modifiers: ['thorough'], bookmarked: true,
        trace: [
          { id: 't1', name: 'Read', summary: 'Read parser.go', done: true, ok: true },
          {
            id: 't2', name: 'PowerShell', summary: 'PowerShell git status --short; go vet ./internal/adapter/...', done: true, ok: true, approved: 'safe',
            why: [
              { part: 'git status --short', by: 'looks' },
              { part: 'go vet ./internal/adapter/...', by: 'safe', class: { kind: 'command', words: 'go vet' } },
            ],
          },
          { id: 't3', name: 'Edit', summary: 'Edit parser.go', done: true, ok: true },
          {
            id: 't4', name: 'Bash', summary: 'Bash rm -rf testdata/fixtures', done: true, ok: false, denied: true, output: 'Permission for this tool use was denied.',
            why: [{ part: 'rm -rf testdata/fixtures', by: 'unsafe', risk: 'deletes', class: { kind: 'command', words: 'rm testdata/fixtures', flags: '-f -r' } }],
          },
          {
            id: 't5', name: 'PowerShell', summary: 'PowerShell npm test 2>&1 | Select-Object -Last 40', done: true, ok: false, approved: 'you',
            output: "Exit code 1\n> grid@0.1.0 test\r\n> vitest run\r\n\r\nnode.exe : 'vitest' is not recognized as an internal or external command,\r\noperable program or batch file.",
          },
        ],
      }),
    ],
    's-chat': [
      page('s-chat', 1, 'Plan a relaxed weekend in Lisbon for two, mostly on foot.', LISBON),
    ],
    's-cowork': [
      page('s-cowork', 1, 'What are the three decisions in these notes?', 'The notes record three decisions:\n\n1. Ship the **desktop app first**; mobile waits.\n2. Keep pricing flat for the first year.\n3. Hire a designer before the public beta.', { bookmarked: true }),
      page('s-cowork', 2, 'Who owns each one?', '- Desktop first: **Priya**\n- Pricing: **Tom**\n- Designer hire: still open', { modifiers: ['efficiency'] }),
      page('s-cowork', 3, 'Draft a two-line update for the team.', 'We\'re shipping desktop first, keeping pricing flat for a year, and hiring a designer before the beta. Owners are in the planning doc.', { modifiers: ['efficiency'] }),
    ],
  }
  let cli: CLIStatus = { installed: true, version: '2.1.285', pinned: '2.1.285', custom: false, loggedIn: true, email: 'you@example.com', subscription: 'max', ...opts.cli }
  let h: Handlers | null = null
  const lastNew: NewSessionChoices = { models: {}, folders: {}, ...opts.lastNew }
  let prefs: Preferences = { quickTaskModel: { provider: 'claude', model: 'haiku' }, autoSummary: 'off', ...opts.prefs }
  const timers = new Map<string, number[]>()
  // Approvals: a question that mentions "push" asks to run git push first,
  // one that mentions a "note" to save it with an MCP tool (mock-access.ts).
  const safe: SafeEntry[] = [] // the safe list, every project's
  const judged = new Set<string>() // tools the quick-task model has looked at
  const approvalWaits = new Map<string, (o: Outcome) => void>()
  let requestSeq = 0

  const listFor = (s: SessionView) => safe.filter(e => !e.folder || e.folder === s.workdir)
  const judgeAsk = (s: SessionView, ask: MockAsk) => mockJudge(s, ask, listFor(s), prefs.unknownCommands ?? 'model', judged.has(ask.tool))
  const sameClass = (a: SafeEntry, b: SafeEntry) => a.kind === b.kind && a.words === b.words && (a.flags ?? '') === (b.flags ?? '')
  // put adds or replaces an entry at a scope; all projects absorbs the
  // project entries for the same class.
  const put = (s: SessionView, e: SafeEntry, scope: Scope) => {
    const folder = scope === 'all' ? undefined : s.workdir
    for (let i = safe.length - 1; i >= 0; i--) {
      const x = safe[i] as SafeEntry
      if (sameClass(x, e) && (x.folder === folder || scope === 'all')) safe.splice(i, 1)
    }
    safe.push({ kind: e.kind, words: e.words, flags: e.flags || undefined, verdict: e.verdict, folder })
  }
  const drop = (e: SafeEntry) => {
    const i = safe.findIndex(x => sameClass(x, e) && x.folder === e.folder)
    if (i >= 0) safe.splice(i, 1)
  }
  // Look again at waiting cards after the mode or the safe list changed.
  const rejudge = (s: SessionView) => {
    const waiting = s.approvals ?? []
    const still = waiting.filter(a => {
      const ask = MOCK_ASKS.find(x => x.tool === a.tool)
      if (!ask) return true
      const v = judgeAsk(s, ask)
      if (v.action === 'ask' || v.action === 'judge') {
        a.why = v.why
        return true
      }
      approvalWaits.get(a.requestId)?.(v.action === 'allow' ? 'allow' : 'deny')
      return false
    })
    s.approvals = still
    if (still.length === 0 && waiting.length > 0) setState(s, 'running_tools')
    else changed(s)
  }
  const rejudgeAll = () => sessions.forEach(rejudge)

  const changed = (s: SessionView) => h?.sessionChanged({ ...s })
  const setState = (s: SessionView, state: ActivityState) => {
    s.state = state
    changed(s)
  }
  const emit = (sessionId: string, kind: UEvent['kind'], turnSeq: number, data?: unknown) =>
    h?.sessionEvent({ sessionId, event: { kind, turnSeq, at: new Date().toISOString(), data } })
  const find = (id: string) => {
    const s = sessions.find(x => x.id === id)
    if (!s) throw new Error(`no session ${id}`)
    return s
  }

  return {
    kind: 'mock',
    subscribe(handlers) {
      h = handlers
      return () => { h = null }
    },
    async bootstrap() {
      return {
        profiles, modifiers, toolbar, models, platform: 'windows', lastNewSession: structuredClone(lastNew),
        preferences: structuredClone(prefs), providers: [{ id: 'claude', label: 'Claude' }],
        sessions: sessions.map(s => ({ ...s })),
        capabilities: {
          partialStreaming: true, resume: true, liveModelSwitch: true, interrupt: true, approvals: true,
          images: true, usageReporting: true, thinkingEvents: true, slashPassthrough: true,
        },
      }
    },
    async cliStatus() { return cli },
    async installCLI() {
      const total = 243751072
      for (let done = 0; done <= total; done += total / 20) {
        h?.cliProgress({ done, total })
        await new Promise(r => setTimeout(r, 60))
      }
      cli = { ...cli, installed: true }
      h?.cliStatus(cli)
      return cli
    },
    async signIn() {
      return { url: 'https://claude.com/cai/oauth/authorize?code=true&client_id=mock', signedIn: false, status: cli }
    },
    async submitLoginCode(code) {
      await new Promise(r => setTimeout(r, 300))
      if (code.trim() !== 'good-code') throw new Error('Invalid code. Please make sure the full code was copied.')
      cli = { ...cli, loggedIn: true }
      h?.cliStatus(cli)
      return cli
    },
    async cancelSignIn() {},
    async setCLIVersion(v) { cli = { ...cli, version: v || cli.pinned }; return cli },
    async cliChannels() { return { stable: '2.1.285', latest: '2.1.286' } },
    async pickFolder() { return 'C:\\Users\\you\\projects\\demo' },
    async createSession(profileId, workdir, model) {
      const p = profiles.find(x => x.id === profileId)
      const s = session(newId(), profileId, '', 'idle', {
        model: model || p?.model || 'sonnet', workdir: workdir || `C:\\scratch\\${idn}`, sortOrder: sessions.length + 10,
      })
      sessions.unshift(s)
      pages[s.id] = []
      changed(s)
      lastNew.profileId = profileId
      lastNew.models[profileId] = s.model
      if (p?.folder !== 'scratch') lastNew.folders[profileId] = s.workdir
      return { session: { ...s }, lastNewSession: structuredClone(lastNew) }
    },
    async pages(id) { return (pages[id] ?? []).map(p => ({ ...p })) },
    async send(id, text, attachments = []) {
      const s = find(id)
      if (s.busy) throw new Error('this session is still answering; wait or stop it first')
      const list = (pages[id] ??= [])
      const attached = attachments.map(a => {
        const name = a.name || (a.path ?? '').replace(/^.*[\\/]/, '')
        return { name, path: a.path, mediaType: a.mediaType || mockMedia(name), size: a.data ? Math.round((a.data.length * 3) / 4) : 2048 }
      })
      const p = page(id, list.length + 1, text, '', {
        model: s.model, modifiers: [...s.modifiers], status: 'open', outputTokens: 0, costUsd: 0, durationMs: 0, attachments: attached,
      })
      list.push(p)
      if (!s.title) s.title = (text || attached[0]?.name || '').slice(0, 60)
      s.busy = true
      s.running = true
      h?.pageChanged({ ...p })
      setState(s, 'thinking')
      // Like the real backend, send returns once the turn has started; the
      // rest (an approval, then the streamed answer) runs on its own.
      const run = async () => {
        let answer = text.startsWith('/') ? `Ran \`${text}\`.` : STREAM
        const ask = MOCK_ASKS.find(a => a.match.test(text))
        if (ask) {
          let v = judgeAsk(s, ask)
          let outcome: Outcome | undefined
          const requestId = `req_${++requestSeq}`
          const approval: Approval = {
            requestId, tool: ask.tool, input: ask.input, toolUseId: `toolu_${requestSeq}`,
            learn: [ask.learn], class: ask.class, why: v.why, askedAt: Date.now(),
          }
          if (v.action === 'judge') {
            // The quick-task model checks a tool UNCLI can't place: the
            // request waits without a card meanwhile.
            s.approvals = [...(s.approvals ?? []), { ...approval, judging: true }]
            changed(s)
            await new Promise(r => setTimeout(r, 600))
            judged.add(ask.tool)
            s.approvals = (s.approvals ?? []).filter(a => a.requestId !== requestId)
            v = judgeAsk(s, ask)
            approval.why = v.why
          }
          if (v.action === 'allow') outcome = 'allow'
          else if (v.action === 'deny') outcome = 'deny'
          else if (s.unattended) outcome = 'away' // declined at once, as the real backend does
          else {
            s.approvals = [...(s.approvals ?? []), approval]
            setState(s, 'needs_approval')
            outcome = await new Promise<Outcome>(resolve => approvalWaits.set(requestId, resolve))
          }
          answer = ask.answers[outcome ?? 'deny']
        }
        const words = answer.split(/(?<=\s)/)
        const ts: number[] = []
        let at = 700
        words.forEach((wd, i) => {
          ts.push(window.setTimeout(() => {
            if (i === 0) setState(s, 'writing')
            emit(id, 'text_delta', p.seq, { index: 1, text: wd })
          }, at))
          at += 35
        })
        ts.push(window.setTimeout(() => {
          p.answerMd = answer
          p.status = 'done'
          p.outputTokens = 180
          p.costUsd = 0.0061
          p.durationMs = at
          p.finishedAt = Date.now()
          h?.pageChanged({ ...p })
          emit(id, 'text_block', p.seq, { text: answer })
          emit(id, 'turn_result', p.seq, {})
          s.busy = false
          setState(s, 'idle')
        }, at + 50))
        timers.set(id, ts)
      }
      void run()
    },
    async interrupt(id) {
      const s = find(id)
      timers.get(id)?.forEach(t => clearTimeout(t))
      const p = pages[id]?.at(-1)
      if (p && p.status === 'open') {
        p.status = 'interrupted'
        p.answerMd = p.answerMd || 'Sure. In short: **stream-json** keeps one CLI'
        h?.pageChanged({ ...p })
      }
      s.busy = false
      setState(s, 'idle')
    },
    async setModel(id, m) { const s = find(id); s.model = m; changed(s) },
    async toggleModifier(id, mod, on) {
      const s = find(id)
      const def = modifiers.find(x => x.id === mod)
      let next = s.modifiers.filter(x => x !== mod)
      if (on) {
        if (def?.group) next = next.filter(x => modifiers.find(y => y.id === x)?.group !== def.group)
        next.push(mod)
      }
      s.modifiers = modifiers.map(x => x.id).filter(x => next.includes(x))
      changed(s)
      return { ...s }
    },
    async setSortOrder(id, o) { const s = find(id); s.sortOrder = o; changed(s); return { ...s } },
    async answerApproval(sid, rid, decision, scope) {
      const s = find(sid)
      const a = (s.approvals ?? []).find(x => x.requestId === rid)
      if (!a) throw new Error('that request is no longer waiting for an answer')
      if (decision === 'safe') a.learn.forEach(c => put(s, { ...c, verdict: 'safe' }, scope ?? 'project'))
      s.approvals = (s.approvals ?? []).filter(x => x.requestId !== rid)
      if (s.approvals.length === 0) setState(s, 'running_tools')
      else changed(s)
      approvalWaits.get(rid)?.(decision === 'deny' ? 'deny' : 'allow')
      if (decision === 'safe') rejudgeAll()
    },
    async setUnattended(sid, on) {
      const s = find(sid)
      s.unattended = on
      if (on) {
        // Whatever is waiting is declined, as the real backend does.
        const waiting = s.approvals ?? []
        s.approvals = []
        if (waiting.length) setState(s, 'running_tools')
        waiting.forEach(a => approvalWaits.get(a.requestId)?.('away'))
      }
      changed(s)
      return { ...s }
    },
    async setMode(sid, mode) {
      const s = find(sid)
      s.mode = mode
      rejudge(s)
      return { ...s }
    },
    async safeList(sid) { return listFor(find(sid)).map(e => ({ ...e })) },
    async setSafeEntry(sid, entry, scope) {
      const s = find(sid)
      put(s, entry, scope)
      rejudgeAll()
      return listFor(s).map(e => ({ ...e }))
    },
    async deleteSafeEntry(sid, entry) {
      drop(entry)
      rejudgeAll()
      return listFor(find(sid)).map(e => ({ ...e }))
    },
    async moveSafeEntry(sid, entry, scope) {
      const s = find(sid)
      drop(entry)
      put(s, entry, scope)
      rejudgeAll()
      return listFor(s).map(e => ({ ...e }))
    },
    async teach(sid, classes, verdict, scope) {
      const s = find(sid)
      classes.forEach(c => put(s, { ...c, verdict }, scope))
      rejudgeAll()
    },
    async previewClasses(_sid, command) { return mockPreview(command) },
    async knownTools() {
      return ['Bash', 'Edit', 'PowerShell', 'Read', 'WebFetch', 'WebSearch', 'Write', 'mcp__notes__read_note', 'mcp__notes__touch_note']
    },
    async explainCommand(sid, command) {
      const s = find(sid)
      return mockExplain(s, command, listFor(s), MOCK_ALLOWLISTS[s.profileId] ?? [], prefs.unknownCommands ?? 'model')
    },
    async sessionAllowlist(sid) { return [...(MOCK_ALLOWLISTS[find(sid).profileId] ?? [])] },
    async search(q) {
      return mockSearch(q, sessions, pages)
    },
    async describeAttachments(paths) {
      // By extension: images, PDFs and common text files attach; a path
      // without an extension reads as a folder; anything else can't.
      return paths.map(p => {
        const name = p.replace(/[\\/]+$/, '').replace(/^.*[\\/]/, '')
        const media = mockMedia(name)
        if (/[\\/]$/.test(p) || !/\.[^\\/]+$/.test(p)) return { path: p, name, size: 0, kind: 'folder' as const, reason: "folders can't be attached" }
        if (!media) return { path: p, name, size: 4096, kind: 'unsupported' as const, reason: 'only images, PDFs and text files can be attached' }
        const kind = media.startsWith('image/') ? 'image' as const : media === 'application/pdf' ? 'pdf' as const : 'text' as const
        return { path: p, name, size: 2048, kind, mediaType: media }
      })
    },
    async describePaths(paths) {
      // A path ending in a separator or without a dot reads as a folder.
      return paths.map(p => {
        const isDir = /[\\/]$/.test(p) || !/\.[^\\/]+$/.test(p)
        return { path: p, isDir, dir: isDir ? p : p.replace(/[\\/][^\\/]*$/, '') }
      })
    },
    async rename(id, t) { const s = find(id); s.title = t; changed(s); return { ...s } },
    async remove(id) {
      const i = sessions.findIndex(x => x.id === id)
      if (i >= 0) sessions.splice(i, 1)
    },
    async setBookmark(sid, pid, on) {
      const p = pages[sid]?.find(x => x.id === pid)
      if (!p) throw new Error('page not found')
      p.bookmarked = on
      h?.pageChanged({ ...p })
      return { ...p }
    },
    async focus(id) {
      const s = sessions.find(x => x.id === id)
      if (s && s.state === 'unread') setState(s, 'idle')
    },
    async setPreferences(p) {
      prefs = structuredClone(p)
      return structuredClone(prefs)
    },
    async summarisePage(sid, pid, blocks) {
      const p = pages[sid]?.find(x => x.id === pid)
      if (!p) throw new Error('page not found')
      await new Promise(r => setTimeout(r, 400))
      // A canned summary: one section per block, with varied kinds so every colour shows.
      const sections = blocks.map((b, i) => ({
        block: i,
        title: b.replace(/[#*`>|-]/g, '').trim().split(/\s+/).slice(0, 4).join(' ') || `Part ${i + 1}`,
        kind: SECTION_KINDS[(i * 3) % SECTION_KINDS.length],
      }))
      p.outline = { provider: prefs.quickTaskModel.provider, model: prefs.quickTaskModel.model, sections, costUsd: 0.0042, at: Date.now() }
      h?.pageChanged({ ...p })
      return { ...p }
    },
    async usage() {
      const now = Math.floor(Date.now() / 1000)
      return { status: 'allowed', windows: { five_hour: { utilization: 0.18, resetsAt: now + 7200 }, seven_day: { utilization: 0.31, resetsAt: now + 260000 } } }
    },
    async openFolder() {},
    async openURL(url) { window.open(url, '_blank', 'noopener') },
    async copyText(text) { await navigator.clipboard?.writeText(text) },
    async clipboardFiles() { return [] },
  }
}
