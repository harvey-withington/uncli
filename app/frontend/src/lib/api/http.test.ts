import { describe, expect, it, vi } from 'vitest'
import { RemoteOnlyError, RPCError, httpBackend, parseSSE } from './http'
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
