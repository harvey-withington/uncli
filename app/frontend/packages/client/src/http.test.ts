import { describe, expect, it, vi } from 'vitest'
import { RemoteOnlyError, RPCError, absolute, httpBackend, mountedBackend, parseSSE, setHostHeaders } from './http'
import type { Connection, Handlers } from './types'

type Call = { url: string; init: RequestInit }

function reply(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

// A stream that sends chunks, then stays open until the test ends it.
function sse(chunks: string[]): { res: Response; end: () => void } {
  let ctl: ReadableStreamDefaultController<Uint8Array>
  const enc = new TextEncoder()
  const body = new ReadableStream<Uint8Array>({
    start(c) {
      ctl = c
      chunks.forEach(ch => c.enqueue(enc.encode(ch)))
    },
  })
  return { res: new Response(body, { status: 200 }), end: () => ctl.close() }
}

const quiet: Handlers = { sessionEvent() {}, sessionChanged() {}, pageChanged() {}, cliProgress() {}, cliStatus() {} }

describe('parseSSE', () => {
  it('splits frames and keeps what is incomplete', () => {
    const { frames, rest } = parseSSE('id: e:1\nevent: hello\ndata: {"a":1}\n\n: ping\n\nid: e:2\nevent: page:changed\ndata: {}\n\nid: e:3\nev')
    expect(frames).toEqual([{ id: 'e:1', event: 'hello', data: '{"a":1}' }, { id: 'e:2', event: 'page:changed', data: '{}' }])
    expect(rest).toBe('id: e:3\nev')
  })
})

describe('httpBackend', () => {
  it('calls the server with the token in a header, never the URL', async () => {
    const calls: Call[] = []
    const fetch = vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init })
      return reply({ jsonrpc: '2.0', id: 1, result: [{ id: 'p1' }] })
    })
    const b = httpBackend({ base: 'https://h:1', token: () => 'tok', fetch: fetch as never })
    expect(await b.pages('s1')).toEqual([{ id: 'p1' }])
    expect(calls[0]!.url).toBe('https://h:1/rpc')
    expect((calls[0]!.init.headers as Record<string, string>).Authorization).toBe('Bearer tok')
    expect(JSON.parse(calls[0]!.init.body as string)).toMatchObject({ method: 'Pages', params: ['s1'] })
    expect(calls.every(c => !c.url.includes('tok'))).toBe(true)
  })

  it('sends a turn with options only through SendWith, so a plain turn works with any server', async () => {
    const methods: string[] = []
    const fetch = vi.fn(async (_url: string, init: RequestInit) => {
      methods.push(JSON.parse(init.body as string).method)
      return reply({ jsonrpc: '2.0', id: 1, result: null })
    })
    const b = httpBackend({ base: '', token: () => 'tok', fetch: fetch as never })
    await b.send('s1', 'plain')
    await b.send('s1', 'with context', [], { context: 'Card 42' })
    await b.send('s1', 'empty context', [], { context: '' })
    expect(methods).toEqual(['Send', 'SendWith', 'Send'])
  })

  it('surfaces the server\u2019s errors, and a lost pairing', async () => {
    const unauthorized = vi.fn()
    let status = 200
    const fetch = vi.fn(async () => (status === 401 ? reply({ error: 'not paired' }, 401) : reply({ jsonrpc: '2.0', id: 1, error: { code: -32000, message: 'no such session' } })))
    const b = httpBackend({ base: '', token: () => 'tok', unauthorized, fetch: fetch as never })
    await expect(b.pages('x')).rejects.toThrow('no such session')
    await expect(b.pages('x')).rejects.toBeInstanceOf(RPCError)
    status = 401
    await expect(b.pages('x')).rejects.toThrow(/no longer paired/)
    expect(unauthorized).toHaveBeenCalled()
  })

  it('says what came back when a reply isn’t the server’s JSON', async () => {
    const page = new Response('<html>Bad gateway</html>', { status: 502, statusText: 'Bad Gateway', headers: { 'Content-Type': 'text/html' } })
    const b = httpBackend({ base: '', token: () => 'tok', fetch: vi.fn(async () => page) as never })
    const err = await b.pages('x').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(RPCError)
    expect((err as RPCError).message).toBe('The server sent something UNCLI didn’t expect (502 Bad Gateway). Try again in a moment.')
  })

  it('says it couldn’t reach the server, not the browser’s "Failed to fetch"', async () => {
    const b = httpBackend({ base: '', token: () => 'tok', fetch: vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never })
    const err = await b.bootstrap().catch((e: unknown) => e)
    expect(err).toBeInstanceOf(RPCError)
    expect((err as RPCError).message).toBe('Couldn’t reach the server. Check the connection and try again.')
  })

  it('gives up on the opening state after a while, as not reaching the server', async () => {
    vi.useFakeTimers()
    try {
      // A server whose network is down: the request is never answered.
      const fetch = vi.fn((_url: string, init: RequestInit) => new Promise<Response>((_, reject) => {
        init.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      }))
      const b = httpBackend({ base: '', token: () => 'tok', fetch: fetch as never })
      const err = b.bootstrap().catch((e: unknown) => e)
      await vi.advanceTimersByTimeAsync(15_000)
      expect(await err).toBeInstanceOf(RPCError)
      expect(((await err) as RPCError).message).toBe('Couldn’t reach the server. Check the connection and try again.')
      // Other calls wait as long as they need (a summary, say).
      expect(fetch.mock.calls.length).toBe(1)
      void b.pages('s1')
      expect(fetch.mock.calls[1]![1].signal).toBeUndefined()
    } finally {
      vi.useRealTimers()
    }
  })

  it('puts the server’s links under the path a host mounted it on', () => {
    const link = '/l/artifact?d=host&sig=x'
    expect(absolute('/uncli', link)).toBe(`${location.origin}/uncli/l/artifact?d=host&sig=x`)
    expect(absolute('/uncli/', link)).toBe(`${location.origin}/uncli/l/artifact?d=host&sig=x`)
    expect(absolute('https://h:1', link)).toBe('https://h:1/l/artifact?d=host&sig=x')
    expect(absolute('https://h:1/uncli', link)).toBe('https://h:1/uncli/l/artifact?d=host&sig=x')
    expect(absolute('', link)).toBe(`${location.origin}/l/artifact?d=host&sig=x`)
  })

  it('reaches a host’s own UNCLI at a path of the page’s server, with no token, one backend per path', async () => {
    expect(mountedBackend('/uncli')).toBe(mountedBackend('/uncli/'))
    expect(mountedBackend('/uncli')).not.toBe(mountedBackend('/other'))
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(reply({ jsonrpc: '2.0', id: 1, result: [] }))
    try {
      await mountedBackend('/uncli').pages('s1')
      const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit]
      expect(url).toBe('/uncli/rpc')
      expect((init.headers as Record<string, string>).Authorization).toBeUndefined()
    } finally {
      fetch.mockRestore()
    }
  })

  it('sends the host’s own sign-in with every request to its UNCLI, renewed as it changes', async () => {
    const seen: { url: string; auth?: string }[] = []
    const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, init) => {
      seen.push({ url: String(url), auth: (init?.headers as Record<string, string> | undefined)?.Authorization })
      return reply({ jsonrpc: '2.0', id: 1, result: [] })
    })
    let token = 'first'
    setHostHeaders(() => ({ Authorization: `Bearer ${token}` }))
    try {
      // A page from a desktop app's webview reaches the host's server by its full address.
      const b = mountedBackend('https://bruv.example:9870/uncli')
      await b.pages('s1')
      token = 'renewed'
      await b.upload(new Blob(['x']), 'a.txt')
      expect(seen).toEqual([
        { url: 'https://bruv.example:9870/uncli/rpc', auth: 'Bearer first' },
        { url: 'https://bruv.example:9870/uncli/upload?name=a.txt', auth: 'Bearer renewed' },
      ])
    } finally {
      setHostHeaders(null)
      fetch.mockRestore()
    }
  })

  it('sets up the CLIs only on a host’s own UNCLI, leaving the sign-in link to a click', async () => {
    const methods: string[] = []
    const fetch = vi.fn(async (_url: string, init: RequestInit) => {
      const { method } = JSON.parse(init.body as string)
      methods.push(method)
      return reply({ jsonrpc: '2.0', id: 1, result: method === 'SignIn' ? { url: 'https://claude.ai/oauth?x=1', signedIn: false, status: {} } : {} })
    })
    const opened = vi.spyOn(window, 'open').mockReturnValue(null)
    const host = httpBackend({ base: '/uncli', token: () => null, setup: true, fetch: fetch as never })
    await host.installCLI()
    await host.signIn()
    await host.submitLoginCode('abc')
    expect(methods).toEqual(['InstallCLI', 'SignIn', 'SubmitLoginCode'])
    // Opened after an await, a browser would block it as a popup.
    expect(opened).not.toHaveBeenCalled()
    opened.mockRestore()
    const phone = httpBackend({ base: '', token: () => 'tok', fetch: fetch as never })
    await expect(phone.installCLI()).rejects.toBeInstanceOf(RemoteOnlyError)
    await expect(phone.signIn()).rejects.toBeInstanceOf(RemoteOnlyError)
  })

  it('refuses what only works at the server', async () => {
    const b = httpBackend({ base: '', token: () => 'tok', fetch: vi.fn() as never })
    await expect(b.openFolder('C:\\')).rejects.toBeInstanceOf(RemoteOnlyError)
    await expect(b.setPreferences({} as never)).rejects.toBeInstanceOf(RemoteOnlyError)
    expect(await b.theme()).toEqual({ path: '', found: false })
  })

  it('uploads a file as the request body', async () => {
    const fetch = vi.fn(async () => reply({ id: 'u1', name: 'p.jpg', kind: 'image', size: 3, path: 'x' }))
    const b = httpBackend({ base: 'https://h:1', token: () => 'tok', fetch: fetch as never })
    const up = await b.upload(new Blob(['abc'], { type: 'image/jpeg' }), 'p .jpg')
    expect(up.id).toBe('u1')
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('https://h:1/upload?name=p%20.jpg')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok')
  })

  it('streams events, resumes from the last one, and resyncs when told to', async () => {
    const streams = [
      sse(['id: E:5\nevent: hello\ndata: {"reload":true,"cursor":"E:5"}\n\n', 'id: E:6\nevent: session:changed\ndata: {"id":"s1","state":"idle"}\n\n']),
      sse(['id: E:6\nevent: hello\ndata: {"reload":false,"cursor":"E:6"}\n\n', 'id: E:7\nevent: page:changed\ndata: {"id":"p1","sessionId":"s1"}\n\n']),
    ]
    const sent: (string | undefined)[] = []
    let n = 0
    const fetch = vi.fn(async (_url: string, init: RequestInit) => {
      sent.push((init.headers as Record<string, string>)['Last-Event-ID'])
      return streams[n++]!.res
    })
    const views: string[] = []
    const pages: string[] = []
    const states: Connection['state'][] = []
    const resync = vi.fn()
    const b = httpBackend({ base: '', token: () => 'tok', fetch: fetch as never })
    const off = b.subscribe({
      ...quiet, resync,
      sessionChanged: v => views.push(v.id),
      pageChanged: p => pages.push(p.id),
      connection: c => states.push(c.state),
    })
    await vi.waitFor(() => expect(views).toEqual(['s1']))
    expect(resync).toHaveBeenCalledTimes(1)
    streams[0]!.end() // the connection drops
    await vi.waitFor(() => expect(pages).toEqual(['p1']), { timeout: 3000 })
    expect(sent).toEqual([undefined, 'E:6'])
    expect(resync).toHaveBeenCalledTimes(1)
    expect(states).toEqual(['connecting', 'online', 'offline', 'connecting', 'online'])
    off()
  })

  it('stops for good when the device is no longer paired', async () => {
    const unauthorized = vi.fn()
    const fetch = vi.fn(async () => reply({ error: 'not paired' }, 401))
    const off = httpBackend({ base: '', token: () => 'old', unauthorized, fetch: fetch as never }).subscribe(quiet)
    await vi.waitFor(() => expect(unauthorized).toHaveBeenCalled())
    await new Promise(r => setTimeout(r, 700))
    expect(fetch).toHaveBeenCalledTimes(1)
    off()
  })
})
