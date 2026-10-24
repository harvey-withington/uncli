import { describe, expect, it } from 'vitest'
import { LOCAL, SESSION_FIRST, parseRef, qualify, serverKey, withServer } from './identity'
import { mockBackend } from './mock'
import type { Handlers, Page, SessionEventMsg, SessionView } from './types'

const SERVER = 'deviant.tail2ebd58.ts.net:9871'

// Every method that doesn't take a session id first. A new Backend method
// must be added here or to SESSION_FIRST, so none slips through unmapped.
const NOT_SESSION = [
  'subscribe', 'bootstrap', 'cliStatus', 'installCLI', 'signIn', 'submitLoginCode', 'cancelSignIn', 'setCLIVersion',
  'cliChannels', 'providerStatus', 'installProvider', 'setProviderVersion', 'providerChannels', 'enablePlugin',
  'disablePlugin', 'signInTerminal', 'revealCLI', 'startDeviceSignIn', 'cancelDeviceSignIn', 'pickFolder',
  'createSession', 'describePaths', 'describeAttachments', 'search', 'clipboardFiles', 'pinned', 'setPreferences',
  'setDecisionKey', 'hasDecisionKey', 'testDecisionModel', 'testNotification', 'usage', 'usageReport', 'containers',
  'installWSL', 'stopContainers', 'buildContainer', 'removeContainer', 'saveContainer', 'removeContainerConfig',
  'startContainerSignIn', 'finishContainerSignIn', 'cancelContainerSignIn', 'signOutContainers',
  'startContainerAccountSignIn', 'finishContainerAccountSignIn', 'cancelContainerAccountSignIn',
  'signOutContainerAccount', 'theme', 'openFolder', 'revealFile', 'editors', 'transcripts', 'importTranscript',
  'openURL', 'copyText', 'remoteStatus', 'setRemoteAccess', 'startPairing', 'cancelPairing', 'answerPairing',
  'pairedDevices', 'revokeDevice', 'listFolder', 'upload', 'uploadLink',
]

describe('session identity', () => {
  it('keys a session by server and id', () => {
    expect(qualify(LOCAL, 'abc')).toBe('abc')
    expect(qualify(SERVER, 'abc')).toBe(`${SERVER}~abc`)
    expect(parseRef(`${SERVER}~abc`)).toEqual({ server: SERVER, id: 'abc' })
    expect(parseRef('abc')).toEqual({ server: LOCAL, id: 'abc' })
    expect(serverKey('https://deviant.tail2ebd58.ts.net:9871')).toBe(SERVER)
  })

  it('classifies every backend method', () => {
    const mockOnly = ['kind', 'writeWatched'] // the mock's own test hooks
    const methods = Object.keys(mockBackend()).filter(k => !mockOnly.includes(k))
    const known = new Set<string>([...SESSION_FIRST, ...NOT_SESSION])
    expect(methods.filter(m => !known.has(m))).toEqual([])
  })

  it('qualifies what the backend hands out and unqualifies what it takes', async () => {
    const b = withServer(mockBackend(), SERVER)
    const boot = await b.bootstrap()
    const s = boot.sessions[0]!
    expect(s.id.startsWith(`${SERVER}~`)).toBe(true)
    const pages = await b.pages(s.id)
    expect(pages.length).toBeGreaterThan(0)
    expect(pages.every(p => p.sessionId === s.id)).toBe(true)
    const renamed = await b.rename(s.id, 'New name')
    expect(renamed.id).toBe(s.id)
    const created = await b.createSession('chat', '', 'sonnet', '', '', [])
    expect(created.session.id.startsWith(`${SERVER}~`)).toBe(true)
    const found = await b.search({ text: 'the', sessionId: s.id })
    expect(found.hits.every(h => h.sessionId.startsWith(`${SERVER}~`))).toBe(true)
    // Another server's session isn't this backend's to touch.
    await expect(b.pages(`other:1~${parseRef(s.id).id}`)).rejects.toThrow(/isn't on/)
  })

  it('qualifies events', async () => {
    const inner = mockBackend()
    let raw: Handlers | null = null
    inner.subscribe = h => { raw = h; return () => {} }
    const got: { ev?: SessionEventMsg; view?: SessionView; page?: Page; open?: string } = {}
    withServer(inner, SERVER).subscribe({
      sessionEvent: m => (got.ev = m), sessionChanged: v => (got.view = v), pageChanged: p => (got.page = p),
      cliProgress: () => {}, cliStatus: () => {}, notifyOpen: id => (got.open = id),
    })
    const boot = await inner.bootstrap()
    raw!.sessionChanged(boot.sessions[0]!)
    raw!.sessionEvent({ sessionId: 's1', event: { kind: 'text_delta', seq: 1, at: '' } as never })
    raw!.notifyOpen?.('s1')
    expect(got.view?.id).toBe(`${SERVER}~${boot.sessions[0]!.id}`)
    expect(got.ev?.sessionId).toBe(`${SERVER}~s1`)
    expect(got.open).toBe(`${SERVER}~s1`)
  })

  it('leaves this PC’s sessions as they are', () => {
    const b = mockBackend()
    expect(withServer(b, LOCAL)).toBe(b)
  })
})
