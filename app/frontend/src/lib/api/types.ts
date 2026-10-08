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
  runtimeRef?: string // the container profile a wsl session runs in
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
  background?: BackgroundTask[] // what runs in the background now (sub-agents, shells)
}

// A task running in the background. When it finishes, the CLI may carry on
// by itself in a turn of its own (a page with origin 'cli').
export interface BackgroundTask {
  id: string
  kind?: string // the provider's: local_agent, local_bash…
  description: string
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
  label?: string // the user's own name for it ("Deploys the site")
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
  | 'unsure' // the decision model wasn't sure enough that it's safe

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
  touchedFiles: TouchedFile[]
  artifacts?: ArtifactVersion[] // artifacts the turn added, changed or deleted
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
  origin?: 'cli' // the CLI started this turn by itself (a background task finished): no question
}

// A conversation the CLI saved, as the import picker lists it.
export interface TranscriptEntry {
  id: string // the provider's session id
  workdir: string
  started: number
  updated: number
  firstQuestion: string
  turns: number
  cliVersion?: string
  sessionId?: string // already in UNCLI as this session
  folderGone: boolean // its folder no longer exists
  fromChat: boolean // it ran in a UNCLI chat folder: a deleted UNCLI chat
  profile: string // the session type it would be imported as
}

// What an import made.
export interface ImportedSession {
  session: SessionView
  folderGone: boolean // imported archived: its folder no longer exists
  pagesLoaded: number
}

// A pinned page as the sidebar lists it, across sessions.
export interface PinnedPage {
  pageId: string
  sessionId: string
  seq: number
  question: string
  sessionTitle: string
  profileId: string
  archived: boolean
  startedAt: number
}

// A file the page's turn changed: written (created or replaced) or edited
// by a tool, changed or deleted by a command.
export interface TouchedFile {
  path: string // absolute
  line?: number // the latest edit's first changed line
  how: 'write' | 'edit' | 'command' | 'deleted'
  added?: number // lines added and removed by the turn's tools; absent when no tool said (a command's change)
  removed?: number
}

// An artifact as a page's turn left it: its content by hash, or deleted.
export interface ArtifactVersion {
  path: string // relative to the artifacts folder
  hash?: string
  size: number
  deleted?: boolean
}

// An artifact in a session's artifacts folder now.
export interface ArtifactFile {
  path: string // relative, with slashes
  size: number
}

// An artifact to read: a stored version by hash, or the file in the folder by path.
export interface ArtifactRef {
  hash?: string
  path?: string
}

// An artifact's content, base64.
export interface ArtifactContent {
  data: string
  size: number
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
  marked?: boolean // "This is safe" is on: its classes are on the safe list, and it waits for Allow once or Deny
  markedScope?: Scope
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
  artifacts?: boolean // ./artifacts/ is versioned per page and shown in the artifact pane
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
  hookApprovals?: boolean // approvals come through UNCLI's hook (decision 0012)
  images: boolean
  documents?: boolean
  usageReporting: boolean
  thinkingEvents: boolean
  slashPassthrough: boolean
  import?: boolean // saved CLI conversations can be imported
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
  decisionModel?: DecisionModel // answers "is this safe?"; the quick-task model unless set
  notifications?: NotifySetting // desktop notifications (default all)
  editor?: string // opens files in sessions that link to an IDE: a preset id, auto or custom
  editorCommand?: string // custom: a template with {file}, {line}, {folder}
}

// An editor UNCLI knows, and whether it is installed.
export interface Editor {
  id: string
  label: string
  command: string // template: {file}, {line}, {folder}
  found: boolean
}

// Which desktop notifications to show: a background session finishing or
// needing approval (all), only approvals, or none.
export type NotifySetting = 'all' | 'approvals' | 'off'

// The app-level decision model (decision record 0006): the quick-task
// model, or a System One server (Kev, Jev) with its address, model and
// pinned version; the threshold says how sure "safe" must be.
export interface DecisionModel {
  provider: 'quick-task' | 'systemone'
  endpoint?: string
  model?: string
  version?: string
  localOnly?: boolean
  threshold?: number // 0.5 to 1; default 0.9
}

// An AI provider: a CLI sessions run on, with the names the interface uses
// for it (providers.yaml), since UNCLI's own text names no CLI.
export interface Provider {
  id: string
  label: string
  name: string // the CLI: in setup and settings
  agent: string // who acts in a session
  account: string // the account it signs in with
  // How its CLI signs in: through a link UNCLI opens, or elsewhere (the
  // provider's own app or terminal, whose sign-in the CLI shares).
  signIn?: 'link' | 'elsewhere'
  capabilities?: Capabilities
}

// What the user picked last time in the new-session dialog: the type, and
// per type the model and folder.
export interface NewSessionChoices {
  profileId?: string
  models: Record<string, string>
  folders: Record<string, string>
  containers?: Record<string, string> // where each profile last ran: a container id, or "" for this machine
  providers?: Record<string, string> // the AI provider each profile last ran on
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
  editors?: Editor[]
}

export interface CLIStatus {
  provider?: string // whose CLI this is
  installed: boolean
  version: string
  pinned: string
  custom: boolean
  loggedIn: boolean
  email?: string
  subscription?: string
  error?: string
  models?: ModelInfo[] // for CLIs whose sign-in check lists the account's models
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
  provider?: string // whose CLI is downloading
  done: number
  total: number
}

export interface UsageWindow {
  utilization: number
  resetsAt: number
}

// The usage dashboard: what pages recorded, summed over a period.
export interface UsageQuery {
  since?: number // ms; absent = from the start
  until?: number
}
export interface UsageTotals {
  turns: number
  input: number
  output: number
  cacheRead: number
  cacheWrite: number
  costUsd: number
  durationMs: number
}
// One group: a day ("2026-10-06", local), a model, a session type or a session (label: its title).
export interface UsageRow extends UsageTotals {
  key: string
  label?: string
}
export interface UsageReport {
  totals: UsageTotals
  byDay: UsageRow[] // oldest first; days without turns left out
  byModel: UsageRow[]
  byProfile: UsageRow[]
  bySession: UsageRow[] // costliest first, at most 10
  sessions: number
  quickTasks: number
  quickTaskCostUsd: number
  first?: number
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
  | 'background_tasks'
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
  providerStatus?(s: CLIStatus): void // any provider's, its provider field says which
  containersChanged?(c: ContainersInfo): void
  notifyOpen?(sessionId: string): void // the user clicked a notification ("" for the tray icon)
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
  // Any provider's CLI, by its id: status (with the account's models, for
  // CLIs that list them), install, version and release channels.
  providerStatus(id: string, fresh: boolean): Promise<CLIStatus>
  installProvider(id: string): Promise<CLIStatus>
  setProviderVersion(id: string, version: string): Promise<CLIStatus>
  providerChannels(id: string): Promise<Record<string, string>>
  pickFolder(title: string): Promise<string>
  // container: a container profile id, "" for this machine; provider: an AI provider's id, "" for the first
  createSession(profileId: string, workdir: string, model: string, container: string, provider: string): Promise<CreatedSession>
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
  setSafeLabel(sessionId: string, entry: SafeEntry, label: string): Promise<SafeEntry[]> // empty clears it
  markSafe(sessionId: string, requestId: string, on: boolean, scope: Scope): Promise<void> // the "This is safe" toggle on a card
  teach(sessionId: string, classes: SafeClass[], verdict: Verdict, scope: Scope): Promise<void>
  previewClasses(sessionId: string, command: string): Promise<ClassPreview[]>
  knownTools(sessionId: string): Promise<string[]>
  explainCommand(sessionId: string, command: string): Promise<Explanation>
  sessionAllowlist(sessionId: string): Promise<string[]>
  remove(sessionId: string): Promise<void>
  setBookmark(sessionId: string, pageId: string, on: boolean): Promise<Page>
  setPinned(sessionId: string, pageId: string, on: boolean): Promise<Page>
  pinned(): Promise<PinnedPage[]> // pinned pages of every session, newest first
  archive(sessionId: string, on: boolean): Promise<SessionView> // archive (stops its CLI) or restore
  focus(sessionId: string): Promise<void>
  setPreferences(p: Preferences): Promise<Preferences>
  setDecisionKey(key: string): Promise<void> // the decision model's API key; never read back
  hasDecisionKey(): Promise<boolean>
  testDecisionModel(): Promise<DecisionTest> // a sample question to the decision model
  testNotification(): Promise<void> // shows a sample desktop notification
  summarisePage(sessionId: string, pageId: string, blocks: string[]): Promise<Page>
  usage(): Promise<UsageLimit | null>
  usageReport(q: UsageQuery): Promise<UsageReport>
  containers(): Promise<ContainersInfo> // WSL, the sign-in and each container profile (decision 0011)
  installWSL(): Promise<void> // one administrator prompt; Windows restarts before WSL works
  stopContainers(): Promise<void> // stops UNCLI's own containers only (never wsl --shutdown: WSL is shared)
  buildContainer(id: string): Promise<void> // progress arrives as containersChanged
  removeContainer(id: string): Promise<void>
  saveContainer(p: ContainerProfile): Promise<void> // into the user's containers.yaml; a built-in's id replaces it
  removeContainerConfig(id: string): Promise<void> // a built-in: back to UNCLI's; the user's own: deleted with its container
  startContainerSignIn(): Promise<string> // the link to approve
  finishContainerSignIn(code: string): Promise<void>
  cancelContainerSignIn(): Promise<void>
  signOutContainers(): Promise<void>
  startContainerAccountSignIn(): Promise<string> // the full account sign-in: the link to approve
  finishContainerAccountSignIn(code: string): Promise<void>
  cancelContainerAccountSignIn(): Promise<void>
  signOutContainerAccount(): Promise<void>
  theme(): Promise<ThemeFileInfo> // the user's theme.yaml, if there is one (decision 0010)
  openFolder(path: string): Promise<void>
  openFile(sessionId: string, path: string, line: number): Promise<void> // editor at line (IDE-linked sessions) or default app (documents)
  openPath(sessionId: string, path: string, line: number): Promise<void> // a link in an answer: a file as openFile, a folder shown
  revealFile(path: string): Promise<void> // shows it in its folder
  editors(): Promise<Editor[]>
  transcripts(): Promise<TranscriptEntry[]> // conversations the CLI saved, newest first
  importTranscript(id: string, profileId: string): Promise<ImportedSession> // empty profile: guessed
  artifactFiles(sessionId: string): Promise<ArtifactFile[]> // what is in the artifacts folder now
  readArtifact(sessionId: string, ref: ArtifactRef): Promise<ArtifactContent>
  artifactPath(sessionId: string, path: string): Promise<string> // where it is on disk now
  openURL(url: string): Promise<void>
  copyText(text: string): Promise<void>
  clipboardFiles(): Promise<string[]> // full paths of files copied in the file manager; [] if none
}

// What the decision model said about a sample command.
export interface DecisionTest {
  decider: string // which one answered, with its model and version
  calibrated: boolean // its probabilities can be trusted
  command: string
  level: 'looks' | 'routine' | 'risky'
  risk?: Risk
  confidence: number // of the level
  verdict: 'looks' | 'routine' | 'risky' // what UNCLI makes of it (risky when not sure enough)
  reason?: string
  millis: number
}

// The user's theme.yaml: values for the public --uncli-* tokens by scheme.
export interface ThemeFileInfo {
  path: string
  found: boolean
  light?: Record<string, string> | null
  dark?: Record<string, string> | null
}

// Containers: WSL distros UNCLI builds from container profiles (decision 0011).
export interface WSLStatus {
  installed: boolean
  version?: string
  kernel?: string
  distros: string[] // UNCLI's own (uncli-*)
  unresponsive?: boolean // wsl.exe didn't answer in time: WSL's service is stuck
}

export type ContainerStep = 'download' | 'remove' | 'import' | 'packages' | 'setup' | 'cli' | 'lockdown' | 'check'

// A container as containers.yaml defines it; the editor saves one.
export interface ContainerProfile {
  id: string
  label: string
  description?: string
  base: string
  packages: string[]
  setup: string[] // shell steps run as root, in order, after the packages
  brain?: 'shared' | 'sandboxed' | '' // shared: the user's memories, skills, agents, commands, CLAUDE.md, git name
  mcp?: 'shared' | 'none' | '' // shared: the user's MCP servers that can run on Linux
  connectors?: 'shared' | 'none' | '' // shared: the user's claude.ai connectors, with the full account sign-in
}

export interface ContainerBase {
  id: string
  label: string
}

export interface ContainerInfo extends ContainerProfile {
  baseLabel: string
  builtin: boolean // ships with UNCLI
  edited: boolean // the user's containers.yaml has their own version
  built: boolean
  step?: ContainerStep // being built
  percent?: number // download: how much is done
  error?: string // the last build failed
  // What differs from how a built container was built; a rebuild updates it.
  changes?: Array<'packages' | 'base' | 'cli' | 'steps' | 'setup' | 'unknown'>
}

export interface ContainersInfo {
  wsl: WSLStatus
  signedIn: boolean
  accountSignedIn?: boolean // the full account sign-in, for containers that share connectors
  containers: ContainerInfo[]
  bases: ContainerBase[] // what a container can be built on
  error?: string // containers.yaml couldn't be read
}
