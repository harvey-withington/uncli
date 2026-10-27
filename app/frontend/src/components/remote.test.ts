import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import type { Handlers } from '../lib/api'
import { AppStore } from '../stores/app.svelte'
import { atServer } from '../lib/remoteonly'
import { APP_CONTEXT } from '../lib/context'
import Composer from './Composer.svelte'

async function openRemote(backend = mockBackend()) {
  render(App, { props: { backend } })
  await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2./ }))
  const dialog = await screen.findByRole('dialog', { name: 'Settings' })
  await fireEvent.click(within(dialog).getByRole('tab', { name: 'Remote access' }))
  return dialog
}

describe('Settings → Remote access', () => {
  it('turns serving on and shows the address to open', async () => {
    const backend = mockBackend()
    const set = vi.spyOn(backend, 'setRemoteAccess')
    const dialog = await openRemote(backend)
    expect(await within(dialog).findByText('Off. Nothing listens.')).toBeInTheDocument()
    expect(within(dialog).getByRole('textbox', { name: 'Port' })).toHaveValue('9871')
    await fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Serve sessions to paired devices' }))
    await waitFor(() => expect(set).toHaveBeenCalledWith(true, 9871))
    expect(await within(dialog).findByText('On. Open this address on a paired device:')).toBeInTheDocument()
    expect(within(dialog).getByText('https://deviant.tail2ebd58.ts.net:9871')).toBeInTheDocument()
    expect(within(dialog).getByText('On this PC it’s also at http://127.0.0.1:9871.')).toBeInTheDocument()
  })

  it('refuses a port that cannot be used', async () => {
    const dialog = await openRemote()
    const port = await within(dialog).findByRole('textbox', { name: 'Port' })
    await waitFor(() => expect(port).toHaveValue('9871'))
    await fireEvent.input(port, { target: { value: '70000' } })
    expect(within(dialog).getByText('Choose a port from 1 to 65535.')).toBeInTheDocument()
    expect(within(dialog).getByRole('checkbox', { name: 'Serve sessions to paired devices' })).toBeDisabled()
  })

  it('turns serving off even while the port field is wrong', async () => {
    const backend = mockBackend()
    const set = vi.spyOn(backend, 'setRemoteAccess')
    const dialog = await openRemote(backend)
    const box = await within(dialog).findByRole('checkbox', { name: 'Serve sessions to paired devices' })
    await waitFor(() => expect(box).toBeEnabled())
    await fireEvent.click(box)
    await within(dialog).findByText('On. Open this address on a paired device:')
    await fireEvent.input(within(dialog).getByRole('textbox', { name: 'Port' }), { target: { value: 'abc' } })
    expect(box).toBeEnabled()
    await fireEvent.click(box)
    await waitFor(() => expect(set).toHaveBeenLastCalledWith(false, 9871))
    expect(await within(dialog).findByText('Off. Nothing listens.')).toBeInTheDocument()
  })

  it('pairs a device only with Allow here', async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    const answer = vi.spyOn(backend, 'answerPairing').mockResolvedValue()
    const dialog = await openRemote(backend)
    const pair = await within(dialog).findByRole('button', { name: 'Pair a device' })
    expect(pair).toBeDisabled()
    expect(within(dialog).getByText('Turn Remote access on, with Tailscale connected, to pair a device.')).toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Serve sessions to paired devices' }))
    await waitFor(() => expect(pair).toBeEnabled())
    await fireEvent.click(pair)
    const pd = await screen.findByRole('dialog', { name: 'Pair a device' })
    expect(await within(pd).findByText('https://deviant.tail2ebd58.ts.net:9871/m/#pair=K7QWM3XRP2')).toBeInTheDocument()
    expect(within(pd).getByRole('img', { name: 'QR code of the pairing link' })).toBeInTheDocument()
    expect(within(pd).getByText(/The code expires in [45]:\d\d\./)).toBeInTheDocument()

    const expires = new Date(Date.now() + 5 * 60_000).toISOString()
    handlers!.pairRequest?.({ id: 'r1', name: 'Pixel 9', machine: 'pixel-9', user: 'harvey@example.com', expires })
    const ask = await screen.findByRole('dialog', { name: 'Allow Pixel 9?' })
    // Where it calls from, as Tailscale knows it: the name is only the device's word.
    expect(within(ask).getByText('From pixel-9 on your tailnet, signed in as harvey@example.com.')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Pair a device' })).toBeNull())
    expect(within(ask).getByRole('button', { name: 'Deny' })).toHaveFocus()
    await fireEvent.click(within(ask).getByRole('button', { name: 'Allow' }))
    await waitFor(() => expect(answer).toHaveBeenCalledWith('r1', true))
  })

  it('warns when Tailscale can’t say where a request comes from, and closes once it expires', async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    await openRemote(backend)
    handlers!.pairRequest?.({ id: 'r2', name: 'Harvey’s iPhone', expires: new Date(Date.now() + 300).toISOString() })
    const ask = await screen.findByRole('dialog', { name: 'Allow Harvey’s iPhone?' })
    expect(within(ask).getByText(/Tailscale can't say which machine this is/)).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Allow Harvey’s iPhone?' })).toBeNull())
    expect(await screen.findByText(/stopped waiting to be allowed/)).toBeInTheDocument()
  })

  it('lists paired devices and revokes one after confirming', async () => {
    const backend = mockBackend()
    vi.spyOn(backend, 'pairedDevices').mockResolvedValue([
      { id: 'd1', name: 'Pixel 9', paired: '2026-10-20T10:00:00Z', lastSeen: new Date(Date.now() - 2 * 3600_000).toISOString() },
    ])
    const revoke = vi.spyOn(backend, 'revokeDevice')
    const dialog = await openRemote(backend)
    expect(await within(dialog).findByText('Pixel 9')).toBeInTheDocument()
    expect(within(dialog).getByText(/last seen 2 h/)).toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke' }))
    const sure = await screen.findByRole('dialog', { name: 'Revoke Pixel 9?' })
    await fireEvent.click(within(sure).getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(revoke).toHaveBeenCalledWith('d1'))
    expect(await within(dialog).findByText('No devices yet. Pair one to reach your sessions from it.')).toBeInTheDocument()
  })
})

describe('a remote client', () => {
  it('fetches state afresh when the server says events were lost', async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    const s = new AppStore(backend)
    await s.init()
    const id = s.currentId!
    s.live[id] = { seq: 1, text: 'half an answ', thinkingTokens: 0 }
    const boot = vi.spyOn(backend, 'bootstrap')
    const pages = vi.spyOn(backend, 'pages')
    handlers!.resync?.()
    await waitFor(() => expect(pages).toHaveBeenCalledWith(id))
    expect(boot).toHaveBeenCalledTimes(1)
    expect(s.live[id]).toBeUndefined()
    expect(s.pages[id]?.length).toBeGreaterThan(0)
    handlers!.connection?.({ state: 'offline', lastOnline: 1, error: 'gone' })
    expect(s.connection).toEqual({ state: 'offline', lastOnline: 1, error: 'gone' })
  })

  it('tries again when fetching state afresh fails as the phone wakes', async () => {
    const backend = mockBackend()
    const s = new AppStore(backend)
    await s.init()
    const real = backend.bootstrap.bind(backend)
    let fails = 1
    const boot = vi.spyOn(backend, 'bootstrap').mockImplementation(() => (fails-- > 0 ? Promise.reject(new Error('Couldn’t reach the server.')) : real()))
    vi.useFakeTimers()
    try {
      await s.resync()
      expect(boot).toHaveBeenCalledTimes(1)
      await vi.advanceTimersByTimeAsync(1000)
      expect(boot).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
      s.destroy()
    }
  })

  it('fetches state afresh once the connection is back, after giving up', async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    const s = new AppStore(backend)
    await s.init()
    const real = backend.bootstrap.bind(backend)
    let down = true
    const boot = vi.spyOn(backend, 'bootstrap').mockImplementation(() => (down ? Promise.reject(new Error('Couldn’t reach the server.')) : real()))
    vi.useFakeTimers()
    try {
      await s.resync()
      await vi.advanceTimersByTimeAsync(20_000)
      expect(boot).toHaveBeenCalledTimes(4)
      down = false
      handlers!.connection?.({ state: 'online', lastOnline: Date.now() })
      await vi.advanceTimersByTimeAsync(0)
      expect(boot).toHaveBeenCalledTimes(5)
    } finally {
      vi.useRealTimers()
      s.destroy()
    }
  })

  it("keeps a change that arrives while it's fetching state afresh", async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    const s = new AppStore(backend)
    await s.init()
    const id = s.currentId!
    const old = (await backend.pages(id)).map(p => ({ ...p }))
    const last = old[old.length - 1]!
    let release: (() => void) | null = null
    vi.spyOn(backend, 'pages').mockImplementation(() => new Promise(r => { release = () => r(old) }))
    const done = s.resync()
    await waitFor(() => expect(release).not.toBeNull())
    // The page changes while the older snapshot is on its way.
    handlers!.pageChanged({ ...last, answerMd: 'the newer answer' })
    release!()
    await done
    expect(s.pages[id]?.find(p => p.id === last.id)?.answerMd).toBe('the newer answer')
  })

  it('attaches an upload to the session it was sent from, even after moving to another', async () => {
    const backend = { ...mockBackend(), kind: 'http' as const }
    let finish: (() => void) | null = null
    vi.spyOn(backend, 'upload').mockImplementation(() => new Promise(r => {
      finish = () => r({ id: 'u1', name: 'notes.txt', kind: 'text', size: 5, path: '' } as never)
    }))
    const s = new AppStore(backend)
    await s.init()
    const [a, z] = s.activeSessions
    const { rerender } = render(Composer, { props: { session: a! }, context: new Map([[APP_CONTEXT, s]]) })
    const picker = document.querySelector<HTMLInputElement>('input[type=file]')!
    await fireEvent.change(picker, { target: { files: [new File(['hello'], 'notes.txt', { type: 'text/plain' })] } })
    await waitFor(() => expect(finish).not.toBeNull())
    // Off to another session while it's on its way.
    await rerender({ session: z! })
    finish!()
    await new Promise(r => setTimeout(r, 20))
    expect(screen.queryByRole('list', { name: 'Attached files' })).toBeNull()
    await rerender({ session: a! })
    expect(await screen.findByRole('list', { name: 'Attached files' })).toHaveTextContent('notes.txt')
  })

  it('knows what only works at the server', () => {
    expect(atServer({ kind: 'wails' })).toBe(true)
    expect(atServer({ kind: 'http' })).toBe(false)
  })
})
