// A UNCLI on another machine, as a Backend (decision 0021): calls go to its
// remote access server with the same names and arguments the window's
// bridge takes, and events come from its stream. How they get there is the
// transport's: fetch from a phone or a host's page (http.ts), UNCLI's own
// Go side from the desktop (peer.ts, decision 0031). What only works at the
// server's own screen is refused here, and the UI shows it disabled with
// the reason (lib/remoteonly.ts).
import { text } from './text'
import type { AttachmentRef, Backend, CLIStatus, DeviceSignIn, Handlers, SignInStart, Upload } from './types'

// RemoteOnlyError is a call that only the server's own window can make.
export class RemoteOnlyError extends Error {
  constructor(readonly method: string) {
    super(text('remote.onlyAtServer'))
    this.name = 'RemoteOnlyError'
  }
}

// RPCError is a call the server answered with an error; its message is
// the app's, meant for the user.
export class RPCError extends Error {
  constructor(message: string, readonly code: number) {
    super(message)
    this.name = 'RPCError'
  }
}

// How long the state the app opens with may take before the server is
// taken as unreachable.
const OPEN_DEADLINE_MS = 15_000

export interface Transport {
  // base: the server's address, for its signed links ('' for the page's own).
  base: () => string
  call<T>(method: string, ...params: unknown[]): Promise<T>
  // callWithin: a call that, unanswered after deadlineMs, is taken as not
  // reaching the server.
  callWithin<T>(deadlineMs: number, method: string, ...params: unknown[]): Promise<T>
  subscribe(h: Handlers): () => void
  upload(file: Blob, name: string): Promise<Upload>
  // attach turns attachments into what the server takes (files on this PC
  // uploaded first); absent, they go as they are.
  attach?(refs: AttachmentRef[]): Promise<AttachmentRef[]>
  // canSetUp: the server is a host's own UNCLI (decision 0027), which lets
  // its page install the CLIs and sign in to them; a phone can't.
  canSetUp?: boolean
}

// dispatchEvent hands one of the server's events to its handler; hello
// and anything unknown are the transport's to deal with.
export function dispatchEvent(name: string, data: unknown, h: Handlers): void {
  switch (name) {
    case 'session:event': return h.sessionEvent(data as never)
    case 'session:changed': return h.sessionChanged(data as never)
    case 'page:changed': return h.pageChanged(data as never)
    case 'cli:progress': return h.cliProgress(data as never)
    case 'cli:status': return h.cliStatus(data as never)
    case 'provider:status': return h.providerStatus?.(data as never)
    case 'containers:changed': return h.containersChanged?.(data as never)
    case 'update:status': return h.updateStatus?.(data as never)
  }
}

export function remoteBackend(tr: Transport): Backend {
  const call = <T>(method: string, ...params: unknown[]) => tr.call<T>(method, ...params)
  const attach = async (refs: AttachmentRef[]) => (tr.attach ? tr.attach(refs) : refs)
  const only = (method: string) => () => Promise.reject(new RemoteOnlyError(method))
  // Setting up the CLIs, where the server allows it (canSetUp).
  const setup = tr.canSetUp
    ? {
        installCLI: () => call<CLIStatus>('InstallCLI'),
        // The links aren't opened here: after an await, a browser takes that
        // for a popup. The setup screen offers them to click.
        signIn: () => call<SignInStart>('SignIn'),
        submitLoginCode: (code: string) => call<CLIStatus>('SubmitLoginCode', code),
        cancelSignIn: () => call<void>('CancelSignIn'),
        installProvider: (pid: string) => call<CLIStatus>('InstallProvider', pid),
        startDeviceSignIn: (pid: string) => call<DeviceSignIn>('StartDeviceSignIn', pid),
        cancelDeviceSignIn: (pid: string) => call<void>('CancelDeviceSignIn', pid),
      }
    : {}

  return {
    kind: 'http',
    canSetUp: !!tr.canSetUp,
    subscribe: h => tr.subscribe(h),
    bootstrap: () => tr.callWithin(OPEN_DEADLINE_MS, 'Bootstrap'),
    cliStatus: fresh => (fresh ? call('CLIStatus', fresh) : tr.callWithin(OPEN_DEADLINE_MS, 'CLIStatus', fresh)),
    providerStatus: (pid, fresh) => call('ProviderStatus', pid, fresh),
    cliChannels: () => call('CLIChannels'),
    providerChannels: pid => call('ProviderChannels', pid),
    installCLI: only('installCLI'),
    signIn: only('signIn'),
    submitLoginCode: only('submitLoginCode'),
    cancelSignIn: async () => {},
    setCLIVersion: only('setCLIVersion'),
    installProvider: only('installProvider'),
    setProviderVersion: only('setProviderVersion'),
    enablePlugin: only('enablePlugin'),
    disablePlugin: only('disablePlugin'),
    signInTerminal: only('signInTerminal'),
    revealCLI: only('revealCLI'),
    startDeviceSignIn: only('startDeviceSignIn'),
    cancelDeviceSignIn: async () => {},
    pickFolder: only('pickFolder'),
    createSession: (p, d, m, c, prov, sh, o) => (o?.meta || o?.title ? call('CreateSessionWith', p, d, m, c, prov, sh, o) : call('CreateSession', p, d, m, c, prov, sh)),
    setShares: (s, sh) => call('SetShares', s, sh),
    pages: s => call('Pages', s),
    // Only a turn with options needs SendWith; Send works with any server.
    send: async (s, text, attachments = [], sendOpts) => {
      const files = await attach(attachments)
      return sendOpts?.context ? call('SendWith', s, text, files, sendOpts) : call('Send', s, text, files)
    },
    interrupt: s => call('Interrupt', s),
    restart: s => call('Restart', s),
    setModel: (s, m) => call('SetModel', s, m),
    toggleModifier: (s, mod, on) => call('ToggleModifier', s, mod, on),
    rename: (s, title) => call('Rename', s, title),
    setSortOrder: (s, o) => call('SetSortOrder', s, o),
    describePaths: async () => [],
    describeAttachments: async () => [],
    search: q => call('Search', q),
    answerApproval: (s, r, d, scope) => call('AnswerApproval', s, r, d, scope ?? ''),
    answerAsk: (s, r, allow, answers, note) => call('AnswerAsk', s, r, allow, answers, note),
    commands: s => call('Commands', s),
    mcpSignIn: (s, server) => call('MCPSignIn', s, server),
    mcpCallback: (s, server, url) => call('MCPCallback', s, server, url),
    mcpReconnect: (s, server) => call('MCPReconnect', s, server),
    mcpToggle: (s, server, on) => call('MCPToggle', s, server, on),
    queue: async (s, text, attachments = [], opts) => call('Queue', s, text, await attach(attachments), opts ?? {}),
    unqueue: s => call('Unqueue', s),
    sendQueued: s => call('SendQueued', s),
    mcpSignOut: (s, server) => call('MCPSignOut', s, server),
    mcpCancel: (s, server) => call('MCPCancel', s, server),
    setUnattended: (s, on) => call('SetUnattended', s, on),
    setMode: (s, m) => call('SetMode', s, m),
    safeList: s => call('SafeList', s),
    setSafeEntry: (s, e, scope) => call('SetSafeEntry', s, e, scope),
    deleteSafeEntry: (s, e) => call('DeleteSafeEntry', s, e),
    moveSafeEntry: (s, e, scope) => call('MoveSafeEntry', s, e, scope),
    setSafeLabel: (s, e, label) => call('SetSafeLabel', s, e, label),
    markSafe: (s, r, on, scope) => call('MarkSafe', s, r, on, scope),
    teach: (s, c, v, scope) => call('Teach', s, c, v, scope),
    previewClasses: (s, c) => call('PreviewClasses', s, c),
    knownTools: s => call('KnownTools', s),
    explainCommand: (s, c) => call('ExplainCommand', s, c),
    sessionAllowlist: s => call('SessionAllowlist', s),
    clipboardFiles: async () => [],
    remove: s => call('Delete', s),
    setBookmark: (s, p, on) => call('SetBookmark', s, p, on),
    setPinned: (s, p, on) => call('SetPinned', s, p, on),
    pinned: () => call('Pinned'),
    archive: (s, on) => call('Archive', s, on),
    focus: s => call('Focus', s),
    setPreferences: only('setPreferences'),
    setResumeCutOff: on => call('SetResumeCutOff', on),
    setDecisionKey: only('setDecisionKey'),
    hasDecisionKey: () => call('HasDecisionKey'),
    testDecisionModel: only('testDecisionModel'),
    testNotification: only('testNotification'),
    summarisePage: (s, p, b) => call('SummarisePage', s, p, b),
    pageChanges: (s, p) => call('PageChanges', s, p),
    explainPage: (s, p, o) => call('ExplainPage', s, p, o),
    explainSize: (s, p, o) => call('ExplainSize', s, p, o),
    usage: () => call('Usage'),
    updateStatus: () => call('UpdateStatus'),
    checkForUpdates: () => call('CheckForUpdates'),
    installUpdate: () => call('InstallUpdate'),
    usageReport: q => call('UsageReport', q),
    containers: () => call('Containers'),
    installWSL: only('installWSL'),
    stopContainers: only('stopContainers'),
    buildContainer: only('buildContainer'),
    removeContainer: only('removeContainer'),
    saveContainer: only('saveContainer'),
    suggestedContainers: only('suggestedContainers'),
    importContainer: only('importContainer'),
    removeContainerConfig: only('removeContainerConfig'),
    startContainerSignIn: only('startContainerSignIn'),
    finishContainerSignIn: only('finishContainerSignIn'),
    cancelContainerSignIn: async () => {},
    signOutContainers: only('signOutContainers'),
    startContainerAccountSignIn: only('startContainerAccountSignIn'),
    finishContainerAccountSignIn: only('finishContainerAccountSignIn'),
    cancelContainerAccountSignIn: async () => {},
    signOutContainerAccount: only('signOutContainerAccount'),
    // The theme file is the server's; a remote client keeps its own look.
    theme: async () => ({ path: '', found: false }),
    openFolder: only('openFolder'),
    openFile: only('openFile'),
    openPath: only('openPath'),
    revealFile: only('revealFile'),
    watchFiles: (s, q) => call('WatchFiles', s, q),
    watchRead: (s, p, since) => call('WatchRead', s, p, since),
    editors: () => call('Editors'),
    transcripts: () => call('Transcripts'),
    importTranscript: (tid, p) => call('ImportTranscript', tid, p),
    artifactFiles: s => call('ArtifactFiles', s),
    readArtifact: (s, r) => call('ReadArtifact', s, r),
    artifactPath: only('artifactPath'),
    openURL: async u => {
      if (/^https?:\/\//.test(u)) window.open(u, '_blank', 'noopener,noreferrer')
    },
    copyText: text => navigator.clipboard.writeText(text),
    remoteStatus: only('remoteStatus'),
    setRemoteAccess: only('setRemoteAccess'),
    startPairing: only('startPairing'),
    cancelPairing: async () => {},
    answerPairing: only('answerPairing'),
    pairedDevices: async () => [],
    revokeDevice: only('revokeDevice'),
    // Other servers are the desktop's to pair with (decision 0031).
    remoteServers: async () => [],
    addRemoteServer: only('addRemoteServer'),
    cancelAddRemoteServer: async () => {},
    removeRemoteServer: only('removeRemoteServer'),
    thisPCName: async () => '',
    peer: key => {
      throw new Error(`no server ${key} here`)
    },
    listFolder: p => call('ListFolder', p),
    upload: (file, name) => tr.upload(file, name),
    pushKey: () => call('PushKey'),
    pushState: () => call('PushState'),
    pushSubscribe: sub => call('PushSubscribe', sub),
    pushUnsubscribe: () => call('PushUnsubscribe'),
    setPushNotifications: setting => call('SetPushNotifications', setting),
    testPush: () => call('TestPush'),
    artifactLink: async (s, r) => absolute(tr.base(), await call<string>('ArtifactLink', s, r)),
    uploadLink: async up => absolute(tr.base(), await call<string>('UploadLink', up)),
    ...setup,
  }
}


// absolute is a server path as a full URL, to open in the browser. The
// server's paths start at its root, which for a host's own UNCLI is the
// path the host mounted it on (/uncli, say), so they go under base.
export function absolute(base: string, path: string): string {
  const under = path.startsWith('/') ? base.replace(/\/+$/, '') + path : path
  return new URL(under, base && !base.startsWith('/') ? base : globalThis.location?.origin).href
}
