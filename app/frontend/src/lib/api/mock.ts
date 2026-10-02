// A backend for running the UI in a plain browser (vite dev, screenshots,
// component tests). It behaves like the real one closely enough to
// exercise streaming, states, bookmarks and errors, with canned content.
import type {
  ActivityState, Backend, Bootstrap, CLIStatus, Handlers, NewSessionChoices, Page, Preferences, SessionView, UEvent,
} from './types'
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

export function mockBackend(opts: { cli?: Partial<CLIStatus>; empty?: boolean; lastNew?: Partial<NewSessionChoices>; prefs?: Partial<Preferences> } = {}): Backend {
  const sessions: SessionView[] = opts.empty ? [] : [
    session('s-code', 'code', 'Fix the flaky parser test', 'idle', { model: 'opus', modifiers: ['thorough'], sortOrder: 3 }),
    session('s-chat', 'chat', 'Plan a weekend in Lisbon', 'unread', { sortOrder: 2 }),
    session('s-cowork', 'cowork', 'Summarise the Q3 planning notes', 'idle', { modifiers: ['efficiency'], sortOrder: 1 }),
  ]
  const pages: Record<string, Page[]> = opts.empty ? {} : {
    's-code': [
      page('s-code', 1, 'Why does TestMultiTurnPartial fail about one run in five?', PARSER, {
        model: 'opus', modifiers: ['thorough'], bookmarked: true,
        trace: [
          { id: 't1', name: 'Read', summary: 'Read parser.go', done: true, ok: true },
          { id: 't2', name: 'Bash', summary: 'Bash go test -count=5 ./internal/adapter/...', done: true, ok: true },
          { id: 't3', name: 'Edit', summary: 'Edit parser.go', done: true, ok: true },
          { id: 't4', name: 'Bash', summary: 'Bash rm -rf testdata/tmp', done: true, ok: false, denied: true, output: 'Permission for this tool use was denied.' },
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
      const answer = text.startsWith('/') ? `Ran \`${text}\`.` : STREAM
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
