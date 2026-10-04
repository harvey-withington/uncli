// Types mirroring the Go JSON the bridge sends. The UI only ever sees
// UNCLI's own events and pages, never a provider's raw output.

export type ActivityState =
  | 'idle'
  | 'starting'
  | 'thinking'
  | 'writing'
  | 'running_tools'
  | 'needs_approval'
  | 'unread'
  | 'error'
  | 'exited'

export interface SessionView {
  id: string
  title: string
  adapter: string
  runtime: string
  profileId: string
  workdir: string
  providerSid: string
  cliVersion: string
  model: string
  modifiers: string[]
  sortOrder: number
  archived: boolean
  createdAt: number
  updatedAt: number
  state: ActivityState
  running: boolean
  busy: boolean
  error?: string
  approvals?: Approval[] // tool uses waiting for the user, oldest first
  unattended?: boolean // requests that would wait for the user are declined
  mode?: SessionMode | '' // when Claude stops to prompt; empty = when unsafe
}

// When Claude stops to prompt the user: always (anything but reading),
// unsafe (only what could do harm; the default) or never.
export type SessionMode = 'always' | 'unsafe' | 'never'

// What UNCLI takes a tool use to do, and on whose word.
export interface ToolClassView {
  write: boolean
  destructive?: boolean
  openWorld?: boolean
  source: 'you' | 'built-in' | 'server' | 'unknown'
}

export interface TraceItem {
  id: string
  name: string
  summary: string
  done: boolean
  ok: boolean
  denied?: boolean
  output?: string
  approved?: Approver // who let it run, when the CLI asked first
  why?: TraceReason[] // when UNCLI answered without the user: the reason for each part of the command
}

// Who let a tool use run: you (on its card), your safe list, the session
// type's list, UNCLI judging it safe (or reading), the setting for what it
// can't place, Run without prompting, or a mix. (Older traces also hold
// rule, session, profile, routine, full.)
export type Approver = 'you' | 'listed' | 'builtin' | 'safe' | 'looks' | 'inside' | 'never' | 'mixed'
  | 'rule' | 'session' | 'profile' | 'routine' | 'full'

// Why one part of a command (or a whole tool use) ran, prompted or was blocked.
export interface TraceReason {
  part?: string // the part of the command; absent for a whole tool use
  by:
    | 'looks' | 'safe' | 'listed' | 'builtin' | 'inside' | 'never' // it ran
    | 'always' | 'unsafe' | 'unknown' | 'judging' | 'user' // it prompted (judging: the model first)
    | 'blocked' // the user's safe list blocks it
    | 'rule' | 'session' | 'profile' | 'routine' | 'full' | 'deny' | 'readonly' | 'outside' | 'ask' | 'risky' | 'none' | 'complex' // older traces
  entry?: SafeEntry // the safe-list entry that decided it
  allow?: string // the session type's entry that matched, as written
  class?: SafeClass // what "This is safe" or "This should prompt" would remember
  fixed?: 'inline' | 'complex' | 'context' // why nothing can be remembered for it
  rule?: ToolRule // older traces: the rule that matched
  risk?: Risk // how an unsafe part could do harm
  judged?: string // the quick-task model that judged a part UNCLI didn't recognise
  note?: string // its one-line reason
}

// The safe list: the user's corrections to what UNCLI counts as safe.
export type Verdict = 'safe' | 'unsafe' | 'blocked'
export interface SafeClass {
  kind: 'command' | 'tool'
  words: string // "npm run test", a git class "git:local", or a tool's name
  flags?: string // risk flags that set the class apart, e.g. "--force"
}
export interface SafeEntry extends SafeClass {
  verdict: Verdict
  folder?: string // the project it belongs to; absent for all projects
}
// Where an entry applies: this session's project, or all projects.
export type Scope = 'project' | 'all'

// What an example command would be remembered as, part by part.
export interface ClassPreview {
  part: string
  class?: SafeClass
  fixed?: 'inline' | 'complex' | 'context'
}

// How something risky could do harm.
export type Risk = 'deletes' | 'discards' | 'outside' | 'publishes' | 'installs' | 'system' | 'stops' | 'remote' | 'secrets' | 'runs-code' | 'cloud'

// What "Prompt when unsafe" does with a command UNCLI can't place.
export type UnknownCommands = 'model' | 'inside' | 'ask'

// What a session would do with a command now, and why.
export interface Explanation {
  action: 'allow' | 'ask' | 'deny' // runs, prompts, or blocked (or declined while away)
  why: TraceReason[]
}

// A table of contents written by the quick-task model, anchored to the
// answer's top-level markdown blocks.
export interface PageOutline {
  provider: string
  model: string
  sections: { block: number; title: string; kind?: string }[]
  costUsd: number
  at: number
}

export type PageStatus = 'open' | 'done' | 'error' | 'interrupted'

export interface Page {
  id: string
  sessionId: string
  seq: number
  question: string
  directives?: string
  model: string
  modifiers: string[]
  answerMd: string
  trace: TraceItem[]
  touchedFiles: { path: string; line?: number; how: string }[]
  status: PageStatus
  error?: string
  bookmarked: boolean
  pinned: boolean
  inputTokens: number
  outputTokens: number
  cacheRead: number
  cacheWrite: number
  costUsd: number
  durationMs: number
  startedAt: number
  finishedAt: number
  outline?: PageOutline
  attachments?: PageAttachment[] // files sent with the question
}

// A file sent with a page's question (its content is in the CLI's transcript).
export interface PageAttachment {
  name: string
  path?: string // empty for pasted data
  mediaType: string
  size: number
}

// Older traces name the rule that let a tool use run.
export type RuleAction = 'allow' | 'ask' | 'deny'
export interface ToolRule {
  tool: string
  prefix?: string
  action: RuleAction
}

// A tool use waiting for the user's answer.
export interface Approval {
  requestId: string
  tool: string
  input?: unknown
  description?: string
  toolUseId?: string
  learn: SafeClass[] // what "This is safe" would remember; empty when it can only be allowed once
  class?: ToolClassView // what UNCLI takes it to do
  why?: TraceReason[] // what each part does, and why it needs the user
  judging?: boolean // the quick-task model is judging it: no card yet
  askedAt: number
}

// allow: this once; safe: and remember it as safe (with a scope); deny.
export type ApprovalDecision = 'allow' | 'safe' | 'deny'

// Search across sessions (the store's full-text index).
export interface SearchQuery {
  text: string
  profiles?: string[] // session types; empty = all
  sessionId?: string // only this session
  bookmarked?: boolean
  since?: number // ms
  until?: number // ms
  limit?: number
}

// One matching page; the snippet marks matches with \x01 … \x02.
export interface SearchHit {
  pageId: string
  sessionId: string
  sessionTitle: string
  profileId: string
  seq: number
  question: string
  field: 'title' | 'question' | 'answer' | 'extra'
  snippet: string
  bookmarked: boolean
  startedAt: number
}

export interface SearchResult {
  hits: SearchHit[]
  terms: string[] // the words searched for
}

// What a dropped or pasted path would attach as, or why it can't be.
export type AttachmentKind = 'image' | 'pdf' | 'text' | 'folder' | 'unsupported'
export interface AttachmentInfo {
  path: string
  name: string
  size: number
  kind: AttachmentKind
  mediaType?: string
  reason?: string
}

// A file to send with a turn: a path the app reads, or pasted image data.
export interface AttachmentRef {
  path?: string
  name?: string
  mediaType?: string
  data?: string // base64
}

export interface Profile {
  id: string
  label: string
  icon: string
  hue: number // type colour; 0 = the app accent
  folder: 'scratch' | 'pick' | 'repo'
  model: string
  tools: string[] | null
  modifiersOn: string[] | null
  ideLinks: boolean
}

export interface Modifier {
  id: string
  label: string
  icon: string
  scope: 'turn' | 'system'
  group?: string
  requiresTools?: string[]
  settings?: Record<string, string>
}

export interface ToolbarItem {
  kind: 'model_picker' | 'modifiers' | 'separator' | 'slash' | 'native'
  label?: string
  icon?: string
  command?: string
  action?: string
}

export interface ModelInfo {
  value: string
  resolved: string
  displayName: string
  description: string
  effortLevels: string[] | null
}

export interface Capabilities {
  partialStreaming: boolean
  resume: boolean
  liveModelSwitch: boolean
  interrupt: boolean
  approvals: boolean
  images: boolean
  usageReporting: boolean
  thinkingEvents: boolean
  slashPassthrough: boolean
}

// One model, chosen by the user, for every quick task (summaries now,
// things like commit messages later).
export interface ModelRef {
  provider: string
  model: string
}

// Which answers summarise themselves as they finish.
export type AutoSummary = 'off' | 'long' | 'always'

export interface Preferences {
  quickTaskModel: ModelRef
  autoSummary: AutoSummary // which answers summarise themselves as they finish
  unknownCommands?: UnknownCommands // Ask mode and commands UNCLI doesn't recognise (default model)
}

export interface Provider {
  id: string
  label: string
}

// What the user picked last time in the new-session dialog: the type, and
// per type the model and folder.
export interface NewSessionChoices {
  profileId?: string
  models: Record<string, string>
  folders: Record<string, string>
}

export interface CreatedSession {
  session: SessionView
  lastNewSession: NewSessionChoices
}

export interface Bootstrap {
  profiles: Profile[]
  modifiers: Modifier[]
  toolbar: ToolbarItem[]
  sessions: SessionView[]
  models: ModelInfo[] | null
  capabilities: Capabilities
  platform: string
  lastNewSession: NewSessionChoices
  preferences: Preferences
  providers: Provider[]
}

export interface CLIStatus {
  installed: boolean
  version: string
  pinned: string
  custom: boolean
  loggedIn: boolean
  email?: string
  subscription?: string
  error?: string
}

// Starting sign-in gives a link (already opened in the browser), or says
// the CLI signed in by itself.
export interface SignInStart {
  url?: string
  signedIn: boolean
  status: CLIStatus
}

// A file or folder dropped onto the window.
export interface DroppedPath {
  path: string
  isDir: boolean
  dir: string // the folder itself, or the file's folder
}

export interface Progress {
  done: number
  total: number
}

export interface UsageWindow {
  utilization: number
  resetsAt: number
}

export interface UsageLimit {
  status: string
  windows: Record<string, UsageWindow>
}

export type EventKind =
  | 'session_ready'
  | 'account'
  | 'turn_started'
  | 'thinking'
  | 'text_delta'
  | 'text_block'
  | 'tool_started'
  | 'tool_finished'
  | 'file_touched'
  | 'approval_asked'
  | 'notice'
  | 'usage_limit'
  | 'turn_result'
  | 'error'
  | 'exited'
  | 'unknown'

export interface UEvent {
  kind: EventKind
  turnSeq: number
  at: string
  data?: unknown
}

export interface SessionEventMsg {
  sessionId: string
  event: UEvent
}

export interface TextDelta {
  index: number
  text: string
}

export interface ThinkingData {
  estimatedTokens?: number
}

export interface Handlers {
  sessionEvent(msg: SessionEventMsg): void
  sessionChanged(v: SessionView): void
  pageChanged(p: Page): void
  cliProgress(p: Progress): void
  cliStatus(s: CLIStatus): void
}

// Backend is everything the UI can ask of UNCLI. The Wails bridge
// implements it in the app; a mock implements it in a plain browser.
export interface Backend {
  readonly kind: 'wails' | 'mock'
  subscribe(h: Handlers): () => void
  bootstrap(): Promise<Bootstrap>
  cliStatus(fresh: boolean): Promise<CLIStatus>
  installCLI(): Promise<CLIStatus>
  signIn(): Promise<SignInStart>
  submitLoginCode(code: string): Promise<CLIStatus>
  cancelSignIn(): Promise<void>
  setCLIVersion(version: string): Promise<CLIStatus>
  cliChannels(): Promise<Record<string, string>>
  pickFolder(title: string): Promise<string>
  createSession(profileId: string, workdir: string, model: string): Promise<CreatedSession>
  pages(sessionId: string): Promise<Page[]>
  send(sessionId: string, text: string, attachments?: AttachmentRef[]): Promise<void>
  interrupt(sessionId: string): Promise<void>
  setModel(sessionId: string, model: string): Promise<void>
  toggleModifier(sessionId: string, modifierId: string, on: boolean): Promise<SessionView>
  rename(sessionId: string, title: string): Promise<SessionView>
  setSortOrder(sessionId: string, order: number): Promise<SessionView>
  describePaths(paths: string[]): Promise<DroppedPath[]>
  describeAttachments(paths: string[]): Promise<AttachmentInfo[]>
  search(q: SearchQuery): Promise<SearchResult>
  answerApproval(sessionId: string, requestId: string, decision: ApprovalDecision, scope?: Scope): Promise<void>
  setUnattended(sessionId: string, on: boolean): Promise<SessionView>
  setMode(sessionId: string, mode: SessionMode): Promise<SessionView>
  safeList(sessionId: string): Promise<SafeEntry[]>
  setSafeEntry(sessionId: string, entry: SafeEntry, scope: Scope): Promise<SafeEntry[]>
  deleteSafeEntry(sessionId: string, entry: SafeEntry): Promise<SafeEntry[]>
  moveSafeEntry(sessionId: string, entry: SafeEntry, scope: Scope): Promise<SafeEntry[]>
  teach(sessionId: string, classes: SafeClass[], verdict: Verdict, scope: Scope): Promise<void>
  previewClasses(sessionId: string, command: string): Promise<ClassPreview[]>
  knownTools(sessionId: string): Promise<string[]>
  explainCommand(sessionId: string, command: string): Promise<Explanation>
  sessionAllowlist(sessionId: string): Promise<string[]>
  remove(sessionId: string): Promise<void>
  setBookmark(sessionId: string, pageId: string, on: boolean): Promise<Page>
  focus(sessionId: string): Promise<void>
  setPreferences(p: Preferences): Promise<Preferences>
  summarisePage(sessionId: string, pageId: string, blocks: string[]): Promise<Page>
  usage(): Promise<UsageLimit | null>
  openFolder(path: string): Promise<void>
  openURL(url: string): Promise<void>
  copyText(text: string): Promise<void>
  clipboardFiles(): Promise<string[]> // full paths of files copied in the file manager; [] if none
}
