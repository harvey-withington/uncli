// The client's state: plain, immutable objects. A change replaces what
// changed (and the objects above it), so anything holding the old snapshot
// keeps a consistent picture, and comparing references says what changed.
import type {
  ArtifactFile, Bootstrap, CLIStatus, CommandInfo, Connection, ContainersInfo, MCPServer, Page, PairRequest, PairedDevice,
  PeerState, PinnedPage, Progress, RemoteStatus, SessionView, UpdateStatus, UsageLimit,
} from '../types'

// Live is the text block of an answer still streaming, not yet in the
// page's answerMd, and how far its thinking has got.
export interface Live {
  seq: number
  text: string
  thinkingTokens: number
}

// SessionData is what the client holds for one session beyond its view.
export interface SessionData {
  pages: Page[] | null // null until loaded
  index: number // the page shown
  live: Live | null
  commands: CommandInfo[] // the slash commands its CLI takes
  commandsPending: boolean // its CLI is starting, to report them
  artifactSel: string // the artifact shown on its Artifacts tab
  artifactFiles: ArtifactFile[] | null // what is in its artifacts folder now
  mcpLinks: Record<string, string> // MCP server → its last sign-in link
  summarising: Record<string, boolean> // page id → a summary is being made
}

export const EMPTY_SESSION: SessionData = Object.freeze({
  pages: null,
  index: 0,
  live: null,
  commands: [],
  commandsPending: false,
  artifactSel: '',
  artifactFiles: null,
  mcpLinks: {},
  summarising: {},
})

// ClientState is everything the client knows about its server.
export interface ClientState {
  ready: boolean // the first state is in
  boot: Bootstrap | null
  cli: CLIStatus | null // the first provider's CLI
  providerStatus: Record<string, CLIStatus> // the others', by provider id
  progress: Progress | null // a CLI download
  containers: ContainersInfo | null
  remote: RemoteStatus | null // the server's remote access (its own window only)
  devices: PairedDevice[]
  servers: PeerState[] // the servers this PC is paired with (decision 0031), the desktop's only
  pairRequest: PairRequest | null
  connection: Connection | null // a remote client's link to its server
  update: UpdateStatus | null
  usage: UsageLimit | null // the latest usage limit any session reported
  sessions: SessionView[] // highest sortOrder first
  pins: PinnedPage[]
  data: Readonly<Record<string, SessionData>> // session id → its data
}

export const INITIAL_STATE: ClientState = Object.freeze({
  ready: false,
  boot: null,
  cli: null,
  providerStatus: {},
  progress: null,
  containers: null,
  remote: null,
  devices: [],
  servers: [],
  pairRequest: null,
  connection: null,
  update: null,
  usage: null,
  sessions: [],
  pins: [],
  data: {},
})

// Notices are things that happened that a UI may want to say or act on.
export type Notice =
  | { kind: 'mcpConnected'; sessionId: string; server: string; tools: number } // a server the user signed in to or reconnected
  | { kind: 'pageDone'; page: Page } // an answer finished
  | { kind: 'turnEnded'; sessionId: string }
  | { kind: 'notifyOpen'; sessionId: string } // a notification was opened
  | { kind: 'resynced' } // state was fetched afresh
  | { kind: 'error'; error: unknown } // something done in the background failed

export function sortSessions(list: SessionView[]): SessionView[] {
  return [...list].sort((a, b) => b.sortOrder - a.sortOrder || b.createdAt - a.createdAt)
}

// displayAnswer is the answer to show for a page: what's stored plus any
// block still streaming.
export function displayAnswer(page: Page, live: Live | null | undefined): string {
  if (!live || live.seq !== page.seq || !live.text) return page.answerMd
  return page.answerMd ? `${page.answerMd}\n\n${live.text}` : live.text
}

// connectedNow: servers the user acted on (by key "session:name") that are
// connected in the new list and weren't in the old one.
export function connectedNow(sessionId: string, before: MCPServer[] | undefined, after: MCPServer[] | undefined, acted: Set<string>): MCPServer[] {
  const was = new Map((before ?? []).map(s => [s.name, s.status]))
  return (after ?? []).filter(s => s.status === 'connected' && was.get(s.name) !== 'connected' && acted.has(`${sessionId}:${s.name}`))
}
