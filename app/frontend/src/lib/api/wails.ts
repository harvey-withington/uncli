import type { Backend, Handlers } from './types'

// The Wails runtime injects window.go (bound methods) and window.runtime
// (events, clipboard). We call them directly rather than through generated
// bindings so the TypeScript types stay ours.
type Bound = Record<string, (...args: unknown[]) => Promise<unknown>>

interface WailsWindow {
  go?: { bridge?: { App?: Bound } }
  runtime?: {
    EventsOn(name: string, cb: (data: unknown) => void): () => void
    OnFileDrop?(cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean): void
    ClipboardSetText(text: string): Promise<boolean>
  }
}

const w = (): WailsWindow => window as unknown as WailsWindow

export function hasWails(): boolean {
  return !!w().go?.bridge?.App
}

function call<T>(method: string, ...args: unknown[]): Promise<T> {
  const fn = w().go?.bridge?.App?.[method]
  if (!fn) return Promise.reject(new Error(`bridge method ${method} missing`))
  return fn(...args) as Promise<T>
}

export function wailsBackend(): Backend {
  return {
    kind: 'wails',
    subscribe(h: Handlers) {
      const rt = w().runtime
      if (!rt) return () => {}
      const offs = [
        rt.EventsOn('session:event', d => h.sessionEvent(d as never)),
        rt.EventsOn('session:changed', d => h.sessionChanged(d as never)),
        rt.EventsOn('page:changed', d => h.pageChanged(d as never)),
        rt.EventsOn('cli:progress', d => h.cliProgress(d as never)),
        rt.EventsOn('cli:status', d => h.cliStatus(d as never)),
        rt.EventsOn('provider:status', d => h.providerStatus?.(d as never)),
        rt.EventsOn('containers:changed', d => h.containersChanged?.(d as never)),
        rt.EventsOn('notify:open', d => h.notifyOpen?.(String(d ?? ''))),
      ]
      return () => offs.forEach(off => off())
    },
    bootstrap: () => call('Bootstrap'),
    cliStatus: fresh => call('CLIStatus', fresh),
    installCLI: () => call('InstallCLI'),
    signIn: () => call('SignIn'),
    submitLoginCode: code => call('SubmitLoginCode', code),
    cancelSignIn: () => call('CancelSignIn'),
    setCLIVersion: v => call('SetCLIVersion', v),
    cliChannels: () => call('CLIChannels'),
    providerStatus: (id, fresh) => call('ProviderStatus', id, fresh),
    installProvider: id => call('InstallProvider', id),
    setProviderVersion: (id, v) => call('SetProviderVersion', id, v),
    providerChannels: id => call('ProviderChannels', id),
    enablePlugin: (id, hash) => call('EnablePlugin', id, hash),
    signInTerminal: id => call('SignInTerminal', id),
    revealCLI: id => call('RevealCLI', id),
    startDeviceSignIn: id => call('StartDeviceSignIn', id),
    cancelDeviceSignIn: id => call('CancelDeviceSignIn', id),
    disablePlugin: id => call('DisablePlugin', id),
    pickFolder: title => call('PickFolder', title),
    createSession: (p, d, m, c, prov) => call('CreateSession', p, d, m, c, prov),
    pages: id => call('Pages', id),
    send: (id, text, attachments = []) => call('Send', id, text, attachments),
    interrupt: id => call('Interrupt', id),
    setModel: (id, m) => call('SetModel', id, m),
    toggleModifier: (id, mod, on) => call('ToggleModifier', id, mod, on),
    rename: (id, t) => call('Rename', id, t),
    setSortOrder: (id, o) => call('SetSortOrder', id, o),
    describePaths: p => call('DescribePaths', p),
    describeAttachments: p => call('DescribeAttachments', p),
    search: q => call('Search', q),
    answerApproval: (s, r, d, scope) => call('AnswerApproval', s, r, d, scope ?? ''),
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
    clipboardFiles: () => call('ClipboardFiles'),
    remove: id => call('Delete', id),
    setBookmark: (s, p, on) => call('SetBookmark', s, p, on),
    setPinned: (s, p, on) => call('SetPinned', s, p, on),
    pinned: () => call('Pinned'),
    archive: (s, on) => call('Archive', s, on),
    focus: id => call('Focus', id),
    setPreferences: p => call('SetPreferences', p),
    setDecisionKey: k => call('SetDecisionKey', k),
    hasDecisionKey: () => call('HasDecisionKey'),
    testDecisionModel: () => call('TestDecisionModel'),
    testNotification: () => call('TestNotification'),
    summarisePage: (s, p, b) => call('SummarisePage', s, p, b),
    usage: () => call('Usage'),
    usageReport: q => call('UsageReport', q),
    containers: () => call('Containers'),
    installWSL: () => call('InstallWSL'),
    stopContainers: () => call('StopContainers'),
    buildContainer: id => call('BuildContainer', id),
    removeContainer: id => call('RemoveContainer', id),
    saveContainer: p => call('SaveContainer', p),
    removeContainerConfig: id => call('RemoveContainerConfig', id),
    startContainerSignIn: () => call('StartContainerSignIn'),
    finishContainerSignIn: code => call('FinishContainerSignIn', code),
    cancelContainerSignIn: () => call('CancelContainerSignIn'),
    signOutContainers: () => call('SignOutContainers'),
    startContainerAccountSignIn: () => call('StartContainerAccountSignIn'),
    finishContainerAccountSignIn: code => call('FinishContainerAccountSignIn', code),
    cancelContainerAccountSignIn: () => call('CancelContainerAccountSignIn'),
    signOutContainerAccount: () => call('SignOutContainerAccount'),
    theme: () => call('Theme'),
    openFolder: p => call('OpenFolder', p),
    openFile: (s, p, l) => call('OpenFile', s, p, l),
    openPath: (s, p, l) => call('OpenPath', s, p, l),
    revealFile: p => call('RevealFile', p),
    editors: () => call('Editors'),
    transcripts: () => call('Transcripts'),
    importTranscript: (id, p) => call('ImportTranscript', id, p),
    artifactFiles: s => call('ArtifactFiles', s),
    readArtifact: (s, r) => call('ReadArtifact', s, r),
    artifactPath: (s, p) => call('ArtifactPath', s, p),
    openURL: u => call('OpenURL', u),
    // The WebView's own clipboard first; the native one if that's refused.
    async copyText(text) {
      try {
        await navigator.clipboard.writeText(text)
        return
      } catch (first) {
        const rt = w().runtime
        if (!rt) throw first
        await rt.ClipboardSetText(text)
      }
    },
  }
}
