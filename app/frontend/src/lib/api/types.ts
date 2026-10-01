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
}

export interface TraceItem {
  id: string
  name: string
  summary: string
  done: boolean
  ok: boolean
  denied?: boolean
  output?: string
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
}

export interface Profile {
  id: string
  label: string
  icon: string
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

export interface Bootstrap {
  profiles: Profile[]
  modifiers: Modifier[]
  toolbar: ToolbarItem[]
  sessions: SessionView[]
  models: ModelInfo[] | null
  capabilities: Capabilities
  platform: string
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
  createSession(profileId: string, workdir: string, model: string): Promise<SessionView>
  pages(sessionId: string): Promise<Page[]>
  send(sessionId: string, text: string): Promise<void>
  interrupt(sessionId: string): Promise<void>
  setModel(sessionId: string, model: string): Promise<void>
  toggleModifier(sessionId: string, modifierId: string, on: boolean): Promise<SessionView>
  rename(sessionId: string, title: string): Promise<SessionView>
  remove(sessionId: string): Promise<void>
  setBookmark(sessionId: string, pageId: string, on: boolean): Promise<Page>
  focus(sessionId: string): Promise<void>
  usage(): Promise<UsageLimit | null>
  openFolder(path: string): Promise<void>
  openURL(url: string): Promise<void>
  copyText(text: string): Promise<void>
}
