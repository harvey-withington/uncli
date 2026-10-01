import type { Backend, Handlers } from './types'

// The Wails runtime injects window.go (bound methods) and window.runtime
// (events, clipboard). We call them directly rather than through generated
// bindings so the TypeScript types stay ours.
type Bound = Record<string, (...args: unknown[]) => Promise<unknown>>

interface WailsWindow {
  go?: { bridge?: { App?: Bound } }
  runtime?: {
    EventsOn(name: string, cb: (data: unknown) => void): () => void
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
    pickFolder: title => call('PickFolder', title),
    createSession: (p, d, m) => call('CreateSession', p, d, m),
    pages: id => call('Pages', id),
    send: (id, text) => call('Send', id, text),
    interrupt: id => call('Interrupt', id),
    setModel: (id, m) => call('SetModel', id, m),
    toggleModifier: (id, mod, on) => call('ToggleModifier', id, mod, on),
    rename: (id, t) => call('Rename', id, t),
    remove: id => call('Delete', id),
    setBookmark: (s, p, on) => call('SetBookmark', s, p, on),
    focus: id => call('Focus', id),
    usage: () => call('Usage'),
    openFolder: p => call('OpenFolder', p),
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
