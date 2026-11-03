import { describe, expect, it, vi } from 'vitest'
import { mockBackend } from '../mock'
import type { Backend, Handlers, Page, SessionView } from '../types'
import { ChatClient, chatClient } from './client'
import type { Notice } from './state'

// The client on the mock, with the server's event handlers in reach, so a
// test can play the server.
async function started(backend: Backend = mockBackend()) {
  let h: Handlers | null = null
  const sub = backend.subscribe.bind(backend)
  backend.subscribe = x => { h = x; return sub(x) }
  const c = new ChatClient(backend)
  const notices: Notice[] = []
  c.onNotice(n => notices.push(n))
  await c.init()
  return { c, backend, h: () => h!, notices }
}

const delta = (sessionId: string, turnSeq: number, text: string) =>
  ({ sessionId, event: { kind: 'text_delta' as const, turnSeq, at: '', data: { text } } })

describe('ChatClient', () => {
  it('loads the server’s state, and replaces its snapshot only when something changes', async () => {
    const { c } = await started()
    const s = c.snapshot()
    expect(s.ready).toBe(true)
    expect(s.sessions.map(x => x.id)).toContain('s-code')
    expect(c.snapshot()).toBe(s) // nothing changed
    const seen = vi.fn()
    c.subscribe(seen)
    c.session('s-code').goTo(0)
    expect(seen).toHaveBeenCalledOnce()
    expect(c.snapshot()).not.toBe(s)
  })

  it('is one per backend', () => {
    const b = mockBackend()
    expect(chatClient(b)).toBe(chatClient(b))
    expect(chatClient(mockBackend())).not.toBe(chatClient(b))
  })

  it('streams a turn’s text per session, and tells only that session’s store', async () => {
    const { c, h } = await started()
    const code = c.session('s-code')
    const chat = c.session('s-chat')
    const onCode = vi.fn()
    const onChat = vi.fn()
    code.subscribe(onCode)
    chat.subscribe(onChat)
    h().sessionEvent(delta('s-code', 7, 'Hel'))
    h().sessionEvent(delta('s-code', 7, 'lo'))
    expect(code.snapshot().live).toEqual({ seq: 7, text: 'Hello', thinkingTokens: 0 })
    expect(onCode).toHaveBeenCalledTimes(2)
    expect(onChat).not.toHaveBeenCalled()
    // A new turn starts its own text; the turn's end clears it.
    h().sessionEvent(delta('s-code', 8, 'Next'))
    expect(code.snapshot().live?.text).toBe('Next')
    h().sessionEvent({ sessionId: 's-code', event: { kind: 'turn_result', turnSeq: 8, at: '', data: {} } })
    expect(code.snapshot().live).toBeNull()
  })

  it('keeps a session’s snapshot the same object while another session streams', async () => {
    const { c, h } = await started()
    await c.session('s-code').load()
    const before = c.session('s-code').snapshot()
    h().sessionEvent(delta('s-chat', 1, 'elsewhere'))
    expect(c.session('s-code').snapshot()).toBe(before)
  })

  it('drops the live text once the page stores it, shows a page just sent, and says when an answer finishes', async () => {
    const { c, h, notices } = await started()
    const code = c.session('s-code')
    await code.load()
    const n = code.snapshot().pages!.length
    const page: Page = { ...code.snapshot().pages![0]!, id: 'p-new', seq: 99, status: 'open', answerMd: '' }
    h().pageChanged(page)
    expect(code.snapshot().index).toBe(n) // the new page is shown
    h().sessionEvent(delta('s-code', 99, 'Partial'))
    h().pageChanged({ ...page, answerMd: 'Partial' })
    expect(code.snapshot().live?.text).toBe('') // stored now: not shown twice
    h().pageChanged({ ...page, answerMd: 'Partial answer.', status: 'done' })
    expect(notices).toContainEqual({ kind: 'pageDone', page: expect.objectContaining({ id: 'p-new' }) })
  })

  it('resyncs: every loaded session’s pages afresh, live text gone, the shown page kept in range, and changes meanwhile applied after', async () => {
    const backend = mockBackend()
    const { c, h, notices } = await started(backend)
    const code = c.session('s-code')
    const chat = c.session('s-chat')
    await code.load()
    await chat.load()
    h().sessionEvent(delta('s-code', 50, 'half'))
    h().sessionEvent(delta('s-chat', 51, 'other half'))
    code.goTo(1000) // further than the server will have

    let release: () => void = () => {}
    const boot = backend.bootstrap.bind(backend)
    backend.bootstrap = () => new Promise(r => { release = () => r(boot()) })
    const resync = c.resync()
    // A rename arrives while the snapshot is on its way: it must win.
    const renamed: SessionView = { ...c.viewOf('s-chat')!, title: 'Renamed meanwhile' }
    h().sessionChanged(renamed)
    expect(c.viewOf('s-chat')?.title).not.toBe('Renamed meanwhile') // held
    release()
    await resync

    expect(code.snapshot().live).toBeNull()
    expect(chat.snapshot().live).toBeNull()
    expect(code.snapshot().index).toBe(code.snapshot().pages!.length - 1)
    expect(c.viewOf('s-chat')?.title).toBe('Renamed meanwhile')
    expect(notices).toContainEqual({ kind: 'resynced' })
  })

  it('tries a failed resync again, then gives up with a notice and waits for the connection', async () => {
    vi.useFakeTimers()
    try {
      const backend = mockBackend()
      const { c, h, notices } = await started(backend)
      const boot = backend.bootstrap.bind(backend)
      const failing = vi.fn(() => Promise.reject(new Error('offline')))
      backend.bootstrap = failing
      await c.resync()
      for (const ms of [1000, 3000, 8000]) await vi.advanceTimersByTimeAsync(ms)
      expect(failing).toHaveBeenCalledTimes(4)
      expect(notices.some(n => n.kind === 'error')).toBe(true)
      backend.bootstrap = boot
      h().connection?.({ state: 'online', lastOnline: Date.now() })
      await vi.advanceTimersByTimeAsync(0)
      expect(notices).toContainEqual({ kind: 'resynced' })
    } finally {
      vi.useRealTimers()
    }
  })

  it('moves a session at once, and back if saving fails', async () => {
    const backend = mockBackend()
    const { c } = await started(backend)
    const active = () => c.snapshot().sessions.filter(s => !s.archived).map(s => s.id)
    const first = active()[0]!
    backend.setSortOrder = () => Promise.reject(new Error('no'))
    const before = active()
    await expect(c.moveSession(first, before.length)).rejects.toThrow('no')
    expect(active()).toEqual(before)
  })

  it('says when an MCP server the user signed in to connects', async () => {
    const { c, h, notices } = await started()
    const v = c.viewOf('s-code')!
    c.mcpActed.add('s-code:BRUV')
    h().sessionChanged({ ...v, mcp: [{ name: 'BRUV', status: 'connected', tools: 12 } as never] })
    expect(notices).toContainEqual({ kind: 'mcpConnected', sessionId: 's-code', server: 'BRUV', tools: 12 })
    expect(c.mcpActed.has('s-code:BRUV')).toBe(false)
  })

  it('removes a session with its pins and data', async () => {
    const { c } = await started()
    await c.session('s-chat').load()
    await c.remove('s-chat')
    expect(c.viewOf('s-chat')).toBeNull()
    expect(c.snapshot().data['s-chat']).toBeUndefined()
    expect(c.snapshot().pins.some(p => p.sessionId === 's-chat')).toBe(false)
  })
})
